package completion

import (
	"crypto/ed25519"
	"time"

	"siq-agent-security/apps/agentshield/internal/effectevidence"
)

// Subject is supplied by a verified task-activity binding, not by evidence.
type Subject struct{ Platform, SessionID, AgentID string }

// EvaluateForSubject prevents a task shared across sessions or actors from
// borrowing another group's successful effects. Evaluate remains the sole
// authority for interpreting the selected evidence and requirements.
func EvaluateForSubject(task Task, subject Subject, records []effectevidence.Record, pub ed25519.PublicKey, lookup ActionLookup, now time.Time) (Result, error) {
	if subject.Platform == "" || subject.SessionID == "" || subject.AgentID == "" || lookup == nil || len(records) > effectevidence.MaxRecords {
		return Result{}, ErrEvidence
	}
	type pair struct{ action, receipt string }
	actions := make(map[pair]effectevidence.Action)
	seen := make(map[string]bool)
	selected := make([]effectevidence.Record, 0)
	for _, r := range records {
		if r.Verify(pub, now) != nil || seen[r.Evidence.EvidenceID] {
			return Result{}, ErrEvidence
		}
		seen[r.Evidence.EvidenceID] = true
		if r.TaskID != task.ID {
			continue
		}
		p := pair{r.Evidence.ActionID, r.Evidence.DecisionReceiptID}
		a, ok := actions[p]
		if !ok {
			var err error
			a, err = lookup(p.action, p.receipt)
			if err != nil || a.ActionID != p.action || a.DecisionReceiptID != p.receipt || a.TaskID != r.TaskID {
				return Result{}, ErrEvidence
			}
			actions[p] = a
		}
		if a.IntentID != task.IntentID || a.IntentDigest != task.IntentDigest || a.Platform != subject.Platform || a.SessionID != subject.SessionID || a.AgentID != subject.AgentID {
			continue
		}
		selected = append(selected, r)
	}
	return Evaluate(task, selected, pub, func(action, receipt string) (effectevidence.Action, error) {
		a, ok := actions[pair{action, receipt}]
		if !ok {
			return effectevidence.Action{}, ErrEvidence
		}
		return a, nil
	}, now)
}
