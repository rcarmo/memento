//go:build !linux

package main

import "time"

// Unsupported hosts explicitly report CPUAccountingAvailable=false, not evidence
// of zero CPU consumption.
func processCPU() (user, system time.Duration, ok bool) { return 0, 0, false }
