//go:build linux

// Package processnice contains Linux priority syscall adapters.
package processnice

import "syscall"

var (
	getPrioritySyscall = syscall.Getpriority
	allThreadsSyscall  = syscall.AllThreadsSyscall
)

func getPriorityImpl() (int, error) {
	// Linux returns 20-nice from getpriority(2) so that successful calls never
	// collide with the syscall error sentinel. syscall.Getpriority exposes that
	// kernel value directly rather than the user-visible nice priority.
	kernelPriority, err := getPrioritySyscall(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return 0, err
	}
	return 20 - kernelPriority, nil
}

func setPriorityImpl(priority int) error {
	// Linux stores nice per thread. The worker is pure Go (CGO_ENABLED=0), so
	// AllThreadsSyscall can apply the priority consistently to every runtime
	// thread; later threads inherit the creating thread's priority.
	_, _, errno := allThreadsSyscall(syscall.SYS_SETPRIORITY, uintptr(syscall.PRIO_PROCESS), 0, uintptr(priority))
	if errno != 0 {
		return errno
	}
	return nil
}
