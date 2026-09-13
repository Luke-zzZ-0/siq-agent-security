package receipt

import "errors"

// TaskActivityKey preserves every existing binding boundary. Task IDs alone
// are not a safe join key across agents, sessions, or intent revisions.
type TaskActivityKey struct {
	ChainID, Platform, SessionID, AgentID, TaskID, IntentID, IntentDigest string
}

type TaskActivity struct {
	Key            TaskActivityKey
	ReceiptIndexes []int
}

// TaskActivities refers only to the supplied immutable snapshot. Verification
// describes chain integrity, never permission validity or verified effects.
// These internal projection types are not a wire contract.
type TaskActivities struct {
	Verification VerificationReport
	Tasks        []TaskActivity
	Unassigned   []int
}

func ProjectTaskActivities(all []Receipt, public []byte, checkpoint *Checkpoint) (TaskActivities, error) {
	result := TaskActivities{Verification: VerifyDetailed(all, public, checkpoint), Tasks: []TaskActivity{}, Unassigned: []int{}}
	if !result.Verification.PrefixValid {
		return result, errors.New("task activities: receipt integrity invalid")
	}
	groups := make(map[TaskActivityKey]int)
	for i, r := range all {
		key := TaskActivityKey{ChainID: r.ChainID, Platform: r.Platform, SessionID: r.SessionID, AgentID: str(r.AgentID), TaskID: r.TaskID, IntentID: r.IntentID, IntentDigest: r.IntentDigest}
		if r.IntentBinding != "bound" || key.ChainID == "" || key.Platform == "" || key.SessionID == "" || key.AgentID == "" || key.TaskID == "" || key.IntentID == "" || key.IntentDigest == "" {
			result.Unassigned = append(result.Unassigned, i)
			continue
		}
		group, exists := groups[key]
		if !exists {
			group = len(result.Tasks)
			groups[key] = group
			result.Tasks = append(result.Tasks, TaskActivity{Key: key, ReceiptIndexes: []int{}})
		}
		result.Tasks[group].ReceiptIndexes = append(result.Tasks[group].ReceiptIndexes, i)
	}
	return result, nil
}
