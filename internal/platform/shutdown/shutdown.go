package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WaitForSignal blocks until SIGINT or SIGTERM is received.
func WaitForSignal() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx
}
