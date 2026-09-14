package rawcontent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Concurrency negatives for N08 item 6: captures racing a terminal revocation
// must never produce a late ciphertext, an unreadable record, or an error
// outside the documented fail-closed vocabulary.
func TestCaptureRacingRevokeNeverProducesLateWrite(t *testing.T) {
	_, store, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 6, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("concurrent-task", []string{"output"}, "local-user", 2*time.Hour, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}

	const captures = 16
	type result struct {
		seq      int
		envelope Envelope
		err      error
	}
	results := make([]result, captures)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(captures + 1)
	for i := 0; i < captures; i++ {
		go func(seq int) {
			defer wg.Done()
			<-start
			prepared, perr := Prepare("output", []Field{{Path: "/result", Value: fmt.Sprintf("captured result %d", seq)}})
			if perr != nil {
				results[seq] = result{seq: seq, err: perr}
				return
			}
			at := now.Add(time.Duration(seq) * time.Millisecond)
			envelope, err := authority.Capture(grant.GrantID, "concurrent-task", prepared, at)
			results[seq] = result{seq: seq, envelope: envelope, err: err}
		}(i)
	}
	go func() {
		defer wg.Done()
		<-start
		// Give the capture goroutines a moment to claim authorityMu first so
		// the race window is actually exercised instead of revoke winning the
		// start-line scheduling race every time.
		time.Sleep(2 * time.Millisecond)
		if _, err := authority.Revoke(grant.GrantID, grant.Signature, "local-user", now.Add(time.Minute)); err != nil {
			t.Errorf("revoke failed: %v", err)
		}
	}()
	close(start)
	wg.Wait()

	succeeded := 0
	seen := map[string]int{}
	for _, r := range results {
		switch {
		case r.err == nil:
			succeeded++
			if seen[r.envelope.RecordID] > 0 {
				t.Fatalf("duplicate record id %s across concurrent captures", r.envelope.RecordID)
			}
			seen[r.envelope.RecordID] = r.seq
			if r.envelope.TaskRef != grant.TaskRef || r.envelope.Kind != "output" {
				t.Fatalf("envelope %s not bound to the granted task/kind", r.envelope.RecordID)
			}
		case errors.Is(r.err, ErrRevoked):
			// The revoke tombstone landed first. Fail closed, no ciphertext.
		default:
			t.Fatalf("capture %d failed with undocumented error: %v", r.seq, r.err)
		}
	}
	if succeeded == 0 {
		t.Fatal("no capture completed before revocation; race window never exercised")
	}

	// Every accepted capture stays readable and decrypts to the exact value its
	// own goroutine captured, even though revocation is terminal.
	for id, seq := range seen {
		fields, _, err := store.Read("concurrent-task", id, now.Add(2*time.Minute))
		if err != nil || len(fields) != 1 || fields[0].Value != fmt.Sprintf("captured result %d", seq) {
			t.Fatalf("record %s from capture %d unreadable or wrong value: %v", id, seq, err)
		}
	}
	// No pending residue from the concurrent write paths.
	entries, err := os.ReadDir(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' && len(seen) > 0 {
			t.Fatalf("pending write residue %q after concurrent captures", entry.Name())
		}
	}

	// After Revoke returned, capture is terminally denied.
	later, _ := Prepare("output", []Field{{Path: "/result", Value: "after revocation"}})
	if _, err := authority.Capture(grant.GrantID, "concurrent-task", later, now.Add(2*time.Minute)); !errors.Is(err, ErrRevoked) {
		t.Fatalf("post-revocation capture error = %v, want ErrRevoked", err)
	}
	view, err := authority.Inspect(grant.GrantID, now.Add(2*time.Minute))
	if err != nil || view.Status != "revoked" || view.Revocation == nil {
		t.Fatalf("final grant view = %v, %v", view, err)
	}
}

// Concurrent revocations of the same grant share one tombstone via the CAS
// precondition; losers must observe the first record, never a second one.
func TestConcurrentRevokeProducesSingleTombstone(t *testing.T) {
	dir, _, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 7, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("revoke-task", []string{"note"}, "local-user", 2*time.Hour, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}

	const revocations = 8
	first := make(chan Revocation, 1)
	errs := make([]error, revocations)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(revocations)
	for i := 0; i < revocations; i++ {
		go func(seq int) {
			defer wg.Done()
			<-start
			revocation, err := authority.Revoke(grant.GrantID, grant.Signature, "local-user", now.Add(time.Duration(seq)*time.Millisecond))
			errs[seq] = err
			if err == nil && seq == 0 {
				first <- revocation
			}
		}(i)
	}
	close(start)
	wg.Wait()

	var agreed Revocation
	select {
	case agreed = <-first:
	default:
		t.Fatal("no revocation succeeded")
	}
	for seq, err := range errs {
		if err != nil {
			t.Fatalf("concurrent revoke %d failed: %v", seq, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "raw-task-content-authority"))
	if err != nil {
		t.Fatal(err)
	}
	tombstones := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".revocation.json") {
			tombstones++
		}
	}
	if tombstones != 1 {
		t.Fatalf("tombstone count = %d, want exactly 1", tombstones)
	}
	view, err := authority.Inspect(grant.GrantID, now.Add(time.Minute))
	if err != nil || view.Status != "revoked" || view.Revocation == nil || *view.Revocation != agreed {
		t.Fatalf("revocation view disagrees with tombstone: %v, %v", view, err)
	}
}
