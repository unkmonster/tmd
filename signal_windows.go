//go:build windows

package main

import (
	"os"
	"syscall"
)

func shutdownSignals() []os.Signal {
	return []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
	}
}
