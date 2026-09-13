package rawcontent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"siq-agent-security/apps/agentshield/internal/signing"
)

var activationMu sync.Mutex

// Activation is the immutable signed enable record and the source of truth for
// raw-content limits after restart. It grants no task permission by itself.
type Activation struct {
	SchemaVersion    string `json:"schema_version"`
	Enabled          bool   `json:"enabled"`
	ActorRef         string `json:"actor_ref"`
	ActivatedAt      string `json:"activated_at"`
	RetentionSeconds int    `json:"retention_seconds"`
	BudgetBytes      int64  `json:"budget_bytes"`
	KeyFingerprint   string `json:"key_fingerprint"`
	SigningSchema    string `json:"signing_schema"`
	Signature        string `json:"signature"`
}

func activationPath(stateDir string) string {
	return filepath.Join(authorityDir(stateDir), "activation.json")
}

func activationMap(activation Activation) map[string]any {
	return map[string]any{
		"schema_version":    activation.SchemaVersion,
		"enabled":           activation.Enabled,
		"actor_ref":         activation.ActorRef,
		"activated_at":      activation.ActivatedAt,
		"retention_seconds": activation.RetentionSeconds,
		"budget_bytes":      activation.BudgetBytes,
		"key_fingerprint":   activation.KeyFingerprint,
		"signing_schema":    activation.SigningSchema,
	}
}

func validActivation(activation Activation, key *signing.Key) bool {
	_, timeErr := time.Parse(time.RFC3339Nano, activation.ActivatedAt)
	limits := Limits{Retention: time.Duration(activation.RetentionSeconds) * time.Second, Budget: activation.BudgetBytes}
	return key != nil && activation.SchemaVersion == "local-raw-task-content-activation/v1" && activation.Enabled &&
		digestRefPattern.MatchString(activation.ActorRef) && timeErr == nil && limits.valid() &&
		digestRefPattern.MatchString(activation.KeyFingerprint) && activation.SigningSchema == signing.SchemaLocalCanonicalV1 &&
		lowerHex(activation.Signature, 64) && signing.VerifyCanonical(key.Public(), activationMap(activation), activation.Signature)
}

// OpenActivated has no side effects. A key or directory without a valid signed
// activation remains disabled or invalid rather than becoming active.
func OpenActivated(stateDir string, key *signing.Key) (*Store, *Authority, Activation, error) {
	var activation Activation
	if stateDir == "" || key == nil {
		return nil, nil, activation, ErrInvalid
	}
	dir := authorityDir(stateDir)
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil, activation, ErrDisabled
	}
	if err := privateDir(dir); err != nil {
		return nil, nil, activation, err
	}
	if err := readAuthorityRecord(activationPath(stateDir), &activation); errors.Is(err, os.ErrNotExist) {
		return nil, nil, Activation{}, ErrDisabled
	} else if err != nil || !validActivation(activation, key) {
		return nil, nil, Activation{}, ErrState
	}
	limits := Limits{Retention: time.Duration(activation.RetentionSeconds) * time.Second, Budget: activation.BudgetBytes}
	store, err := OpenExisting(stateDir, limits)
	if err != nil {
		return nil, nil, Activation{}, err
	}
	if activation.KeyFingerprint != "sha256:"+store.KeyFingerprint() {
		return nil, nil, Activation{}, ErrState
	}
	authority, err := OpenAuthorityExisting(stateDir, key, store)
	if err != nil {
		return nil, nil, Activation{}, err
	}
	return store, authority, activation, nil
}

// Activate explicitly initializes the encrypted store and publishes one signed
// immutable source of truth. Retries with the same limits are idempotent.
func Activate(stateDir string, key *signing.Key, limits Limits, actorID string, now time.Time) (*Store, *Authority, Activation, error) {
	activationMu.Lock()
	defer activationMu.Unlock()
	var zero Activation
	actor, actorOK := hashRef(actorID)
	if stateDir == "" || key == nil || !limits.valid() || !actorOK || now.IsZero() || limits.Retention%time.Second != 0 {
		return nil, nil, zero, ErrInvalid
	}
	if store, authority, existing, err := OpenActivated(stateDir, key); err == nil {
		if existing.RetentionSeconds != int(limits.Retention/time.Second) || existing.BudgetBytes != limits.Budget {
			return nil, nil, zero, ErrConflict
		}
		return store, authority, existing, nil
	} else if !errors.Is(err, ErrDisabled) {
		return nil, nil, zero, err
	}
	store, err := Initialize(stateDir, limits)
	if err != nil {
		return nil, nil, zero, err
	}
	authority, err := InitializeAuthority(stateDir, key, store)
	if err != nil {
		return nil, nil, zero, err
	}
	activation := Activation{
		SchemaVersion:    "local-raw-task-content-activation/v1",
		Enabled:          true,
		ActorRef:         actor,
		ActivatedAt:      now.UTC().Format(time.RFC3339Nano),
		RetentionSeconds: int(limits.Retention / time.Second),
		BudgetBytes:      limits.Budget,
		KeyFingerprint:   "sha256:" + store.KeyFingerprint(),
		SigningSchema:    signing.SchemaLocalCanonicalV1,
	}
	activation.Signature, err = key.SignCanonical(activationMap(activation))
	if err != nil {
		return nil, nil, zero, ErrState
	}
	raw, err := marshalIndented(activation)
	if err != nil {
		return nil, nil, zero, ErrState
	}
	if err := publishAuthority(activationPath(stateDir), raw); errors.Is(err, ErrConflict) {
		openedStore, openedAuthority, existing, openErr := OpenActivated(stateDir, key)
		if openErr == nil && existing.RetentionSeconds == activation.RetentionSeconds && existing.BudgetBytes == activation.BudgetBytes {
			return openedStore, openedAuthority, existing, nil
		}
		return nil, nil, zero, ErrConflict
	} else if err != nil {
		return nil, nil, zero, err
	}
	return store, authority, activation, nil
}

func marshalIndented(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}
