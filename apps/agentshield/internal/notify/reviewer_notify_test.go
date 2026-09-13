package notify

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReviewerFailedNotifyRespectsRetryWindow(t *testing.T) {
	attempts := 0
	d := NewDispatcher(func() int { return 1 }, &stubNotifier{failN: 100, failEr: errors.New("offline")}, DispatcherOptions{PollInterval: 5 * time.Second, CoalesceInterval: 15 * time.Second, Log: func(error) { attempts++ }})
	now := time.Now()
	d.Tick(now)
	d.Tick(now.Add(5 * time.Second))
	d.Tick(now.Add(10 * time.Second))
	if attempts != 1 {
		t.Fatalf("within 15s coalesce window got %d failed delivery attempts, want 1", attempts)
	}
}

func TestReviewerNotifierOutputDoesNotReachLog(t *testing.T) {
	if os.Getenv("SIQ_REVIEW_NOTIFY_CHILD") == "1" {
		os.Stderr.WriteString("review-secret-marker")
		os.Stdout.WriteString(strings.Repeat("x", 2<<20))
		os.Exit(1)
	}
	t.Setenv("SIQ_REVIEW_NOTIFY_CHILD", "1")
	command := CommandNotifier{Bin: os.Args[0], Args: []string{"-test.run=^TestReviewerNotifierOutputDoesNotReachLog$"}, Timeout: time.Second}
	err := command.Notify(Notification{Title: "SIQ", Body: "count 1"})
	if err == nil {
		t.Fatal("expected child failure")
	}
	if strings.Contains(err.Error(), "review-secret-marker") {
		t.Fatal("notifier stderr is returned verbatim and forwarded to daemon logs")
	}
}

func TestNotifyFailureBackoffSurvivesEmptyInboxAndRecovers(t *testing.T) {
	count := 1
	st := &stubNotifier{failN: 1, failEr: errors.New("offline")}
	d := NewDispatcher(func() int { return count }, st, DispatcherOptions{CoalesceInterval: 15 * time.Second})
	base := time.Now()
	d.Tick(base)
	count = 0
	d.Tick(base.Add(time.Second))
	count = 2
	if d.Tick(base.Add(2 * time.Second)) {
		t.Fatal("empty inbox reset failure backoff")
	}
	if !d.Tick(base.Add(15 * time.Second)) {
		t.Fatal("missed recovery at retry deadline")
	}
	if st.count() != 1 {
		t.Fatal("unexpected delivery count")
	}
}

func TestNotifyTimeoutAndLaunchErrorsAreCategorical(t *testing.T) {
	if os.Getenv("SIQ_REVIEW_NOTIFY_SLEEP") == "1" {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}
	t.Setenv("SIQ_REVIEW_NOTIFY_SLEEP", "1")
	c := CommandNotifier{Bin: os.Args[0], Args: []string{"-test.run=^TestNotifyTimeoutAndLaunchErrorsAreCategorical$"}, Timeout: 20 * time.Millisecond}
	if err := c.Notify(Notification{}); !errors.Is(err, ErrDeliveryTimeout) {
		t.Fatal("timeout category", err)
	}
	c = CommandNotifier{Bin: "/missing/review-secret-marker/notifier"}
	if err := c.Notify(Notification{}); !errors.Is(err, ErrDeliveryFailed) || strings.Contains(err.Error(), "review-secret-marker") {
		t.Fatal("launch category", err)
	}
}
