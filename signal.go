package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// signal_notifyContext returns a context that is cancelled on SIGTERM or SIGINT.
// Callers must call the returned stop function to release resources.
func signal_notifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
