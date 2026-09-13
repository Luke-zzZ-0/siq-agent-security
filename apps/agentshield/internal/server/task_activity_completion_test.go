package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

func TestActivityCompletionUnknownAndContract(t *testing.T) {
	s, _ := newServer(t, "block")
	contract := apiIntent()
	effectCall(t, s, "POST", "/v1/intents", s.bootAdmin, contract, 201)
	saved, err := s.intents.Get(contract.IntentID)
	if err != nil {
		t.Fatal(err)
	}
	r := receipt.Receipt{ReceiptID: "completion-activity", Platform: saved.Agent.Platform, AgentID: &saved.Agent.ID, SessionID: "s", TaskID: saved.TaskID, IntentID: saved.IntentID, IntentDigest: saved.Digest, IntentBinding: "bound"}
	if err := s.d.Chain.Append(&r); err != nil {
		t.Fatal(err)
	}
	list := effectCall(t, s, "GET", "/v1/task-activities", s.bootAdmin, nil, 200)
	id := list["items"].([]any)[0].(map[string]any)["activity_id"].(string)
	route := "/v1/task-activities/" + id + "/completion"
	out := effectCall(t, s, "GET", route, s.bootAdmin, nil, 200)
	if out["reason_code"] != "evaluated" || out["result"].(map[string]any)["status"] != "unknown" {
		t.Fatal("no requirements became verified", out)
	}
	if _, err := time.Parse(time.RFC3339Nano, out["evaluated_at"].(string)); err != nil {
		t.Fatal(err)
	}
	// Normalize only observation time for the cross-language output fixture.
	out["evaluated_at"] = "2026-09-12T00:00:00Z"
	raw, _ := json.MarshalIndent(out, "", "  ")
	raw = append(raw, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-activity-completion.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatal("fixture differs", err)
	}
	effectCall(t, s, "GET", route, token, nil, 403)
	effectCall(t, s, "POST", route, s.bootAdmin, map[string]any{"completed": true}, 405)
	effectCall(t, s, "GET", route+"?offset=0", s.bootAdmin, nil, 400)
	effectCall(t, s, "GET", route+"?snapshot="+strings.Repeat("0", 64), s.bootAdmin, nil, 409)
	r.IntentID = "missing"
	r.ReceiptID = "missing-intent"
	if err := s.d.Chain.Append(&r); err != nil {
		t.Fatal(err)
	}
	list = effectCall(t, s, "GET", "/v1/task-activities", s.bootAdmin, nil, 200)
	missingID := list["items"].([]any)[1].(map[string]any)["activity_id"].(string)
	missing := effectCall(t, s, "GET", "/v1/task-activities/"+missingID+"/completion", s.bootAdmin, nil, 200)
	if missing["reason_code"] != "intent_missing" || missing["result"] != nil {
		t.Fatal("missing authority hidden", missing)
	}
	r.IntentBinding = "unbound"
	r.ReceiptID = "unknown"
	if err := s.d.Chain.Append(&r); err != nil {
		t.Fatal(err)
	}
	list = effectCall(t, s, "GET", "/v1/task-activities?view=unassigned", s.bootAdmin, nil, 200)
	unknownID := list["items"].([]any)[0].(map[string]any)["activity_id"].(string)
	unknown := effectCall(t, s, "GET", "/v1/task-activities/"+unknownID+"/completion?view=unassigned", s.bootAdmin, nil, 200)
	if unknown["reason_code"] != "attribution_unknown" || unknown["result"] != nil {
		t.Fatal("invented attribution", unknown)
	}
}
