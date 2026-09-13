package server

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

func TestTaskActivitySearch(t *testing.T) {
	s, st := newServer(t, "block")
	agent := "agent"
	for i, task := range []string{"first", "报告-One", "third", "报告-Two", "loose"} {
		rc := receipt.Receipt{ReceiptID: task, Platform: "hermes", AgentID: &agent, SessionID: task, TaskID: task, IntentID: "intent", IntentDigest: "digest", IntentBinding: "bound", Action: "allow"}
		if i == 4 {
			rc.IntentBinding = "unbound"
		}
		if err := s.d.Chain.Append(&rc); err != nil {
			t.Fatal(err)
		}
	}
	read := func(query string) activitySearch {
		t.Helper()
		data := effectCall(t, s, "GET", "/v1/task-activities/search"+query, s.bootAdmin, nil, 200)
		raw, _ := json.Marshal(data)
		var out activitySearch
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := read("?q=" + url.QueryEscape("报告") + "&limit=1")
	if first.Total != 2 || len(first.Items) != 1 || first.Items[0].First != 1 || first.Next == nil || *first.Next != 1 {
		t.Fatalf("filter after pagination %+v", first)
	}
	second := read("?q=" + url.QueryEscape("报告") + "&limit=1&offset=1&snapshot=" + first.Snapshot)
	if second.Items[0].First != 3 || second.Next != nil {
		t.Fatal("wrong second filtered page")
	}
	full := effectCall(t, s, "GET", "/v1/task-activities", s.bootAdmin, nil, 200)
	if full["snapshot"] != first.Snapshot {
		t.Fatal("filtered snapshot lost source identity")
	}
	effectCall(t, s, "GET", "/v1/task-activities/"+first.Items[0].ID+"?snapshot="+first.Snapshot, s.bootAdmin, nil, 200)
	for query, want := range map[string]int{"?platform=hermes&agent_id=agent": 4, "?q=ONE": 1, "?q=%E6%8A%A5%E5%91%8A&task_id=third": 0, "?q=.*": 0, "?session_id=third": 1, "?view=unassigned&task_id=loose": 1} {
		if out := read(query); out.Total != want {
			t.Fatal(query, out.Total, want)
		}
	}
	unknown := read("?view=unassigned&task_id=loose")
	if unknown.Items[0].Binding != nil || unknown.Items[0].Attribution != "unknown" {
		t.Fatal("invented unknown binding")
	}
	raw, _ := json.MarshalIndent(first, "", "  ")
	raw = append(raw, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-activity-search.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatal("fixture differs", err)
	}
	for _, query := range []string{"?q=a&q=b", "?q=%FF", "?q=%00", "?unexpected=1", "?offset=1", "?q=" + strings.Repeat("a", 257)} {
		effectCall(t, s, "GET", "/v1/task-activities/search"+query, s.bootAdmin, nil, 400)
	}
	read("?q=" + url.QueryEscape(strings.Repeat("报", 256)))
	effectCall(t, s, "GET", "/v1/task-activities/search?offset=1&snapshot="+strings.Repeat("0", 64), s.bootAdmin, nil, 409)
	effectCall(t, s, "GET", "/v1/task-activities/search", token, nil, 403)
	effectCall(t, s, "GET", "/v1/task-activities/search", "", nil, 401)
	effectCall(t, s, "POST", "/v1/task-activities/search", s.bootAdmin, map[string]any{}, 405)
	rc := receipt.Receipt{ReceiptID: "new"}
	if err := s.d.Chain.Append(&rc); err != nil {
		t.Fatal(err)
	}
	effectCall(t, s, "GET", "/v1/task-activities/search?snapshot="+first.Snapshot, s.bootAdmin, nil, 409)
	files, err := filepath.Glob(filepath.Join(st.Dir, "receipts", "local", "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatal("missing ledger", err)
	}
	body, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.Replace(body, []byte("first"), []byte("tampered"), 1)
	if err := os.WriteFile(files[0], body, 0600); err != nil {
		t.Fatal(err)
	}
	effectCall(t, s, "GET", "/v1/task-activities/search?q=no-match", s.bootAdmin, nil, 500)

}
