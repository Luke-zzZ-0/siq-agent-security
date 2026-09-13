package state

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
)

type ServiceStopAcceptance struct {
	SchemaVersion string `json:"schema_version"`
	Action        string `json:"action"`
	BootID        string `json:"boot_id"`
	DirectoryID   string `json:"state_directory_id"`
	RequestSHA256 string `json:"request_sha256"`
	AcceptedAt    int64  `json:"accepted_at"`
	Signature     string `json:"signature"`
}

func (r ServiceStopAcceptance) unsigned() map[string]any {
	return map[string]any{"schema_version": r.SchemaVersion, "action": r.Action, "boot_id": r.BootID, "state_directory_id": r.DirectoryID, "request_sha256": r.RequestSHA256, "accepted_at": r.AcceptedAt}
}

func stopIDValid(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}

func newServiceStopAcceptance(key *signing.Key, request localcontrol.Message, now time.Time) (ServiceStopAcceptance, error) {
	digest, err := localcontrol.StopDigest(request, key, now)
	if err != nil {
		return ServiceStopAcceptance{}, err
	}
	r := ServiceStopAcceptance{SchemaVersion: "local-service-stop-acceptance/v1", Action: "stop_accepted", BootID: request.BootID, DirectoryID: request.DirectoryID, RequestSHA256: digest, AcceptedAt: now.Unix()}
	if r.AcceptedAt <= 0 {
		return ServiceStopAcceptance{}, errors.New("state: invalid stop acceptance time")
	}
	r.Signature, err = key.SignCanonical(r.unsigned())
	return r, err
}

func (s *Store) ReadServiceStopAcceptance(key *signing.Key, boot string) (ServiceStopAcceptance, error) {
	var r ServiceStopAcceptance
	if key == nil || !stopIDValid(boot) {
		return r, errors.New("state: invalid stop acceptance identity")
	}
	directory, err := s.DirectoryID()
	if err != nil {
		return r, err
	}
	raw, err := readInitializationFile(filepath.Join(s.Dir, "service-stop-"+boot+".json"))
	if err != nil {
		return r, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF || r.SchemaVersion != "local-service-stop-acceptance/v1" || r.Action != "stop_accepted" ||
		r.BootID != boot || r.DirectoryID != directory || !stopIDValid(r.RequestSHA256) || r.AcceptedAt <= 0 || !signing.VerifyCanonical(key.Public(), r.unsigned(), r.Signature) {
		return ServiceStopAcceptance{}, errors.New("state: invalid stop acceptance record")
	}
	return r, nil
}

func (s *Store) RecordServiceStopAcceptance(w *Writer, key *signing.Key, request localcontrol.Message, now time.Time) (ServiceStopAcceptance, error) {
	if err := s.serviceWriter(w); err != nil {
		return ServiceStopAcceptance{}, err
	}
	directory, err := s.DirectoryID()
	if err != nil {
		return ServiceStopAcceptance{}, err
	}
	if request.DirectoryID != directory {
		return ServiceStopAcceptance{}, errors.New("state: stop request belongs to another directory")
	}
	r, err := newServiceStopAcceptance(key, request, now)
	if err != nil {
		return ServiceStopAcceptance{}, err
	}
	saved, err := s.ReadServiceStopAcceptance(key, r.BootID)
	if err == nil {
		if saved.RequestSHA256 != r.RequestSHA256 || saved.AcceptedAt >= request.ExpiresAt || saved.AcceptedAt < request.ExpiresAt-30 {
			return ServiceStopAcceptance{}, errors.New("state: conflicting stop acceptance record")
		}
		return saved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return ServiceStopAcceptance{}, err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return ServiceStopAcceptance{}, err
	}
	if err := publishCommitFile(filepath.Join(s.Dir, "service-stop-"+r.BootID+".json"), raw); err != nil {
		return ServiceStopAcceptance{}, err
	}
	return r, nil
}
