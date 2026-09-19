package main

import (
	"flag"
	"fmt"
	"github.com/rcarmo/memento/go/assets"
	"io"
	"os"
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("memento-skill-import-go", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workspace := flags.String("workspace", ".", "workspace root")
	name := flags.String("name", "", "skill name")
	version := flags.String("version", "", "skill version")
	skillPath := flags.String("skill-md", "", "SKILL.md path")
	zipPath := flags.String("zip", "", "skill ZIP path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *name == "" || *version == "" || *skillPath == "" || *zipPath == "" {
		fmt.Fprintln(stderr, "memento-skill-import-go: --name, --version, --skill-md and --zip are required")
		return 2
	}
	skillMD, err := os.ReadFile(*skillPath)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 1
	}
	raw, err := os.ReadFile(*zipPath)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 1
	}
	destination, err := assets.ImportSkillPack(*workspace, *name, *version, string(skillMD), raw)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 1
	}
	fmt.Fprintln(stdout, destination)
	return 0
}

var exit = os.Exit

func main() { exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
