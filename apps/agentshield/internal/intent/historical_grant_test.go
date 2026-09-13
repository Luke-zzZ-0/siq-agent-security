package intent

import (
	"bytes"
	"os"
	"siq-agent-security/apps/agentshield/internal/grant"
	"testing"
)

func TestHistoricalGrantScopeAndTamper(t *testing.T) {
	s, input, g := grantSelectionFixture(t)
	b, err := s.BindWithGrant(input, g.GrantID, 3)
	if err != nil {
		t.Fatal(err)
	}
	subject := HistoricalGrantSubject{b.Platform, b.SessionID, b.AgentID, b.TaskID, b.IntentID, b.IntentDigest, b.AuthorityRevision, g.GrantID}
	// Live authority is deliberately unavailable; history must not consult it.
	s.grants = func(string) (*grant.Grant, int, error) {
		t.Fatal("historical read consulted live grant")
		return nil, 0, os.ErrNotExist
	}
	ref, err := s.HistoricalGrantReference(subject)
	if err != nil || ref != *b.GrantRef {
		t.Fatal("historical reference unavailable", err)
	}
	if _, err := s.RevokeBinding(b.BindingID, b.IntentDigest); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ResolveBinding(b.Platform, b.SessionID, b.AgentID); err == nil {
		t.Fatal("revoked binding authorized")
	}
	if got, err := s.HistoricalGrantReference(subject); err != nil || got != ref {
		t.Fatal("revocation erased history", err)
	}
	changes := []func(*HistoricalGrantSubject){
		func(x *HistoricalGrantSubject) { x.Platform = "other" }, func(x *HistoricalGrantSubject) { x.SessionID = "other" },
		func(x *HistoricalGrantSubject) { x.AgentID = "other" }, func(x *HistoricalGrantSubject) { x.TaskID = "other" },
		func(x *HistoricalGrantSubject) { x.IntentID = "other" }, func(x *HistoricalGrantSubject) { x.IntentDigest = "other" },
		func(x *HistoricalGrantSubject) { x.AuthorityRevision = "other" }, func(x *HistoricalGrantSubject) { x.MatchedGrantID = "other" },
		func(x *HistoricalGrantSubject) { x.IntentDigest = "" },
	}
	for _, change := range changes {
		bad := subject
		change(&bad)
		if got, err := s.HistoricalGrantReference(bad); err == nil || got.GrantID != "" {
			t.Fatal("cross scope reference returned")
		}
	}
	p, _ := s.bindingPath(b.BindingID)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte(g.AdmissionID), []byte("tampered-admission"), 1)
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.HistoricalGrantReference(subject); err == nil || got.GrantID != "" {
		t.Fatal("tampered binding accepted")
	}
}

func TestHistoricalGrantMissingSelection(t *testing.T) {
	s, input, g := grantSelectionFixture(t)
	b, err := s.Bind(input)
	if err != nil {
		t.Fatal(err)
	}
	subject := HistoricalGrantSubject{b.Platform, b.SessionID, b.AgentID, b.TaskID, b.IntentID, b.IntentDigest, b.AuthorityRevision, g.GrantID}
	if got, err := s.HistoricalGrantReference(subject); err == nil || got.GrantID != "" {
		t.Fatal("invented selection from current grant")
	}
}
