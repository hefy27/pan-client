package internal

import (
	"context"
	"sync"
)

var (
	// globalCtx is the root context for all background operations.
	globalCtx    context.Context
	globalCancel context.CancelFunc
	shutdownOnce sync.Once
	shutdownWg   sync.WaitGroup
)

func init() {
	globalCtx, globalCancel = context.WithCancel(context.Background())
}

// GlobalContext returns the global context used by background goroutines.
func GlobalContext() context.Context {
	return globalCtx
}

// ShutdownWg returns the WaitGroup for tracking background tasks.
func ShutdownWg() *sync.WaitGroup {
	return &shutdownWg
}

// Shutdown cancels the global context and waits for all background tasks to finish.
func Shutdown() {
	shutdownOnce.Do(func() {
		globalCancel()
		shutdownWg.Wait()
	})
}
