package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHistoricalAdmissionReadBounds(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir, "admissions", "history.json")
	body := []byte(`{"admission_id":"history","skill_name":"sample"}`)
	write := func(raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(body)
	if a, err := s.GetHistoricalAdmission("history"); err != nil || a.SkillName != "sample" {
		t.Fatal("valid read failed", err)
	}
	// JSON whitespace gives an exact boundary without changing the document.
	exact := append(append([]byte{}, body...), bytes.Repeat([]byte(" "), maxCommitBytes-len(body))...)
	write(exact)
	if _, err := s.GetHistoricalAdmission("history"); err != nil {
		t.Fatal("exact budget rejected", err)
	}
	write(append(exact, ' '))
	if a, err := s.GetHistoricalAdmission("history"); err == nil || a != nil {
		t.Fatal("over budget read")
	}
	for _, raw := range [][]byte{[]byte(`{"admission_id":"other"}`), []byte(`{"admission_id":"history","secret":"raw"}`), append(append([]byte{}, body...), []byte(` {}`)...), []byte(`null`), []byte(`{"admission_id":`)} {
		write(raw)
		if a, err := s.GetHistoricalAdmission("history"); err == nil || a != nil {
			t.Fatal("invalid document accepted")
		}
	}
	for _, id := range []string{"../history", "", "history/child"} {
		if _, err := s.GetHistoricalAdmission(id); err == nil {
			t.Fatal("unsafe id accepted")
		}
	}
	if _, err := s.GetHistoricalAdmission("missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing status lost", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetHistoricalAdmission("history"); err == nil {
		t.Fatal("directory read")
	}
}

func TestHistoricalAdmissionRejectsLinks(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"admission_id":"history"}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir, "admissions", "history.json")
	if err := os.Symlink(outside, path); err != nil {
		t.Skip("symlink unavailable", err)
	}
	if _, err := s.GetHistoricalAdmission("history"); err == nil {
		t.Fatal("file link accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(path)
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), parent); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetHistoricalAdmission("outside"); err == nil {
		t.Fatal("directory link accepted")
	}
}
