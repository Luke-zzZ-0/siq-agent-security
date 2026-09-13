package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"siq-agent-security/apps/agentshield/internal/admission"
)

// GetHistoricalAdmission bounds reads for historical metadata. Callers must
// verify the signature and signed selection separately before trusting it.
func (s *Store) GetHistoricalAdmission(id string) (*admission.Admission, error) {
	invalid := errors.New("state: historical admission unavailable")
	if !safeID(id) {
		return nil, invalid
	}
	parent := filepath.Join(s.Dir, "admissions")
	before, err := os.Lstat(parent)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, invalid
	}
	// Reuse the existing 8 MiB immutable-record file budget and identity check.
	raw, err := readCommitFile(filepath.Join(parent, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, invalid
	}
	after, err := os.Lstat(parent)
	if err != nil || !after.IsDir() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, after) {
		return nil, invalid
	}
	var a admission.Admission
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&a) != nil || a.AdmissionID != id {
		return nil, invalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, invalid
	}
	return &a, nil
}
