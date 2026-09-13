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
	"siq-agent-security/apps/agentshield/internal/signing"
	"strconv"
	"strings"
)

type WindowsTaskRecord struct {
	SchemaVersion string `json:"schema_version"`
	InstanceID    string `json:"instance_id"`
	DirectoryID   string `json:"state_directory_id"`
	TaskName      string `json:"task_name"`
	XMLSHA256     string `json:"xml_sha256"`
	UserSID       string `json:"user_sid"`
	Signature     string `json:"signature"`
}

func (r WindowsTaskRecord) unsigned() map[string]any {
	return map[string]any{"schema_version": r.SchemaVersion, "instance_id": r.InstanceID, "state_directory_id": r.DirectoryID, "task_name": r.TaskName, "xml_sha256": r.XMLSHA256, "user_sid": r.UserSID}
}
func (s *Store) expectedWindowsTask(key *signing.Key, unit []byte, sid string) (WindowsTaskRecord, error) {
	var r WindowsTaskRecord
	if key == nil || len(unit) == 0 || len(unit) > 16384 || !WindowsUserSIDValid(sid) {
		return r, errors.New("state: invalid WindowsTask configuration")
	}
	instance, err := s.ReadLocalInstance()
	if err != nil {
		return r, err
	}
	directory, err := s.DirectoryID()
	if err != nil {
		return r, err
	}
	hash := sha256.Sum256(unit)
	return WindowsTaskRecord{SchemaVersion: "local-windows-task-record/v1", InstanceID: instance.InstanceID, DirectoryID: directory, TaskName: "\\SIQ-Agent-Security-" + instance.InstanceID, XMLSHA256: hex.EncodeToString(hash[:]), UserSID: sid}, nil
}
func verifyWindowsTaskRecord(raw []byte, key *signing.Key, expected WindowsTaskRecord) (WindowsTaskRecord, error) {
	var saved WindowsTaskRecord
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&saved) != nil || dec.Decode(new(any)) != io.EOF || !signing.VerifyCanonical(key.Public(), saved.unsigned(), saved.Signature) {
		return WindowsTaskRecord{}, errors.New("state: invalid WindowsTask ownership record")
	}
	expected.Signature = saved.Signature
	if expected != saved {
		return WindowsTaskRecord{}, errors.New("state: WindowsTask configuration changed; explicit migration required")
	}
	return saved, nil
}

// PrepareWindowsTask publishes signed intent before exclusive unit publication.
func (s *Store) PrepareWindowsTask(w *Writer, key *signing.Key, unit []byte, sid string) (WindowsTaskRecord, error) {
	var empty WindowsTaskRecord
	if err := s.serviceWriter(w); err != nil {
		return empty, err
	}
	record, err := s.expectedWindowsTask(key, unit, sid)
	if err != nil {
		return empty, err
	}
	recordPath := filepath.Join(s.Dir, "windows-task.json")
	unitPath := filepath.Join(s.Dir, strings.TrimPrefix(record.TaskName, `\`)+".xml")
	raw, err := readInitializationFile(recordPath)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := os.Lstat(unitPath); !errors.Is(err, os.ErrNotExist) {
			return empty, errors.New("state: unowned WindowsTask file exists")
		}
		record.Signature, err = key.SignCanonical(record.unsigned())
		if err != nil {
			return empty, err
		}
		raw, err = json.Marshal(record)
		if err != nil {
			return empty, err
		}
		if err = publishCommitFile(recordPath, raw); err != nil {
			return empty, err
		}
	} else if err != nil {
		return empty, err
	} else {
		record, err = verifyWindowsTaskRecord(raw, key, record)
		if err != nil {
			return empty, err
		}
	}
	existing, err := readInitializationFile(unitPath)
	if errors.Is(err, os.ErrNotExist) {
		err = publishCommitFile(unitPath, unit)
	} else if err == nil && !bytes.Equal(existing, unit) {
		err = errors.New("state: WindowsTask XML changed; restore before retrying")
	}
	if err != nil {
		return empty, err
	}
	return record, nil
}

// VerifyWindowsTask never writes, repairs or acquires the daemon's writer.
func (s *Store) VerifyWindowsTask(key *signing.Key, unit []byte, sid string) (WindowsTaskRecord, error) {
	r, err := s.expectedWindowsTask(key, unit, sid)
	if err != nil {
		return WindowsTaskRecord{}, err
	}
	raw, err := readInitializationFile(filepath.Join(s.Dir, "windows-task.json"))
	if err != nil {
		return WindowsTaskRecord{}, err
	}
	r, err = verifyWindowsTaskRecord(raw, key, r)
	if err != nil {
		return WindowsTaskRecord{}, err
	}
	actual, err := readInitializationFile(filepath.Join(s.Dir, strings.TrimPrefix(r.TaskName, `\`)+".xml"))
	if err != nil {
		return WindowsTaskRecord{}, err
	}
	if !bytes.Equal(actual, unit) {
		return WindowsTaskRecord{}, errors.New("state: WindowsTask configuration drift")
	}
	return r, nil
}

func WindowsUserSIDValid(sid string) bool {
	parts := strings.Split(sid, "-")
	if len(parts) < 4 || len(parts) > 18 || parts[0] != "S" || parts[1] != "1" {
		return false
	}
	if sid == "S-1-5-18" || sid == "S-1-5-19" || sid == "S-1-5-20" {
		return false
	}
	for i, part := range parts[2:] {
		bits := 32
		if i == 0 {
			bits = 48
		}
		n, err := strconv.ParseUint(part, 10, bits)
		if err != nil || strconv.FormatUint(n, 10) != part {
			return false
		}
	}
	return true
}
