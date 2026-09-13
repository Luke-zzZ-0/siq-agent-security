package export

import (
	"encoding/json"
	"errors"

	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/signing"
)

// ActivityDocument attests to a redacted projection, never a raw receipt chain.
type ActivityDocument struct {
	Schema        string              `json:"schema_version"`
	Scope         string              `json:"attestation_scope"`
	ActivityID    string              `json:"activity_id"`
	Snapshot      string              `json:"snapshot"`
	GeneratedAt   string              `json:"generated_at"`
	SourceCount   int                 `json:"source_count"`
	SourceTipHash string              `json:"source_tip_hash"`
	SourceLastSeq int                 `json:"source_last_seq"`
	PrefixValid   bool                `json:"prefix_valid"`
	History       string              `json:"history_integrity"`
	PublicKey     string              `json:"public_key_base64"`
	Receipts      []ActivityExportRow `json:"receipts"`
	SigningSchema string              `json:"signing_schema"`
	Signature     string              `json:"signature"`
}

type ActivityExportRow struct {
	Seq        int    `json:"seq"`
	IssuedAt   string `json:"issued_at"`
	ReceiptRef string `json:"receipt_ref"`
	ToolRef    string `json:"tool_ref"`
	Action     string `json:"action"`
	SourceHash string `json:"source_hash"`
}

func activityDocumentMap(doc ActivityDocument) (map[string]any, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	value, err := canon.Decode(raw)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("activity export: invalid document")
	}
	delete(m, "signature")
	return m, nil
}

func SealActivity(key *signing.Key, doc *ActivityDocument) error {
	if key == nil || doc == nil {
		return errors.New("activity export: missing signing input")
	}
	doc.Schema = "local-task-activity-export/v1"
	doc.Scope = AttestationShareProjectionOnly
	doc.PublicKey = key.PublicBase64()
	doc.SigningSchema = signing.SchemaLocalCanonicalV1
	doc.Signature = ""
	m, err := activityDocumentMap(*doc)
	if err != nil {
		return err
	}
	doc.Signature, err = key.SignCanonical(m)
	return err
}

// VerifyActivity requires an externally trusted public key. A key carried inside
// a downloaded document is not itself a trust anchor.
func VerifyActivity(public []byte, doc ActivityDocument) error {
	if doc.Signature == "" {
		return ErrUnsigned
	}
	if doc.Schema != "local-task-activity-export/v1" || doc.Scope != AttestationShareProjectionOnly {
		return ErrTampered
	}
	m, err := activityDocumentMap(doc)
	if err != nil {
		return err
	}
	if err := signing.VerifyDocument(public, m, doc.Signature); err != nil {
		return ErrTampered
	}
	return nil
}
