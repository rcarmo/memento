//go:build linux

package main

import (
	"syscall"
	"time"
)

// RUSAGE_SELF includes every OS thread in this process, including Gemma workers.
func processCPU() (user, system time.Duration, ok bool) {
	var r syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &r) != nil {
		return 0, 0, false
	}
	return time.Duration(r.Utime.Sec)*time.Second + time.Duration(r.Utime.Usec)*time.Microsecond,
		time.Duration(r.Stime.Sec)*time.Second + time.Duration(r.Stime.Usec)*time.Microsecond, true
}
