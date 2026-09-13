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

func TestDisabledByDefaultAndEncryptedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	limits := Limits{Retention: DefaultRetention, Budget: DefaultBudget}
	if _, err := OpenExisting(dir, limits); !errors.Is(err, ErrDisabled) {
		t.Fatal("optional store created by read", err)
	}
	if _, err := os.Stat(keyPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("default read created key")
	}
	prepared, err := Prepare("parameters", []Field{
		{Path: "/prompt", Value: "summarize the approved report"},
		{Path: "/api_key", Value: "PRIVATE_API_KEY", Secret: false},
		{Path: "/nested/token", Value: "PRIVATE_TOKEN", Secret: false},
		{Path: "/declared", Value: "PRIVATE_DECLARED", Secret: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := Initialize(dir, limits)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC)
	envelope, err := store.write("PRIVATE_TASK", prepared, limits.Retention, now)
	if err != nil || envelope.OmittedCount != 3 || envelope.Kind != "parameters" || envelope.ExpiresAt != "2026-09-14T01:00:00Z" {
		t.Fatal("write", envelope, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "raw-task-content", envelope.RecordID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_TASK", "PRIVATE_API_KEY", "PRIVATE_TOKEN", "PRIVATE_DECLARED", "summarize the approved report"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("plaintext leaked: %s", secret)
		}
	}
	fields, got, err := store.Read("PRIVATE_TASK", envelope.RecordID, now.Add(time.Minute))
	if err != nil || len(fields) != 1 || fields[0].Path != "/prompt" || fields[0].Value != "summarize the approved report" || got.PlaintextHash != envelope.PlaintextHash {
		t.Fatal("read", fields, got, err)
	}
	if _, _, err := store.Read("OTHER_TASK", envelope.RecordID, now); !errors.Is(err, ErrState) {
		t.Fatal("cross-task read", err)
	}
	if _, _, err := store.Read("PRIVATE_TASK", envelope.RecordID, now.Add(DefaultRetention)); !errors.Is(err, ErrExpired) {
		t.Fatal("expired content read", err)
	}
	if err := store.Delete("OTHER_TASK", envelope.RecordID); !errors.Is(err, ErrState) {
		t.Fatal("cross-task delete", err)
	}
	if err := store.Delete("PRIVATE_TASK", envelope.RecordID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "raw-task-content", envelope.RecordID+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ciphertext not deleted")
	}
}

func TestPrepareRejectsUnsafeAndOmitsCredentialValues(t *testing.T) {
	credential := "Bearer " + strings.Repeat("A", 32)
	prepared, err := Prepare("input", []Field{{Path: "/message", Value: "keep"}, {Path: "/header", Value: credential}})
	if err != nil {
		t.Fatal(err)
	}
	var p payload
	if json.Unmarshal(prepared.raw, &p) != nil || len(p.Fields) != 1 || p.OmittedCount != 1 {
		t.Fatal("credential value retained")
	}
	for _, tc := range []struct {
		kind   string
		fields []Field
	}{
		{"unknown", []Field{{Path: "/x", Value: "x"}}},
		{"input", nil},
		{"input", []Field{{Path: "relative", Value: "x"}}},
		{"input", []Field{{Path: "/x", Value: "x"}, {Path: "/x", Value: "y"}}},
		{"input", []Field{{Path: "/x", Value: "bad\x00value"}}},
		{"input", []Field{{Path: "/token", Value: "only secret"}}},
	} {
		if _, err := Prepare(tc.kind, tc.fields); !errors.Is(err, ErrInvalid) {
			t.Fatal("unsafe content accepted", tc, err)
		}
	}
}

func TestTamperKeyAndBudgetFailClosed(t *testing.T) {
	dir := t.TempDir()
	limits := Limits{Retention: time.Hour, Budget: 1 << 20}
	store, err := Initialize(dir, limits)
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := Prepare("output", []Field{{Path: "/text", Value: "safe output"}})
	now := time.Now().UTC()
	e, err := store.write("task", prepared, limits.Retention, now)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "raw-task-content", e.RecordID+".json")
	raw, _ := os.ReadFile(path)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	doc["kind"] = "input"
	tampered, _ := json.Marshal(doc)
	if err := os.WriteFile(path, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Read("task", e.RecordID, now); !errors.Is(err, ErrState) {
		t.Fatal("tampered AAD accepted", err)
	}
	if err := os.WriteFile(keyPath(dir), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenExisting(dir, limits); !errors.Is(err, ErrState) {
		t.Fatal("corrupt key replaced", err)
	}
	if string(rawKey(t, keyPath(dir))) != "broken" {
		t.Fatal("corrupt key changed")
	}
	if _, err := Initialize(dir, Limits{Retention: time.Minute, Budget: 1 << 20}); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid limits accepted", err)
	}
}

func TestPurgeExpiredDoesNotTouchOtherFacts(t *testing.T) {
	dir := t.TempDir()
	store, err := Initialize(dir, Limits{Retention: time.Hour, Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := Prepare("note", []Field{{Path: "/text", Value: "temporary detail"}})
	now := time.Date(2026, 9, 13, 2, 0, 0, 0, time.UTC)
	first, _ := store.write("task-a", prepared, time.Hour, now)
	second, _ := store.write("task-b", prepared, time.Hour, now.Add(30*time.Minute))
	audit := filepath.Join(dir, "receipts", "local", "facts.jsonl")
	if err := os.MkdirAll(filepath.Dir(audit), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(audit, []byte("immutable fact"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := store.PurgeExpired(now.Add(75 * time.Minute))
	if err != nil || result.Deleted != 1 || result.Bytes == 0 {
		t.Fatal(result, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "raw-task-content", first.RecordID+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired ciphertext retained")
	}
	if _, err := os.Stat(filepath.Join(dir, "raw-task-content", second.RecordID+".json")); err != nil {
		t.Fatal("live ciphertext deleted", err)
	}
	if raw, err := os.ReadFile(audit); err != nil || string(raw) != "immutable fact" {
		t.Fatal("fact chain changed", err)
	}
}

func rawKey(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestEnvelopeFixture(t *testing.T) {
	oldRandom := random
	random = bytes.NewReader(bytes.Repeat([]byte{7}, 60))
	t.Cleanup(func() { random = oldRandom })
	dir := t.TempDir()
	store, err := Initialize(dir, Limits{Retention: DefaultRetention, Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := Prepare("input", []Field{{Path: "/prompt", Value: "fixture content"}, {Path: "/token", Value: "omitted"}})
	envelope, err := store.write("fixture-task", prepared, DefaultRetention, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(envelope, "", "  ")
	raw = append(raw, '\n')
	want, err := os.ReadFile("../../testdata/contracts/local-raw-task-content-envelope.json")
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatal("cross-language fixture differs", err)
	}
}
