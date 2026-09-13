package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewFutureStateOpenMustNotCreateDirectories(t *testing.T) {
	dir := t.TempDir()
	writeMarker(t, dir, StateFormatMarker{Schema: StateFormatSchema, FormatVersion: 999})
	if _, err := Open(dir); err == nil {
		t.Error("Open accepted future state")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("Open mutated rejected state: %d entries", len(entries))
	}
}
func TestReviewFutureStateWriterMustNotQuarantineLock(t *testing.T) {
	dir := t.TempDir()
	writeMarker(t, dir, StateFormatMarker{Schema: StateFormatSchema, FormatVersion: 999})
	w, err := AcquireWriter(dir)
	if err == nil {
		_ = w.Release()
		t.Fatal("AcquireWriter accepted future state")
	}
}
func TestReviewRejectTrailingJSONAndDuplicateVersion(t *testing.T) {
	for _, raw := range []string{`{"schema":"state-format/v1","format_version":1} {"format_version":999}`, `{"schema":"state-format/v1","format_version":999,"format_version":1}`} {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, StateFormatMarkerName), []byte(raw), 0600)
		if _, err := CheckStateCompatibility(dir); err == nil {
			t.Error("ambiguous JSON accepted")
		}
	}
}
func TestReviewCorruptStatusReturned(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, StateFormatMarkerName), []byte(`broken`), 0600)
	got, err := CheckStateCompatibility(dir)
	if !errors.Is(err, ErrCorruptState) || got.Status != CompatStatusCorrupt {
		t.Fatalf("status=%q error=%v", got.Status, err)
	}
}
