package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestDrainRejectsNewRequestsAndWaitsForActiveHandler(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	h := &drainingHandler{done: make(chan struct{}), handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	})}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-entered
	done := h.drain()
	late := httptest.NewRecorder()
	h.ServeHTTP(late, httptest.NewRequest("POST", "/", nil))
	if late.Code != 503 || calls.Load() != 1 {
		t.Error("draining dispatched another operation")
	}
	select {
	case <-done:
		t.Error("drain completed before active operation")
	default:
	}
	close(release)
	<-finished
	select {
	case <-done:
	default:
		t.Fatal("completed request did not finish drain")
	}
	<-h.drain() // Repeated drain is safe and cannot double close.
}

func TestHTTPStopCancelsConnectionButWaitsForHandler(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	hs := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(canceled)
		<-release // Model durable cleanup that must finish after request cancellation.
	})}
	stop := make(chan os.Signal, 1)
	result := make(chan error, 1)
	go func() { result <- serveLocalHTTP(hs, ln, stop, 20*time.Millisecond) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := localClient()
		defer client.CloseIdleConnections()
		resp, err := client.Get("http://" + ln.Addr().String())
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}()
	defer func() {
		close(release)
		_ = hs.Close()
		select {
		case <-clientDone:
		case <-time.After(5 * time.Second):
			t.Error("client did not exit")
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("handler not entered")
	}
	stop <- os.Interrupt
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("request not canceled after grace")
	}
	select {
	case err := <-result:
		t.Fatalf("serve returned while handler still owns state: %v", err)
	default:
	}
	// Cleanup runs after the deferred handler release and connection cleanup.
	t.Cleanup(func() {
		select {
		case err := <-result:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("missing forced-close diagnostic: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("serve did not finish after handler completion")
		}
	})
}

func TestHTTPStopWithoutRequests(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan os.Signal, 1)
	stop <- os.Interrupt
	hs := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	if err := serveLocalHTTP(hs, ln, stop, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeCheckCloseWaitsAfterGraceFailure(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	calls := 0
	go func() {
		done <- closeLocalRuntimeChecks(func(ctx context.Context) error {
			calls++
			if calls == 1 {
				<-ctx.Done()
				return ctx.Err()
			}
			close(entered)
			<-release
			return nil
		}, 0)
	}()
	<-entered
	select {
	case <-done:
		t.Fatal("returned while runtime worker still active")
	default:
	}
	close(release)
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("lost grace failure", err)
	}
	if calls != 2 {
		t.Fatal("unexpected close calls", calls)
	}
}

func TestRuntimeCheckCleanClose(t *testing.T) {
	calls := 0
	if err := closeLocalRuntimeChecks(func(context.Context) error { calls++; return nil }, time.Second); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
}

func TestServeMaintenanceRunsStartupAndTicksUntilCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	refreshTicks := make(chan time.Time, 1)
	purgeTicks := make(chan time.Time, 1)
	refreshCalls := make(chan struct{}, 1)
	purgeCalls := make(chan time.Time, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runServeMaintenance(ctx, refreshTicks, purgeTicks, func() error {
			refreshCalls <- struct{}{}
			return errors.New("refresh failure is isolated")
		}, func(now time.Time) error {
			purgeCalls <- now
			return errors.New("purge failure is isolated")
		})
	}()

	select {
	case startup := <-purgeCalls:
		if startup.IsZero() {
			t.Fatal("startup purge received zero time")
		}
	case <-time.After(time.Second):
		t.Fatal("startup purge did not run")
	}
	refreshTicks <- time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	select {
	case <-refreshCalls:
	case <-time.After(time.Second):
		t.Fatal("refresh tick did not run")
	}
	wantPurge := time.Date(2026, 9, 13, 10, 15, 0, 0, time.UTC)
	purgeTicks <- wantPurge
	select {
	case got := <-purgeCalls:
		if !got.Equal(wantPurge) {
			t.Fatal("purge tick time changed", got)
		}
	case <-time.After(time.Second):
		t.Fatal("purge tick did not run")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("maintenance did not stop with serve context")
	}
}
