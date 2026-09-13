package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

func TestTaskActivitiesPaginationAndAuth(t *testing.T) {
	s, _ := newServer(t, "block")
	agent := "agent-1"
	for i := 0; i < 3; i++ {
		r := receipt.Receipt{ReceiptID: "r", Platform: "hermes", SessionID: "s", AgentID: &agent, TaskID: "t", IntentID: "i", IntentDigest: "digest", IntentBinding: "bound"}
		if i == 1 {
			r.IntentBinding = "unbound"
		}
		if i == 2 {
			r.TaskID = "other"
		}
		if err := s.d.Chain.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	request := func(path, method, credential string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://127.0.0.1:47611"+path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec
	}
	first := request("/v1/task-activities?limit=1", "GET", s.bootAdmin)
	if first.Code != 200 || first.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(first.Code, first.Body.String())
	}
	var page taskActivityPage
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 1 || page.Next == nil || *page.Next != 1 || page.History != "unknown" {
		t.Fatalf("wrong page %+v", page)
	}
	raw, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-activities.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(want, raw) {
		t.Fatal("contract fixture differs", err)
	}
	next := request("/v1/task-activities?limit=1&offset=1&snapshot="+page.Snapshot, "GET", s.bootAdmin)
	if next.Code != 200 {
		t.Fatal(next.Body.String())
	}
	var tail taskActivityPage
	_ = json.Unmarshal(next.Body.Bytes(), &tail)
	if tail.Next != nil || len(tail.Items) != 1 || tail.Items[0].ID == page.Items[0].ID {
		t.Fatal("pagination repeated activity")
	}
	unknown := request("/v1/task-activities?view=unassigned", "GET", s.bootAdmin)
	var unassigned taskActivityPage
	_ = json.Unmarshal(unknown.Body.Bytes(), &unassigned)
	if unknown.Code != 200 || len(unassigned.Items) != 1 || unassigned.Items[0].Binding != nil || unassigned.Items[0].Attribution != "unknown" {
		t.Fatal("unassigned activity missing")
	}
	for _, query := range []string{"?offset=1", "?limit=0", "?limit=101", "?limit=01", "?limit=1&limit=2", "?view=other", "?unknown=x", "?snapshot=bad", "?offset=-1", "?x=%zz"} {
		if r := request("/v1/task-activities"+query, "GET", s.bootAdmin); r.Code != 400 {
			t.Fatal("query accepted", query, r.Code)
		}
	}
	for credential, want := range map[string]int{"": 401, token: 403} {
		if r := request("/v1/task-activities", "GET", credential); r.Code != want {
			t.Fatal("auth failed", r.Code)
		}
	}
	if r := request("/v1/task-activities", "POST", s.bootAdmin); r.Code != 405 {
		t.Fatal("write method accepted")
	}
	r := receipt.Receipt{ReceiptID: "new"}
	if err := s.d.Chain.Append(&r); err != nil {
		t.Fatal(err)
	}
	if response := request("/v1/task-activities?offset=1&snapshot="+page.Snapshot, "GET", s.bootAdmin); response.Code != 409 {
		t.Fatal("mixed snapshots", response.Code)
	}
}

func TestTaskActivitiesEmptyAndCorrupt(t *testing.T) {
	s, st := newServer(t, "block")
	read := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://127.0.0.1:47611/v1/task-activities", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("Authorization", "Bearer "+s.bootAdmin)
		out := httptest.NewRecorder()
		s.Handler().ServeHTTP(out, req)
		return out
	}
	if out := read(); out.Code != 200 {
		t.Fatal("empty snapshot unavailable", out.Code)
	}
	r := receipt.Receipt{ReceiptID: "original"}
	if err := s.d.Chain.Append(&r); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(st.Dir, "receipts", "local", "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte("original"), []byte("tampered"), 1)
	if err := os.WriteFile(files[0], raw, 0600); err != nil {
		t.Fatal(err)
	}
	out := read()
	if out.Code != 500 || bytes.Contains(out.Body.Bytes(), []byte("items")) || bytes.Contains(out.Body.Bytes(), []byte("tampered")) {
		t.Fatal("corrupt history exposed", out.Code, out.Body.String())
	}
}
