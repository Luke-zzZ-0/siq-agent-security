package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

func TestTaskActivityDetailIsolation(t *testing.T) {
	s, _ := newServer(t, "block")
	agent, excerpt, grant := "agent-a", "PRIVATE PARAMETER EXCERPT", "grant-a"
	for i := 0; i < 4; i++ {
		r := receipt.Receipt{ReceiptID: []string{"r0", "r1", "r2", "r3"}[i], Platform: "hermes", AgentID: &agent, SessionID: "s", TaskID: "t", IntentID: "i", IntentDigest: "digest", IntentBinding: "bound", Action: "allow", Tool: "read_file", MatchedGrantID: &grant, ParamsExcerpt: &excerpt}
		if i == 1 {
			r.SessionID = "different-session"
		}
		if i == 3 {
			r.IntentBinding = "unbound"
		}
		if err := s.d.Chain.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path, credential string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://127.0.0.1:47611"+path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		out := httptest.NewRecorder()
		s.Handler().ServeHTTP(out, req)
		return out
	}
	list := read("/v1/task-activities", s.bootAdmin)
	var page taskActivityPage
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	route := "/v1/task-activities/" + page.Items[0].ID
	first := read(route+"?limit=1&snapshot="+page.Snapshot, s.bootAdmin)
	if first.Code != 200 || bytes.Contains(first.Body.Bytes(), []byte(excerpt)) || bytes.Contains(first.Body.Bytes(), []byte("params_excerpt")) {
		t.Fatal("unsafe detail", first.Code, first.Body.String())
	}
	var detail taskActivityDetail
	if err := json.Unmarshal(first.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Total != 2 || len(detail.Receipts) != 1 || detail.Receipts[0].Seq != 0 || detail.Next == nil || *detail.Next != 1 {
		t.Fatalf("wrong detail %+v", detail)
	}
	raw, _ := json.MarshalIndent(detail, "", "  ")
	raw = append(raw, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-activity-detail.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatal("fixture differs", err)
	}
	second := read(route+"?offset=1&snapshot="+page.Snapshot, s.bootAdmin)
	if second.Code != 200 {
		t.Fatal(second.Code)
	}
	_ = json.Unmarshal(second.Body.Bytes(), &detail)
	if len(detail.Receipts) != 1 || detail.Receipts[0].Seq != 2 || detail.Next != nil {
		t.Fatal("other task leaked into detail")
	}
	for query, want := range map[string]int{"?offset=1": 400, "?snapshot=" + strings.Repeat("0", 64): 409, "?view=unassigned": 404} {
		if out := read(route+query, s.bootAdmin); out.Code != want {
			t.Fatal(query, out.Code, want)
		}
	}
	if out := read("/v1/task-activities/"+strings.Repeat("0", 64), s.bootAdmin); out.Code != 404 {
		t.Fatal("unknown activity exists")
	}
	for credential, want := range map[string]int{"": 401, token: 403} {
		if out := read(route, credential); out.Code != want {
			t.Fatal("auth bypass", out.Code)
		}
	}
	unknownList := read("/v1/task-activities?view=unassigned", s.bootAdmin)
	_ = json.Unmarshal(unknownList.Body.Bytes(), &page)
	out := read("/v1/task-activities/"+page.Items[0].ID+"?view=unassigned", s.bootAdmin)
	if out.Code != 200 {
		t.Fatal(out.Code)
	}
	_ = json.Unmarshal(out.Body.Bytes(), &detail)
	if len(detail.Receipts) != 1 || detail.Receipts[0].Seq != 3 || detail.Activity.Binding != nil {
		t.Fatal("unknown attribution invented")
	}
}
