package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeServer struct {
	listenErr      error
	shutdownErr    error
	shutdownCalled bool
	shutdownCh     chan struct{}
}

type stuckServer struct{}

func (stuckServer) ListenAndServe() error {
	select {}
}

func (stuckServer) Shutdown(context.Context) error {
	return nil
}

func (f *fakeServer) ListenAndServe() error {
	<-f.shutdownCh
	return f.listenErr
}

func (f *fakeServer) Shutdown(context.Context) error {
	f.shutdownCalled = true
	close(f.shutdownCh)
	return f.shutdownErr
}

func TestRunShutsDownOnSignal(t *testing.T) {
	f := &fakeServer{listenErr: http.ErrServerClosed, shutdownCh: make(chan struct{})}
	sigCh := make(chan os.Signal, 1)
	sigCh <- syscall.SIGTERM

	err := Run(f, sigCh, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !f.shutdownCalled {
		t.Fatal("expected shutdown to be called")
	}
}

func TestRunPropagatesListenError(t *testing.T) {
	wantErr := errors.New("listen failed")
	f := &fakeServer{listenErr: wantErr, shutdownCh: make(chan struct{})}
	sigCh := make(chan os.Signal, 1)

	go func() {
		time.Sleep(5 * time.Millisecond)
		sigCh <- syscall.SIGTERM
	}()

	err := Run(f, sigCh, 50*time.Millisecond)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}

func TestRunReturnsTimeoutIfServerDoesNotExitAfterShutdown(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	sigCh <- syscall.SIGTERM

	done := make(chan error, 1)
	go func() {
		done <- Run(stuckServer{}, sigCh, 20*time.Millisecond)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("expected timeout error, got %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected Run to return after timeout")
	}
}
