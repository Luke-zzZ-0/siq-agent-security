package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/grant"
	"siq-agent-security/apps/agentshield/internal/intent"
	"siq-agent-security/apps/agentshield/internal/receipt"
)

func TestTaskActivitySources(t *testing.T) {
	s, st := newServer(t, "block")
	c, err := s.intents.Issue(apiIntent())
	if err != nil {
		t.Fatal(err)
	}
	sign := func(value any) string {
		t.Helper()
		raw, _ := json.Marshal(value)
		v, err := canon.Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]any)
		delete(m, "signature")
		sig, err := s.d.Key.SignCanonical(m)
		if err != nil {
			t.Fatal(err)
		}
		return sig
	}
	version := "1.2.3"
	a := admission.Admission{AdmissionID: "adm-history", SkillName: "history-skill", SkillVersion: &version, ContentHash: strings.Repeat("a", 64), Verdict: "admit", SigningSchema: "local_canonical/v1"}
	a.Signature = sign(a)
	if err := st.PutAdmission(&admission.Result{Admission: a}); err != nil {
		t.Fatal(err)
	}
	g := grant.Grant{GrantID: "grt-history", AdmissionID: a.AdmissionID, Platform: c.Agent.Platform, Subject: grant.Subject{Type: "agent_instance", ID: c.Agent.ID}, Status: "deployed", CreatedAt: "2026-01-01T00:00:00Z", SigningSchema: "local_canonical/v1"}
	g.Signature = sign(g)
	if err := st.PutGrant(g); err != nil {
		t.Fatal(err)
	}
	_, revision, err := st.GetGrantWithSeq(g.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.intents.BindWithGrant(intent.Binding{Platform: c.Agent.Platform, SessionID: "source-session", AgentID: c.Agent.ID, IntentID: c.IntentID}, g.GrantID, revision)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		rc := receipt.Receipt{ReceiptID: []string{"r0", "r1", "r2", "r3"}[i], Platform: b.Platform, SessionID: b.SessionID, AgentID: &b.AgentID, TaskID: b.TaskID, IntentID: b.IntentID, IntentDigest: b.IntentDigest, AuthorityRevision: b.AuthorityRevision, IntentBinding: "bound", MatchedGrantID: &g.GrantID, Action: "allow"}
		if i == 1 {
			rc.SessionID = "other-session"
		}
		if i == 3 {
			rc.IntentBinding = "unbound"
		}
		if err := s.d.Chain.Append(&rc); err != nil {
			t.Fatal(err)
		}
	}
	list := effectCall(t, s, "GET", "/v1/task-activities", s.bootAdmin, nil, 200)
	id := list["items"].([]any)[0].(map[string]any)["activity_id"].(string)
	route := "/v1/task-activities/" + id + "/sources"
	query := "?snapshot=" + list["snapshot"].(string)
	read := func(url string) activitySources {
		t.Helper()
		response := effectCall(t, s, "GET", url, s.bootAdmin, nil, 200)
		raw, _ := json.Marshal(response)
		var out activitySources
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := read(route + query + "&limit=1")
	if out.Total != 2 || out.Next == nil || *out.Next != 1 || len(out.Items) != 1 || out.Items[0].Seq != 0 || out.Items[0].Status != "verified_source" || out.Items[0].Source.ContentHash != a.ContentHash {
		t.Fatalf("wrong sources %+v", out)
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	raw = append(raw, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-activity-sources.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatal("fixture differs", err)
	}
	next := read(route + query + "&offset=1")
	if len(next.Items) != 1 || next.Items[0].Seq != 2 || next.Next != nil {
		t.Fatal("cross-session leak")
	}
	otherID := list["items"].([]any)[1].(map[string]any)["activity_id"].(string)
	other := read("/v1/task-activities/" + otherID + "/sources" + query)
	if other.Items[0].Status != "unavailable" || other.Items[0].Source != nil {
		t.Fatal("invented other session source")
	}
	if _, err := s.intents.RevokeBinding(b.BindingID, b.IntentDigest); err != nil {
		t.Fatal(err)
	}
	if revoked := read(route + query); revoked.Items[0].Status != "verified_source" {
		t.Fatal("revocation erased history")
	}
	path := filepath.Join(st.Dir, "admissions", a.AdmissionID+".json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(original, []byte("history-skill"), []byte("tampered-skill"), 1), 0600); err != nil {
		t.Fatal(err)
	}
	if bad := read(route + query); bad.Items[0].Source != nil || bad.Items[0].Status != "unavailable" {
		t.Fatal("tamper trusted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if missing := read(route + query); missing.Items[0].Source != nil {
		t.Fatal("missing source invented")
	}
	unknown := effectCall(t, s, "GET", "/v1/task-activities?view=unassigned", s.bootAdmin, nil, 200)
	unknownID := unknown["items"].([]any)[0].(map[string]any)["activity_id"].(string)
	if u := read("/v1/task-activities/" + unknownID + "/sources" + query + "&view=unassigned"); u.Items[0].Status != "unattributed" || u.Items[0].Source != nil {
		t.Fatal("unknown attribution invented")
	}
	effectCall(t, s, "GET", route+query, token, nil, 403)
	effectCall(t, s, "POST", route+query, s.bootAdmin, map[string]any{}, 405)
	effectCall(t, s, "GET", route, s.bootAdmin, nil, 400)
	effectCall(t, s, "GET", route+"?snapshot="+strings.Repeat("0", 64), s.bootAdmin, nil, 409)
	effectCall(t, s, "GET", "/v1/task-activities/"+strings.Repeat("0", 64)+"/sources"+query, s.bootAdmin, nil, 404)
	effectCall(t, s, "GET", route+query, "", nil, 401)
	for i := 0; i < 9; i++ {
		grantID := fmt.Sprintf("budget-%d", i)
		rc := receipt.Receipt{ReceiptID: grantID, Platform: b.Platform, SessionID: "budget-session", AgentID: &b.AgentID, TaskID: b.TaskID, IntentID: b.IntentID, IntentDigest: b.IntentDigest, AuthorityRevision: b.AuthorityRevision, IntentBinding: "bound", MatchedGrantID: &grantID}
		if err := s.d.Chain.Append(&rc); err != nil {
			t.Fatal(err)
		}
	}
	refreshed := effectCall(t, s, "GET", "/v1/task-activities", s.bootAdmin, nil, 200)
	budgetID := refreshed["items"].([]any)[2].(map[string]any)["activity_id"].(string)
	budgetURL := "/v1/task-activities/" + budgetID + "/sources?snapshot=" + refreshed["snapshot"].(string)
	effectCall(t, s, "GET", budgetURL+"&limit=8", s.bootAdmin, nil, 200)
	effectCall(t, s, "GET", budgetURL+"&limit=9", s.bootAdmin, nil, 413)
	effectCall(t, s, "GET", route+query, s.bootAdmin, nil, 409)

}
