package receipt

import (
	"bytes"
	"reflect"
	"testing"

	"siq-agent-security/apps/agentshield/internal/signing"
)

func TestTaskActivityBoundariesAndIntegrity(t *testing.T) {
	key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	chain, err := OpenChain(t.TempDir(), "tasks", key)
	if err != nil {
		t.Fatal(err)
	}
	agent := "agent-1"
	base := Receipt{ReceiptID: "r", Platform: "hermes", SessionID: "session-1", AgentID: &agent, TaskID: "task-1", IntentID: "intent-1", IntentDigest: "digest-1", IntentBinding: "bound", Action: ActionAllow}
	changes := []func(*Receipt){
		func(*Receipt) {},
		func(r *Receipt) { r.SessionID = "session-2" },
		func(r *Receipt) { other := "agent-2"; r.AgentID = &other },
		func(r *Receipt) { r.Platform = "openclaw" },
		func(r *Receipt) { r.IntentID = "intent-2" },
		func(r *Receipt) { r.IntentDigest = "digest-2" },
		func(r *Receipt) { r.TaskID = "task-2" },
		func(r *Receipt) { r.IntentBinding = "unbound" },
		func(r *Receipt) { r.AgentID = nil },
		func(r *Receipt) { r.IntentDigest = "" },
		func(r *Receipt) { r.AuthorityStatus = "invalid"; r.Action = ActionDeny },
	}
	var all []Receipt
	for _, change := range changes {
		r := base
		change(&r)
		if err := chain.Append(&r); err != nil {
			t.Fatal(err)
		}
		all = append(all, r)
	}
	result, err := ProjectTaskActivities(all, key.Public(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 7 || !reflect.DeepEqual(result.Tasks[0].ReceiptIndexes, []int{0, 10}) || !reflect.DeepEqual(result.Unassigned, []int{7, 8, 9}) {
		t.Fatalf("incorrect grouping: %+v", result)
	}
	if result.Verification.HistoryIntegrity == HistoryVerified {
		t.Fatal("unanchored history claimed complete")
	}
	for i := 1; i < 7; i++ {
		if !reflect.DeepEqual(result.Tasks[i].ReceiptIndexes, []int{i}) {
			t.Fatal("cross-boundary join")
		}
	}
	all[0].TaskID = "tampered"
	result, err = ProjectTaskActivities(all, key.Public(), nil)
	if err == nil || len(result.Tasks) != 0 || len(result.Unassigned) != 0 || result.Verification.PrefixValid {
		t.Fatal("trusted tampered receipt")
	}
}
