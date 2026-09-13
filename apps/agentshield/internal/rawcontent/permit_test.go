package rawcontent

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCapturePermitBindsRuntimeSessionTaskAndGrant(t *testing.T) {
	dir, store, authority, key := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("runtime-task", []string{"input", "output"}, "operator", time.Hour, 2*time.Hour, 4096, now)
	if err != nil {
		t.Fatal(err)
	}
	permit, err := authority.IssueCapturePermit(
		grant.GrantID, grant.Signature,
		"ri-11111111111111111111111111111111", "native-session", "bind-"+strings.Repeat("2", 64),
		"runtime-task", "input", now.Add(20*time.Minute), 5*time.Minute, now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if permit.ExpiresAt != "2026-09-13T10:06:00Z" || !validCapturePermit(permit, key) {
		t.Fatal("invalid permit", permit)
	}
	serialized, _ := json.Marshal(permit)
	for _, private := range []string{"native-session", "runtime-task", "bind-" + strings.Repeat("2", 64)} {
		if bytes.Contains(serialized, []byte(private)) {
			t.Fatal("runtime identifier leaked into permit", private)
		}
	}
	prepared, err := Prepare("input", []Field{
		{Path: "/prompt", Value: "approved prompt"},
		{Path: "/api_token", Value: "must-not-persist", Secret: false},
		{Path: "/explicit", Value: "also-removed", Secret: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := authority.CaptureWithPermit(
		permit, "ri-11111111111111111111111111111111", "native-session", "bind-"+strings.Repeat("2", 64),
		"runtime-task", prepared, now.Add(2*time.Minute),
	)
	if err != nil || envelope.OmittedCount != 2 {
		t.Fatal("capture", envelope, err)
	}
	fields, _, err := store.Read("runtime-task", envelope.RecordID, now.Add(3*time.Minute))
	if err != nil || len(fields) != 1 || fields[0].Value != "approved prompt" {
		t.Fatal("filtered capture", fields, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "raw-task-content", envelope.RecordID+".json"))
	if err != nil || bytes.Contains(raw, []byte("approved prompt")) || bytes.Contains(raw, []byte("must-not-persist")) {
		t.Fatal("plaintext reached disk", err)
	}
}

func TestCapturePermitRejectsBorrowingTamperExpiryAndRevocation(t *testing.T) {
	_, _, authority, key := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("task", []string{"output"}, "operator", time.Hour, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}
	identity := "ri-" + strings.Repeat("3", 32)
	binding := "bind-" + strings.Repeat("4", 64)
	permit, err := authority.IssueCapturePermit(grant.GrantID, grant.Signature, identity, "session", binding, "task", "output", now.Add(time.Hour), time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := Prepare("output", []Field{{Path: "/result", Value: "value"}})
	for name, candidate := range map[string]struct {
		identity string
		session  string
		binding  string
		task     string
		content  Prepared
	}{
		"identity": {"ri-" + strings.Repeat("5", 32), "session", binding, "task", prepared},
		"session":  {identity, "other-session", binding, "task", prepared},
		"binding":  {identity, "session", "bind-" + strings.Repeat("6", 64), "task", prepared},
		"task":     {identity, "session", binding, "other-task", prepared},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := authority.CaptureWithPermit(permit, candidate.identity, candidate.session, candidate.binding, candidate.task, candidate.content, now.Add(time.Second)); !errors.Is(err, ErrDenied) {
				t.Fatal("borrowed permit accepted", err)
			}
		})
	}
	note, _ := Prepare("note", []Field{{Path: "/note", Value: "wrong kind"}})
	if _, err := authority.CaptureWithPermit(permit, identity, "session", binding, "task", note, now.Add(time.Second)); !errors.Is(err, ErrDenied) {
		t.Fatal("wrong content kind accepted", err)
	}
	tampered := permit
	tampered.Kind = "input"
	if validCapturePermit(tampered, key) {
		t.Fatal("tampered permit verified")
	}
	if _, err := authority.CaptureWithPermit(tampered, identity, "session", binding, "task", prepared, now.Add(time.Second)); !errors.Is(err, ErrInvalid) {
		t.Fatal("tampered permit accepted", err)
	}
	if _, err := authority.CaptureWithPermit(permit, identity, "session", binding, "task", prepared, now.Add(time.Minute)); !errors.Is(err, ErrExpired) {
		t.Fatal("expiry boundary accepted", err)
	}
	if _, err := authority.Revoke(grant.GrantID, grant.Signature, "operator", now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.CaptureWithPermit(permit, identity, "session", binding, "task", prepared, now.Add(31*time.Second)); !errors.Is(err, ErrRevoked) {
		t.Fatal("permit survived grant revocation", err)
	}
}

func TestCapturePermitIssuanceBoundsAndClampsUpstreamExpiry(t *testing.T) {
	_, _, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("task", []string{"input"}, "operator", time.Minute, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{grant.GrantID, grant.Signature, "ri-" + strings.Repeat("7", 32), "session", "bind-" + strings.Repeat("8", 64), "task", "input"}
	permit, err := authority.IssueCapturePermit(args[0], args[1], args[2], args[3], args[4], args[5], args[6], now.Add(20*time.Second), MaxPermitDuration, now)
	if err != nil || permit.ExpiresAt != "2026-09-13T12:00:20Z" {
		t.Fatal("runtime expiry did not clamp permit", permit, err)
	}
	for _, tc := range []struct {
		name      string
		signature string
		kind      string
		duration  time.Duration
		expires   time.Time
		want      error
	}{
		{"short", grant.Signature, "input", MinPermitDuration - time.Second, now.Add(time.Hour), ErrInvalid},
		{"long", grant.Signature, "input", MaxPermitDuration + time.Second, now.Add(time.Hour), ErrInvalid},
		{"fractional", grant.Signature, "input", MinPermitDuration + time.Nanosecond, now.Add(time.Hour), ErrInvalid},
		{"wrong-signature", strings.Repeat("0", 128), "input", time.Minute, now.Add(time.Hour), ErrDenied},
		{"wrong-kind", grant.Signature, "note", time.Minute, now.Add(time.Hour), ErrDenied},
		{"runtime-expired", grant.Signature, "input", time.Minute, now, ErrExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := authority.IssueCapturePermit(args[0], tc.signature, args[2], args[3], args[4], args[5], tc.kind, tc.expires, tc.duration, now)
			if !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
}

func TestCapturePermitFixture(t *testing.T) {
	_, _, authority, _ := testAuthority(t, DefaultRetention)
	oldRandom := random
	random = bytes.NewReader(bytes.Repeat([]byte{10}, 32))
	t.Cleanup(func() { random = oldRandom })
	now := time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("fixture-runtime-task", []string{"input"}, "fixture-operator", time.Hour, time.Hour, 4096, now)
	if err != nil {
		t.Fatal(err)
	}
	permit, err := authority.IssueCapturePermit(
		grant.GrantID, grant.Signature, "ri-"+strings.Repeat("1", 32), "fixture-session", "bind-"+strings.Repeat("2", 64),
		"fixture-runtime-task", "input", now.Add(time.Hour), time.Minute, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(permit, "", "  ")
	raw = append(raw, '\n')
	want, err := os.ReadFile(filepath.Join("../../testdata/contracts", "local-raw-task-content-capture-permit.json"))
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatalf("capture permit fixture differs: %v\n%s", err, raw)
	}
}

func TestCapturePermitRejectsInvalidRuntimeReferences(t *testing.T) {
	_, _, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 14, 0, 0, 0, time.UTC)
	grant, _ := authority.Issue("task", []string{"input"}, "operator", time.Hour, time.Hour, MaxPlaintext, now)
	base := []string{"ri-valid", "session", "binding", "task"}
	for i := range base {
		args := append([]string(nil), base...)
		args[i] = "bad\nvalue"
		if _, err := authority.IssueCapturePermit(grant.GrantID, grant.Signature, args[0], args[1], args[2], args[3], "input", now.Add(time.Hour), time.Minute, now); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid reference accepted", i, err)
		}
	}
}
