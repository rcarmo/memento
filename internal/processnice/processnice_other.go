//go:build !linux

// Package processnice provides no-op adapters on non-Linux development hosts.
package processnice

func getPriorityImpl() (int, error) {
	return 0, nil
}

func setPriorityImpl(int) error {
	return nil
}
