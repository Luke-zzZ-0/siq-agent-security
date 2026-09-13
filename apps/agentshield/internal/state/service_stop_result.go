package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/signing"
)

type ServiceStopResult struct {
	SchemaVersion    string `json:"schema_version"`
	BootID           string `json:"boot_id"`
	DirectoryID      string `json:"state_directory_id"`
	AcceptanceSHA256 string `json:"acceptance_sha256"`
	FinishedAt       int64  `json:"finished_at"`
	Status           string `json:"status"`
	Signature        string `json:"signature"`
}

func (r ServiceStopResult) unsigned() map[string]any {
	return map[string]any{"schema_version": r.SchemaVersion, "boot_id": r.BootID, "state_directory_id": r.DirectoryID, "acceptance_sha256": r.AcceptanceSHA256, "finished_at": r.FinishedAt, "status": r.Status}
}

func stopAcceptanceDigest(accepted ServiceStopAcceptance) (string, error) {
	doc := accepted.unsigned()
	doc["signature"] = accepted.Signature
	raw, err := canon.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func newServiceStopResult(key *signing.Key, accepted ServiceStopAcceptance, clean bool, now time.Time) (ServiceStopResult, error) {
	if key == nil || !signing.VerifyCanonical(key.Public(), accepted.unsigned(), accepted.Signature) || now.Unix() < accepted.AcceptedAt {
		return ServiceStopResult{}, errors.New("state: invalid stop result source or time")
	}
	digest, err := stopAcceptanceDigest(accepted)
	if err != nil {
		return ServiceStopResult{}, err
	}
	r := ServiceStopResult{SchemaVersion: "local-service-stop-result/v1", BootID: accepted.BootID, DirectoryID: accepted.DirectoryID, AcceptanceSHA256: digest, FinishedAt: now.Unix(), Status: "drain_failed"}
	if clean {
		r.Status = "drained"
	}
	r.Signature, err = key.SignCanonical(r.unsigned())
	return r, err
}

func (s *Store) ReadServiceStopResult(key *signing.Key, boot string) (ServiceStopResult, error) {
	var r ServiceStopResult
	accepted, err := s.ReadServiceStopAcceptance(key, boot)
	if err != nil {
		return r, err
	}
	digest, err := stopAcceptanceDigest(accepted)
	if err != nil {
		return r, err
	}
	raw, err := readInitializationFile(filepath.Join(s.Dir, "service-stop-"+boot+".result.json"))
	if err != nil {
		return r, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF || r.SchemaVersion != "local-service-stop-result/v1" || r.BootID != boot || r.DirectoryID != accepted.DirectoryID ||
		r.AcceptanceSHA256 != digest || r.FinishedAt < accepted.AcceptedAt || (r.Status != "drained" && r.Status != "drain_failed") || !signing.VerifyCanonical(key.Public(), r.unsigned(), r.Signature) {
		return ServiceStopResult{}, errors.New("state: invalid stop result")
	}
	return r, nil
}

func (s *Store) RecordServiceStopResult(w *Writer, key *signing.Key, boot string, clean bool, now time.Time) (ServiceStopResult, error) {
	if err := s.serviceWriter(w); err != nil {
		return ServiceStopResult{}, err
	}
	accepted, err := s.ReadServiceStopAcceptance(key, boot)
	if err != nil {
		return ServiceStopResult{}, err
	}
	r, err := newServiceStopResult(key, accepted, clean, now)
	if err != nil {
		return ServiceStopResult{}, err
	}
	saved, err := s.ReadServiceStopResult(key, boot)
	if err == nil {
		if saved.Status != r.Status {
			return ServiceStopResult{}, errors.New("state: conflicting stop result")
		}
		return saved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return ServiceStopResult{}, err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return ServiceStopResult{}, err
	}
	if err := publishCommitFile(filepath.Join(s.Dir, "service-stop-"+boot+".result.json"), raw); err != nil {
		return ServiceStopResult{}, err
	}
	return r, nil
}
