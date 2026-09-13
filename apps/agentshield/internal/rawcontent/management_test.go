package rawcontent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListMetadataAuthenticatesAllRecordsAndScopesTask(t *testing.T) {
	dir, _, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, time.UTC)
	for _, task := range []string{"task-a", "task-b"} {
		grant, err := authority.Issue(task, []string{"note"}, "operator", 2*time.Hour, time.Hour, MaxPlaintext, now)
		if err != nil {
			t.Fatal(err)
		}
		prepared, _ := Prepare("note", []Field{{Path: "/text", Value: task + " private"}})
		if _, err := authority.Capture(grant.GrantID, task, prepared, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	items, err := authority.store.ListMetadata("task-a", now.Add(2*time.Minute))
	if err != nil || len(items) != 1 || items[0].Status != "active" || items[0].TaskRef == "task-a" {
		t.Fatal("task metadata", items, err)
	}
	items, err = authority.store.ListMetadata("task-a", now.Add(2*time.Hour))
	if err != nil || len(items) != 1 || items[0].Status != "expired" {
		t.Fatal("expired metadata", items, err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "raw-task-content"))
	path := filepath.Join(dir, "raw-task-content", entries[1].Name())
	raw, _ := os.ReadFile(path)
	var envelope map[string]any
	_ = json.Unmarshal(raw, &envelope)
	envelope["plaintext_bytes"] = float64(1)
	tampered, _ := json.Marshal(envelope)
	if err := os.WriteFile(path, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.store.ListMetadata("task-a", now.Add(2*time.Minute)); !errors.Is(err, ErrState) {
		t.Fatal("corrupt unrelated record allowed partial list", err)
	}
}

func TestPurgeExpiredAuthenticatesBeforeDeleting(t *testing.T) {
	dir, store, authority, _ := testAuthority(t, DefaultRetention)
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	grant, _ := authority.Issue("task", []string{"note"}, "operator", 3*time.Hour, time.Hour, MaxPlaintext, now)
	prepared, _ := Prepare("note", []Field{{Path: "/text", Value: "private"}})
	first, _ := authority.Capture(grant.GrantID, "task", prepared, now)
	second, _ := authority.Capture(grant.GrantID, "task", prepared, now.Add(time.Minute))
	secondPath := filepath.Join(dir, "raw-task-content", second.RecordID+".json")
	raw, _ := os.ReadFile(secondPath)
	raw[len(raw)-2] ^= 1
	if err := os.WriteFile(secondPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PurgeExpired(now.Add(2 * time.Hour)); !errors.Is(err, ErrState) {
		t.Fatal("tampered record did not fail purge", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "raw-task-content", first.RecordID+".json")); err != nil {
		t.Fatal("purge deleted before complete authentication", err)
	}
}
