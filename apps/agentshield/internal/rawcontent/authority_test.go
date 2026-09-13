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

	"siq-agent-security/apps/agentshield/internal/signing"
)

func testAuthority(t *testing.T, retention time.Duration) (string, *Store, *Authority, *signing.Key) {
	t.Helper()
	dir := t.TempDir()
	store, err := Initialize(dir, Limits{Retention: retention, Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	key, err := signing.FromSeed(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := InitializeAuthority(dir, key, store)
	if err != nil {
		t.Fatal(err)
	}
	return dir, store, authority, key
}

func TestAuthorityDisabledUntilExplicitInitialization(t *testing.T) {
	dir := t.TempDir()
	key, _ := signing.FromSeed(bytes.Repeat([]byte{1}, 32))
	if _, err := OpenAuthorityExisting(dir, key, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("authority accepted missing raw store", err)
	}
	if _, err := os.Stat(authorityDir(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read created authority directory", err)
	}
	store, err := Initialize(dir, Limits{Retention: DefaultRetention, Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAuthorityExisting(dir, key, store); !errors.Is(err, ErrDisabled) {
		t.Fatal("missing authority was not disabled", err)
	}
	if _, err := os.Stat(authorityDir(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("open created authority directory", err)
	}
	otherStore, err := Initialize(t.TempDir(), Limits{Retention: DefaultRetention, Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InitializeAuthority(dir, key, otherStore); !errors.Is(err, ErrInvalid) {
		t.Fatal("authority accepted store from another state directory", err)
	}
	if _, err := InitializeAuthority(dir, key, store); err != nil {
		t.Fatal(err)
	}
}

func TestTaskGrantControlsCaptureScopeAndRetention(t *testing.T) {
	dir, store, authority, key := testAuthority(t, 12*time.Hour)
	now := time.Date(2026, 9, 13, 4, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("PRIVATE_TASK", []string{"output", "input"}, "PRIVATE_ACTOR", time.Hour, 2*time.Hour, 512, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(grant.Kinds) != 2 || grant.Kinds[0] != "input" || grant.Kinds[1] != "output" || grant.TaskRef == "PRIVATE_TASK" || grant.ActorRef == "PRIVATE_ACTOR" {
		t.Fatal("grant scope not normalized or hashed", grant)
	}
	if !signing.VerifyCanonical(key.Public(), grantMap(grant), grant.Signature) {
		t.Fatal("grant signature invalid")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "raw-task-content-authority", grant.GrantID+".grant.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"PRIVATE_TASK", "PRIVATE_ACTOR"} {
		if bytes.Contains(raw, []byte(private)) {
			t.Fatal("raw identifier persisted", private)
		}
	}
	prepared, _ := Prepare("input", []Field{{Path: "/prompt", Value: "approved task input"}})
	envelope, err := authority.Capture(grant.GrantID, "PRIVATE_TASK", prepared, now.Add(time.Minute))
	if err != nil || envelope.ExpiresAt != "2026-09-13T06:01:00Z" {
		t.Fatal("authorized capture", envelope, err)
	}
	fields, _, err := store.Read("PRIVATE_TASK", envelope.RecordID, now.Add(2*time.Minute))
	if err != nil || len(fields) != 1 || fields[0].Value != "approved task input" {
		t.Fatal("captured content unreadable", fields, err)
	}
	if _, err := authority.Capture(grant.GrantID, "OTHER_TASK", prepared, now.Add(time.Minute)); !errors.Is(err, ErrDenied) {
		t.Fatal("cross-task capture accepted", err)
	}
	note, _ := Prepare("note", []Field{{Path: "/text", Value: "not allowed"}})
	if _, err := authority.Capture(grant.GrantID, "PRIVATE_TASK", note, now.Add(time.Minute)); !errors.Is(err, ErrDenied) {
		t.Fatal("ungranted kind accepted", err)
	}
	large, _ := Prepare("input", []Field{{Path: "/text", Value: strings.Repeat("x", 700)}})
	if _, err := authority.Capture(grant.GrantID, "PRIVATE_TASK", large, now.Add(time.Minute)); !errors.Is(err, ErrDenied) {
		t.Fatal("grant plaintext limit ignored", err)
	}
	if _, err := authority.Get(grant.GrantID, now.Add(-time.Nanosecond)); !errors.Is(err, ErrDenied) {
		t.Fatal("future grant accepted", err)
	}
	if _, err := authority.Get(grant.GrantID, now.Add(time.Hour)); !errors.Is(err, ErrExpired) {
		t.Fatal("expired grant accepted", err)
	}
	view, err := authority.Inspect(grant.GrantID, now.Add(time.Hour))
	if err != nil || view.Status != "expired" || view.Revocation != nil || view.Grant.Signature != grant.Signature {
		t.Fatal("expired management view", view, err)
	}
	views, err := authority.List(now.Add(time.Minute))
	if err != nil || len(views) != 1 || views[0].Status != "active" || views[0].Grant.GrantID != grant.GrantID {
		t.Fatal("grant list", views, err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "raw-task-content"))
	if len(entries) != 1 {
		t.Fatal("denied capture created ciphertext", len(entries))
	}
}

func TestGrantRevocationIsTerminalAndPreservesExistingCiphertext(t *testing.T) {
	_, store, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 5, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("task", []string{"output"}, "local-user", 2*time.Hour, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := Prepare("output", []Field{{Path: "/result", Value: "existing evidence detail"}})
	envelope, err := authority.Capture(grant.GrantID, "task", prepared, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Revoke(grant.GrantID, strings.Repeat("0", 128), "local-user", now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatal("wrong precondition accepted", err)
	}
	revocation, err := authority.Revoke(grant.GrantID, grant.Signature, "local-user", now.Add(2*time.Minute))
	if err != nil || revocation.ExpectedGrantSignature != grant.Signature {
		t.Fatal("revoke", revocation, err)
	}
	retry, err := authority.Revoke(grant.GrantID, grant.Signature, "retry-actor-is-ignored", now.Add(3*time.Minute))
	if err != nil || retry != revocation {
		t.Fatal("revoke retry not idempotent", retry, err)
	}
	if _, err := authority.Get(grant.GrantID, now.Add(3*time.Minute)); !errors.Is(err, ErrRevoked) {
		t.Fatal("revoked grant active", err)
	}
	view, err := authority.Inspect(grant.GrantID, now.Add(3*time.Minute))
	if err != nil || view.Status != "revoked" || view.Revocation == nil || view.Revocation.Signature != revocation.Signature {
		t.Fatal("revoked management view", view, err)
	}
	if _, err := authority.Capture(grant.GrantID, "task", prepared, now.Add(3*time.Minute)); !errors.Is(err, ErrRevoked) {
		t.Fatal("revoked capture accepted", err)
	}
	fields, _, err := store.Read("task", envelope.RecordID, now.Add(3*time.Minute))
	if err != nil || len(fields) != 1 {
		t.Fatal("revocation removed prior ciphertext", fields, err)
	}
}

func TestResolveActiveRequiresOneVerifiedTaskKindGrant(t *testing.T) {
	_, _, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, time.UTC)
	if _, err := authority.ResolveActive("task", "input", now); !errors.Is(err, ErrDenied) {
		t.Fatal("missing authority was selected", err)
	}
	output, err := authority.Issue("task", []string{"output"}, "operator", time.Hour, time.Hour, 4096, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ResolveActive("task", "input", now); !errors.Is(err, ErrDenied) {
		t.Fatal("wrong kind was selected", err)
	}
	if got, err := authority.ResolveActive("task", "output", now); err != nil || got.GrantID != output.GrantID {
		t.Fatal("unique authority not selected", got, err)
	}
	second, err := authority.Issue("task", []string{"output"}, "operator", time.Hour, time.Hour, 4096, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ResolveActive("task", "output", now.Add(2*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatal("overlapping authority was guessed", err)
	}
	if _, err := authority.Revoke(second.GrantID, second.Signature, "operator", now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got, err := authority.ResolveActive("task", "output", now.Add(4*time.Second)); err != nil || got.GrantID != output.GrantID {
		t.Fatal("revoked authority remained ambiguous", got, err)
	}
	if _, err := authority.ResolveActive("task", "unknown", now); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid kind accepted", err)
	}
}

func TestAuthorityTamperAndInvalidRequestsFailClosed(t *testing.T) {
	dir, _, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 6, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		kinds     []string
		duration  time.Duration
		retention time.Duration
		limit     int
	}{
		{nil, time.Hour, time.Hour, 1},
		{[]string{"input", "input"}, time.Hour, time.Hour, 1},
		{[]string{"input"}, time.Second, time.Hour, 1},
		{[]string{"input"}, 25 * time.Hour, time.Hour, 1},
		{[]string{"input"}, time.Hour, time.Minute, 1},
		{[]string{"input"}, time.Hour, 25 * time.Hour, 1},
		{[]string{"input"}, time.Hour, time.Hour, MaxPlaintext + 1},
	} {
		if _, err := authority.Issue("task", tc.kinds, "actor", tc.duration, tc.retention, tc.limit, now); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid grant accepted", tc, err)
		}
	}
	grant, err := authority.Issue("task", []string{"input"}, "actor", time.Hour, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}
	grantPath := filepath.Join(dir, "raw-task-content-authority", grant.GrantID+".grant.json")
	raw, _ := os.ReadFile(grantPath)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	doc["max_plaintext_bytes"] = float64(1)
	tampered, _ := json.Marshal(doc)
	if err := os.WriteFile(grantPath, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Get(grant.GrantID, now); !errors.Is(err, ErrState) {
		t.Fatal("tampered grant accepted", err)
	}
	if _, err := authority.List(now); !errors.Is(err, ErrState) {
		t.Fatal("grant list returned partial data after tamper", err)
	}

	_, _, second, _ := testAuthority(t, DefaultRetention)
	secondGrant, err := second.Issue("task", []string{"input"}, "actor", time.Hour, time.Hour, MaxPlaintext, now)
	if err != nil {
		t.Fatal(err)
	}
	revocation, err := second.Revoke(secondGrant.GrantID, secondGrant.Signature, "local-user", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	revocationPath, _ := second.revocationPath(secondGrant.GrantID)
	revocationRaw, _ := os.ReadFile(revocationPath)
	var revocationDoc map[string]any
	_ = json.Unmarshal(revocationRaw, &revocationDoc)
	revocationDoc["revoked_at"] = now.Add(2 * time.Minute).Format(time.RFC3339Nano)
	revocationRaw, _ = json.Marshal(revocationDoc)
	if err := os.WriteFile(revocationPath, revocationRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Get(secondGrant.GrantID, now.Add(3*time.Minute)); !errors.Is(err, ErrState) {
		t.Fatal("tampered revocation accepted", revocation, err)
	}
	if _, err := second.List(now.Add(3 * time.Minute)); !errors.Is(err, ErrState) {
		t.Fatal("grant list returned partial data after revocation tamper", err)
	}
}

func TestAuthorityFixtures(t *testing.T) {
	_, _, authority, _ := testAuthority(t, DefaultRetention)
	oldRandom := random
	random = bytes.NewReader(bytes.Repeat([]byte{9}, 16))
	t.Cleanup(func() { random = oldRandom })
	now := time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC)
	grant, err := authority.Issue("fixture-task", []string{"output", "input"}, "fixture-actor", time.Hour, 2*time.Hour, 4096, now)
	if err != nil {
		t.Fatal(err)
	}
	revocation, err := authority.Revoke(grant.GrantID, grant.Signature, "fixture-revoker", now.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	view, err := authority.Inspect(grant.GrantID, now.Add(20*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	views, err := authority.List(now.Add(20 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{
		"local-raw-task-content-grant.json":      grant,
		"local-raw-task-content-revocation.json": revocation,
		"local-raw-task-content-grant-view.json": view,
		"local-raw-task-content-grants.json": map[string]any{
			"schema_version": "local-raw-task-content-grants/v1",
			"items":          views,
		},
	} {
		raw, _ := json.MarshalIndent(value, "", "  ")
		raw = append(raw, '\n')
		want, err := os.ReadFile(filepath.Join("../../testdata/contracts", name))
		if err != nil || !bytes.Equal(raw, want) {
			t.Fatalf("%s cross-language fixture differs: %v\n%s", name, err, raw)
		}
	}
}
