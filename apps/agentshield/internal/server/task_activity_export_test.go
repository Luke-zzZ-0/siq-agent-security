package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	exportpkg "siq-agent-security/apps/agentshield/internal/export"
	"siq-agent-security/apps/agentshield/internal/receipt"
	"siq-agent-security/apps/agentshield/internal/signing"
)

func TestTaskActivityExport(t *testing.T) {
	s, st := newServer(t, "block")
	seedRawExportSentinel(t, st.Dir)
	secret, agent := "PRIVATE_EXPORT_TEXT", "agent"
	for i := 0; i < 3; i++ {
		rc := receipt.Receipt{ReceiptID: secret, Tool: secret, Reason: secret, ParamsExcerpt: &secret, Platform: "hermes", AgentID: &agent, SessionID: "session", TaskID: "task", IntentID: "intent", IntentDigest: "digest", IntentBinding: "bound", Action: "allow"}
		if i == 1 {
			rc.SessionID = "other"
		}
		if err := s.d.Chain.Append(&rc); err != nil {
			t.Fatal(err)
		}
	}
	request := func(method, path, credential string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://127.0.0.1:47611"+path, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		out := httptest.NewRecorder()
		s.Handler().ServeHTTP(out, req)
		return out
	}
	var page taskActivityPage
	out := request("GET", "/v1/task-activities", s.bootAdmin)
	if err := json.Unmarshal(out.Body.Bytes(), &page); err != nil || out.Code != 200 {
		t.Fatal("list unavailable", err)
	}
	route := "/v1/task-activities/" + page.Items[0].ID + "/export"
	url := route + "?snapshot=" + page.Snapshot
	out = request("GET", url, s.bootAdmin)
	if out.Code != 200 || bytes.Contains(out.Body.Bytes(), []byte(secret)) || bytes.Contains(out.Body.Bytes(), []byte(rawExportSentinel)) || bytes.Contains(out.Body.Bytes(), []byte("raw-task-content")) || out.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(out.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("unsafe download", out.Code, out.Body.String())
	}
	var doc exportpkg.ActivityDocument
	if err := json.Unmarshal(out.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if err := exportpkg.VerifyActivity(s.d.Key.Public(), doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Receipts) != 2 || doc.Receipts[0].Seq != 0 || doc.Receipts[1].Seq != 2 || doc.SourceCount != 3 || doc.SourceLastSeq != 2 {
		t.Fatal("wrong scope")
	}
	// Normalize volatile time, then re-sign the fixture using the same real signer.
	doc.GeneratedAt = "2026-09-12T00:00:00Z"
	if err := exportpkg.SealActivity(s.d.Key, &doc); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(doc, "", "  ")
	raw = append(raw, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-activity-export.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatal("fixture differs", err)
	}
	other, _ := signing.FromSeed(bytes.Repeat([]byte{8}, 32))
	if exportpkg.VerifyActivity(other.Public(), doc) == nil {
		t.Fatal("wrong signer accepted")
	}
	doc.Receipts[0].Action = "deny"
	if exportpkg.VerifyActivity(s.d.Key.Public(), doc) == nil {
		t.Fatal("tamper accepted")
	}
	for _, tc := range []struct {
		method, path, credential string
		code                     int
	}{
		{"GET", url, "", 401}, {"GET", url, token, 403}, {"POST", url, s.bootAdmin, 405},
		{"GET", route, s.bootAdmin, 400}, {"GET", url + "&offset=0", s.bootAdmin, 400},
		{"GET", url + "&view=unassigned", s.bootAdmin, 400},
		{"GET", route + "?snapshot=" + strings.Repeat("0", 64), s.bootAdmin, 409},
		{"GET", "/v1/task-activities/" + strings.Repeat("0", 64) + "/export?snapshot=" + page.Snapshot, s.bootAdmin, 404},
	} {
		if got := request(tc.method, tc.path, tc.credential); got.Code != tc.code {
			t.Fatal(tc.path, got.Code, tc.code)
		}
	}
	rc := receipt.Receipt{ReceiptID: "new"}
	if err := s.d.Chain.Append(&rc); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", url, s.bootAdmin); got.Code != 409 || got.Header().Get("Content-Disposition") != "" {
		t.Fatal("stale snapshot downloaded")
	}
	files, err := filepath.Glob(filepath.Join(st.Dir, "receipts", "local", "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatal("missing source", err)
	}
	source, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	source = bytes.Replace(source, []byte(secret), []byte("TAMPERED"), 1)
	if err := os.WriteFile(files[0], source, 0600); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", url, s.bootAdmin); got.Code != 500 || got.Header().Get("Content-Disposition") != "" || strings.Contains(got.Body.String(), "signature") {
		t.Fatal("corrupt source downloaded")
	}

}
