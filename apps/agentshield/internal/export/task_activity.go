package export

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

// ActivityProjection is internal; source verification applies to the full input,
// not to these redacted rows. A transport must supply its own derived contract.
type ActivityProjection struct {
	SourceVerification receipt.VerificationReport
	Rows               []ActivityRow
}

type ActivityRow struct {
	Seq                                               int
	IssuedAt, ReceiptRef, ToolRef, Action, SourceHash string
}

func activityRef(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ProjectActivity verifies before selecting. Callers must additionally enforce
// disk read limits and compare the input with the engine's authoritative tip.
func ProjectActivity(all []receipt.Receipt, public []byte, checkpoint *receipt.Checkpoint, key receipt.TaskActivityKey, limit int) (ActivityProjection, error) {
	if limit < 1 || limit > 100000 || len(all) > 100000 {
		return ActivityProjection{}, errors.New("activity export: invalid budget")
	}
	groups, err := receipt.ProjectTaskActivities(all, public, checkpoint)
	if err != nil {
		return ActivityProjection{}, errors.New("activity export: invalid source")
	}
	for _, group := range groups.Tasks {
		if group.Key != key {
			continue
		}
		if len(group.ReceiptIndexes) > limit {
			return ActivityProjection{}, errors.New("activity export: budget exceeded")
		}
		result := ActivityProjection{SourceVerification: groups.Verification, Rows: make([]ActivityRow, 0, len(group.ReceiptIndexes))}
		for _, index := range group.ReceiptIndexes {
			r := all[index]
			action := "unknown"
			switch r.Action {
			case "allow", "deny", "hold", "redact":
				action = r.Action
			}
			// Do not preserve arbitrary text through a nominal timestamp field.
			issued := ""
			if t, err := time.Parse(time.RFC3339Nano, r.IssuedAt); err == nil {
				issued = t.UTC().Format(time.RFC3339Nano)
			}
			result.Rows = append(result.Rows, ActivityRow{Seq: r.Seq, IssuedAt: issued, ReceiptRef: activityRef(r.ReceiptID), ToolRef: activityRef(r.Tool), Action: action, SourceHash: r.Hash})
		}
		return result, nil
	}
	return ActivityProjection{}, errors.New("activity export: activity not found")
}
