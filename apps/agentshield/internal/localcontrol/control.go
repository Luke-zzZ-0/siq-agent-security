// Package localcontrol authorizes one graceful-stop request per service boot.
// It does not expose HTTP or release the service writer.
package localcontrol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/signing"
)

// StopDigest validates the request at acceptance time and binds its entire
// signed representation, including the signature, to the durable record.
func StopDigest(request Message, key *signing.Key, now time.Time) (string, error) {
	if !validMessage(request, key, now, stopSchema, "stop") {
		return "", errors.New("local-control: invalid stop request")
	}
	signed := request.unsigned()
	signed["signature"] = request.Signature
	raw, err := canon.Marshal(signed)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

const challengeSchema = "local-service-control-challenge/v1"
const stopSchema = "local-service-stop-request/v1"

type Message struct {
	SchemaVersion string `json:"schema_version"`
	Action        string `json:"action"`
	BootID        string `json:"boot_id"`
	DirectoryID   string `json:"state_directory_id"`
	ExpiresAt     int64  `json:"expires_at"`
	Signature     string `json:"signature"`
}

func (m Message) unsigned() map[string]any {
	return map[string]any{"schema_version": m.SchemaVersion, "action": m.Action, "boot_id": m.BootID, "state_directory_id": m.DirectoryID, "expires_at": m.ExpiresAt}
}

type Control struct {
	mu              sync.Mutex
	key             *signing.Key
	boot, directory string
	now             func() time.Time
	consumed        bool
}

func (c *Control) AcceptedBootID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.consumed {
		return ""
	}
	return c.boot
}

func validID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 32 && hex.EncodeToString(b) == id
}

func New(key *signing.Key, directory string, now func() time.Time) (*Control, error) {
	if key == nil || !validID(directory) || now == nil {
		return nil, errors.New("local-control: invalid dependencies")
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	return &Control{key: key, boot: hex.EncodeToString(nonce[:]), directory: directory, now: now}, nil
}

func (c *Control) Challenge() (Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.consumed {
		return Message{}, errors.New("local-control: stop already accepted")
	}
	m := Message{SchemaVersion: challengeSchema, Action: "challenge", BootID: c.boot, DirectoryID: c.directory, ExpiresAt: c.now().Unix() + 30}
	var err error
	m.Signature, err = c.key.SignCanonical(m.unsigned())
	return m, err
}

func validMessage(m Message, key *signing.Key, now time.Time, schema, action string) bool {
	return key != nil && m.SchemaVersion == schema && m.Action == action && validID(m.BootID) && validID(m.DirectoryID) &&
		m.ExpiresAt > now.Unix() && m.ExpiresAt <= now.Unix()+30 && signing.VerifyCanonical(key.Public(), m.unsigned(), m.Signature)
}

// SignStop verifies the challenge's origin, expiry and selected directory before
// signing a distinct request domain. The private key stays in the local CLI.
func SignStop(challenge Message, key *signing.Key, directory string, now time.Time) (Message, error) {
	if challenge.DirectoryID != directory || !validMessage(challenge, key, now, challengeSchema, "challenge") {
		return Message{}, errors.New("local-control: invalid challenge")
	}
	m := challenge
	m.SchemaVersion, m.Action = stopSchema, "stop"
	var err error
	m.Signature, err = key.SignCanonical(m.unsigned())
	return m, err
}

// Accept serializes acceptance with the caller's required durable record.
// Only a nil result authorizes the caller to request graceful draining.
func (c *Control) Accept(request Message, record func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.consumed || record == nil || request.BootID != c.boot || request.DirectoryID != c.directory || !validMessage(request, c.key, c.now(), stopSchema, "stop") {
		return errors.New("local-control: invalid, expired or consumed stop request")
	}
	if err := record(); err != nil {
		return errors.New("local-control: acceptance record failed")
	}
	c.consumed = true
	return nil
}
