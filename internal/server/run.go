package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Runner is the minimal server contract used by Run.
type Runner interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

// Run starts srv and gracefully shuts it down when a signal is received.
func Run(srv Runner, signals <-chan os.Signal, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return err
		}

		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) || err == nil {
				return nil
			}
			return err
		case <-time.After(timeout):
			return fmt.Errorf("timed out waiting for server to stop after shutdown")
		}
	}
}
