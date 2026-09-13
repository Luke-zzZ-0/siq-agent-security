package state

import (
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/grant"
	"siq-agent-security/apps/agentshield/internal/receipt"
)

const (
	attribSkillID   = "marketplace:skill:report-gen@0a1b2c3d4e5f"
	attribSkillHash = "1111111111111111111111111111111111111111111111111111111111111111"
	attribVersion   = "1.2.0"
)

func putSkillGrant(t *testing.T, st *Store, platform, agentID, status, skillID, version, contentHash string, expiresAt *string) {
	t.Helper()
	g := grant.Grant{
		GrantID:       "grt-" + strings.Repeat("d", 12) + "-" + platform + "-" + agentID,
		AdmissionID:   "adm-attrib",
		Platform:      platform,
		Status:        status,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		DefaultEffect: "deny",
		ExpiresAt:     expiresAt,
		Subject:       grant.Subject{Type: "agent_instance", ID: agentID},
	}
	if skillID != "" && contentHash != "" {
		ref := grant.SkillRef{SkillID: skillID, ContentHash: contentHash}
		if version != "" {
			v := version
			ref.Version = &v
		}
		g.Skill = &ref
	}
	if err := st.PutGrant(g); err != nil {
		t.Fatal(err)
	}
}

func TestSkillAttributionCopiedMetadataCannotVerifyExecution(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	putSkillGrant(t, st, "hermes", "inst_1", "deployed", attribSkillID, attribVersion, attribSkillHash, nil)
	a := st.SkillAttribution("hermes", "sess-1", "inst_1", &receipt.SkillClaim{SkillID: attribSkillID, Version: attribVersion, ContentHash: attribSkillHash})
	if a == nil || a.Status != receipt.SkillAttributionUnknown {
		t.Fatalf("copied grant metadata without execution provenance must remain unknown: %+v", a)
	}
	if a.SkillID != attribSkillID || a.ContentHash != attribSkillHash || a.Version != attribVersion {
		t.Fatalf("unknown record should retain matching metadata: %+v", a)
	}
}

func TestSkillAttributionMismatchOnDrift(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	putSkillGrant(t, st, "hermes", "inst_1", "deployed", attribSkillID, attribVersion, attribSkillHash, nil)
	cases := map[string]receipt.SkillClaim{
		"content_drift":   {SkillID: attribSkillID, Version: attribVersion, ContentHash: strings.Repeat("9", 64)},
		"version_switch":  {SkillID: attribSkillID, Version: "9.9.9", ContentHash: attribSkillHash},
		"missing_hash":    {SkillID: attribSkillID, Version: attribVersion},
		"missing_version": {SkillID: attribSkillID, ContentHash: attribSkillHash},
	}
	for name, c := range cases {
		a := st.SkillAttribution("hermes", "sess-1", "inst_1", &c)
		if a == nil || a.Status != receipt.SkillAttributionMismatch {
			t.Fatalf("%s: known skill with drift must be mismatch: %+v", name, a)
		}
		if a.Status == receipt.SkillAttributionVerified {
			t.Fatalf("%s: must never verify", name)
		}
	}
}

func TestSkillAttributionUnknownForUnrelatedSkill(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	putSkillGrant(t, st, "hermes", "inst_1", "deployed", attribSkillID, attribVersion, attribSkillHash, nil)
	a := st.SkillAttribution("hermes", "sess-1", "inst_1", &receipt.SkillClaim{SkillID: "other:skill:pkg@ffffffffffff", ContentHash: attribSkillHash})
	if a != nil {
		t.Fatalf("unknown skill must return nil (engine records unknown): %+v", a)
	}
}

func TestSkillAttributionPerAgentAndPlatform(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	putSkillGrant(t, st, "hermes", "inst_2", "deployed", attribSkillID, attribVersion, attribSkillHash, nil)
	putSkillGrant(t, st, "openclaw", "inst_1", "deployed", attribSkillID, attribVersion, attribSkillHash, nil)
	claim := &receipt.SkillClaim{SkillID: attribSkillID, Version: attribVersion, ContentHash: attribSkillHash}
	// Same skill deployed to a different agent / platform must not attribute here.
	if a := st.SkillAttribution("hermes", "sess-1", "inst_1", claim); a != nil {
		t.Fatalf("cross-agent borrowing must not verify: %+v", a)
	}
	if a := st.SkillAttribution("openclaw", "sess-1", "inst_2", claim); a != nil {
		t.Fatalf("cross-platform borrowing must not verify: %+v", a)
	}
}

func TestSkillAttributionIgnoresExpiredAndNonLiveGrants(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano) // already expired
	putSkillGrant(t, st, "hermes", "inst_1", "deployed", attribSkillID, attribVersion, attribSkillHash, &future)
	putSkillGrant(t, st, "hermes", "inst_1", "approved", attribSkillID, attribVersion, attribSkillHash, nil)
	putSkillGrant(t, st, "hermes", "inst_1", "revoked", attribSkillID, attribVersion, attribSkillHash, nil)
	claim := &receipt.SkillClaim{SkillID: attribSkillID, Version: attribVersion, ContentHash: attribSkillHash}
	if a := st.SkillAttribution("hermes", "sess-1", "inst_1", claim); a != nil {
		t.Fatalf("expired/approved/revoked grants must not attribute: %+v", a)
	}
}

func TestSkillAttributionBaselineGrantNeverVerifies(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	putSkillGrant(t, st, "hermes", "inst_1", "deployed", "", "", "", nil)
	claim := &receipt.SkillClaim{SkillID: attribSkillID, Version: attribVersion, ContentHash: attribSkillHash}
	if a := st.SkillAttribution("hermes", "sess-1", "inst_1", claim); a != nil {
		t.Fatalf("baseline grant carries no skill identity to verify against: %+v", a)
	}
}

// TestSkillAttributionExactAmongMultipleSkills pins the multi-skill call
// boundary at the lookup layer: two live skill grants for the same agent must
// resolve independently, and a claim mixing skill A's id with skill B's hash
// must never verify against either.
func TestSkillAttributionExactAmongMultipleSkills(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	skillA, hashA := attribSkillID, attribSkillHash
	skillB, hashB := "marketplace:skill:web-sum@1b2c3d4e5f6a", strings.Repeat("2", 64)
	for _, g := range []struct {
		id, skill, hash string
	}{
		{"grt-" + strings.Repeat("a", 12) + "-multi", skillA, hashA},
		{"grt-" + strings.Repeat("b", 12) + "-multi", skillB, hashB},
	} {
		gg := grant.Grant{
			GrantID: g.id, AdmissionID: "adm-" + g.id[:8], Platform: "hermes",
			Status: "deployed", CreatedAt: time.Now().UTC().Format(time.RFC3339),
			DefaultEffect: "deny",
			Subject:       grant.Subject{Type: "agent_instance", ID: "inst_1"},
			Skill:         &grant.SkillRef{SkillID: g.skill, ContentHash: g.hash},
		}
		if err := st.PutGrant(gg); err != nil {
			t.Fatal(err)
		}
	}
	// Exact claims remain unknown without trusted execution provenance.
	for _, tc := range []struct{ skill, hash string }{{skillA, hashA}, {skillB, hashB}} {
		a := st.SkillAttribution("hermes", "sess-1", "inst_1", &receipt.SkillClaim{SkillID: tc.skill, ContentHash: tc.hash})
		if a == nil || a.Status != receipt.SkillAttributionUnknown || a.SkillID != tc.skill || a.ContentHash != tc.hash {
			t.Fatalf("claim %s must remain unknown: %+v", tc.skill, a)
		}
	}
	// Cross-mixed identity: skill A id + skill B hash matches nothing.
	a := st.SkillAttribution("hermes", "sess-1", "inst_1", &receipt.SkillClaim{SkillID: skillA, ContentHash: hashB})
	if a == nil || a.Status != receipt.SkillAttributionMismatch {
		t.Fatalf("mixed identity must mismatch, not verify: %+v", a)
	}
}
