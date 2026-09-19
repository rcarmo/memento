package main

import (
	"errors"
	"strings"
	"testing"
)

func FuzzEnvironmentFile(f *testing.F) {
	for _, raw := range []string{"", "# comment\n", "A=b\n", "export A='b c'\n", "BAD NAME=x\n", "A=\x00\n"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		seen := map[string]string{}
		err := loadEnvironment(strings.NewReader(raw), func(name, value string) error {
			if name == "FAIL" {
				return errors.New("setenv")
			}
			seen[name] = value
			return nil
		})
		if err == nil {
			for name := range seen {
				if name == "" || strings.ContainsAny(name, " \t\r\n") {
					t.Fatalf("accepted invalid environment name %q", name)
				}
			}
		}
	})
}

func FuzzHealthcheckArgs(f *testing.F) {
	for _, raw := range []string{"", "--address\x00127.0.0.1:1\x00--timeout\x001ms", "--timeout\x000s", "--bad\x00x", "--address"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		args := []string{}
		if raw != "" {
			args = strings.Split(raw, "\x00")
		}
		address, timeout, err := parseHealthcheckArgs(args)
		if err == nil && (address == "" || timeout <= 0) {
			t.Fatalf("accepted invalid healthcheck result: %q %s", address, timeout)
		}
	})
}
