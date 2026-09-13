package receipt

import (
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/grant"
	"siq-agent-security/apps/agentshield/internal/signing"
)

// skillGrant builds a deployed grant scoped to the given skill identity. It
// mirrors deployedGrant but sets the admission's skill fields so the grant
// becomes skill-scoped (UX-007).
func skillGrant(t *testing.T, platform, skillID, version, contentHash string) *grant.Grant {
	t.Helper()
	f := func(domain, action, rtype, value string) admission.DeclaredFact {
		return admission.DeclaredFact{Domain: domain, Action: action, Resource: admission.Resource{Type: rtype, Value: value},
			Effect: "allow", State: "declared", Authority: "skill_manifest", SourceField: "t", EvidenceIDs: []string{"ev-1"}}
	}
	adm := admission.Admission{AdmissionID: "adm-skill", SkillID: skillID, ContentHash: contentHash, Verdict: "admit_with_conditions", EvidenceIDs: []string{"ev-1"},
		DeclaredFacts: []admission.DeclaredFact{
			f("tool", "tool.invoke", "tool", "read_file"),
			f("filesystem", "fs.read", "path", "/home/u/proj"),
		}}
	if version != "" {
		v := version
		adm.SkillVersion = &v
	}
	k, err := signing.FromSeed([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := grant.Build(adm, grant.Options{Subject: grant.Subject{Type: "agent_instance", ID: "inst_1"}, Platform: platform, Key: k})
	if err != nil {
		t.Fatal(err)
	}
	if res.Grant.Skill == nil {
		t.Fatal("expected skill-scoped grant")
	}
	g, _ := grant.Approve(res.Grant, grant.Approval{ActorType: "human", ActorID: "u", ApprovedAt: "2026-09-13T00:00:00Z"}, k)
	g, _ = grant.MarkDeployed(g, k)
	return &g
}

func claim(skillID, version, contentHash string) *SkillClaim {
	c := &SkillClaim{SkillID: skillID, Version: version, ContentHash: contentHash}
	if version == "" {
		c.Version = ""
	}
	if contentHash == "" {
		c.ContentHash = ""
	}
	return c
}

const (
	skillID      = "marketplace:skill:report-gen@0a1b2c3d4e5f"
	skillHash    = "1111111111111111111111111111111111111111111111111111111111111111"
	skillVersion = "1.2.0"
	verifiedCall = "read_file"
)

func readCallPath() map[string]any {
	return map[string]any{"path": "/home/u/proj/a.txt"}
}

func allowedLookup(platform, agentID string, want *SkillAttribution) SkillAttributionLookup {
	return func(p, s, a string, c *SkillClaim) *SkillAttribution {
		if p == platform && a == agentID {
			return want
		}
		return nil
	}
}

func TestSkillScopedGrantAllowsVerifiedAttribution(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	fx.eng.opts.SkillAttribution = allowedLookup("hermes", "inst_1", &SkillAttribution{SkillID: skillID, Version: skillVersion, ContentHash: skillHash, Status: SkillAttributionVerified})
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d, err := fx.eng.Decide(r)
	if err != nil || d.Action != ActionAllow {
		t.Fatalf("verified attribution must allow: %v %+v", err, d)
	}
	a := d.Receipt.SkillAttribution
	if a == nil || a.Status != SkillAttributionVerified || a.SkillID != skillID || a.ContentHash != skillHash {
		t.Fatalf("receipt must record verified attribution: %+v", a)
	}
}

func TestSkillScopedGrantDeniesUnclaimedCall(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	d, _ := fx.eng.Decide(req("hermes", verifiedCall, readCallPath()))
	if d.Action != ActionDeny || !strings.Contains(d.Reason, "skill attribution does not verify") {
		t.Fatalf("unclaimed call must default deny: %+v", d)
	}
	if d.Receipt.ReasonCode != "skill_attribution_mismatch" {
		t.Fatalf("reason_code: %q", d.Receipt.ReasonCode)
	}
	if d.Receipt.SkillAttribution != nil {
		t.Fatal("unclaimed call must not carry skill_attribution on the receipt")
	}
}

func TestSkillScopedGrantDeniesForgedUnknownClaim(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	// No trusted record exists: the engine's lookup stays the default (unknown).
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d, _ := fx.eng.Decide(r)
	if d.Action != ActionDeny || d.Receipt.ReasonCode != "skill_attribution_mismatch" {
		t.Fatalf("forged claim must default deny: %+v", d)
	}
	a := d.Receipt.SkillAttribution
	if a == nil || a.Status != SkillAttributionUnknown {
		t.Fatalf("unverifiable claim must be recorded unknown, never verified: %+v", a)
	}
}

func TestSkillScopedGrantDeniesVersionSwitch(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	fx.eng.opts.SkillAttribution = allowedLookup("hermes", "inst_1", &SkillAttribution{SkillID: skillID, Version: "9.9.9", ContentHash: skillHash, Status: SkillAttributionMismatch})
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, "9.9.9", skillHash)
	d, _ := fx.eng.Decide(r)
	if d.Action != ActionDeny || d.Receipt.ReasonCode != "skill_attribution_mismatch" {
		t.Fatalf("switched version must deny: %+v", d)
	}
	if d.Receipt.SkillAttribution == nil || d.Receipt.SkillAttribution.Status != SkillAttributionMismatch {
		t.Fatalf("mismatch must be recorded: %+v", d.Receipt.SkillAttribution)
	}
}

func TestSkillScopedGrantDeniesBorrowedIdentity(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	// Lookup only verifies for agent inst_2: inst_1 borrowing another agent's
	// skill identity must not expand its own authorization.
	fx.eng.opts.SkillAttribution = allowedLookup("hermes", "inst_2", &SkillAttribution{SkillID: skillID, Version: skillVersion, ContentHash: skillHash, Status: SkillAttributionVerified})
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d, _ := fx.eng.Decide(r)
	if d.Action != ActionDeny {
		t.Fatalf("borrowed identity must not authorize another agent: %+v", d)
	}
	if d.Receipt.SkillAttribution == nil || d.Receipt.SkillAttribution.Status != SkillAttributionUnknown {
		t.Fatalf("unattributable agent claim stays unknown: %+v", d.Receipt.SkillAttribution)
	}
}

func TestBaselineGrantUnaffectedBySkillClaims(t *testing.T) {
	g := deployedGrant(t, "hermes", false)
	if g.Skill != nil {
		t.Fatal("fixture must be baseline")
	}
	fx := newFixture(t, "block", g, false)
	d, _ := fx.eng.Decide(req("hermes", verifiedCall, readCallPath()))
	if d.Action != ActionAllow {
		t.Fatalf("baseline allow without claim: %+v", d)
	}
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d, _ = fx.eng.Decide(r)
	if d.Action != ActionAllow {
		t.Fatalf("baseline allow with unknown claim: %+v", d)
	}
	if d.Receipt.SkillAttribution == nil || d.Receipt.SkillAttribution.Status != SkillAttributionUnknown {
		t.Fatalf("claim is still recorded honestly: %+v", d.Receipt.SkillAttribution)
	}
}

func TestMalformedSkillClaimStaysUnknown(t *testing.T) {
	fx := newFixture(t, "block", nil, false)
	for name, c := range map[string]*SkillClaim{
		"bad_skill_id":   {SkillID: "bad id with spaces", ContentHash: skillHash},
		"bad_hash":       {SkillID: skillID, ContentHash: "ZZZ"},
		"oversize_hash":  {SkillID: skillID, ContentHash: strings.Repeat("a", 65)},
		"bad_version":    {SkillID: skillID, Version: "v/" + strings.Repeat("x", 70)},
		"empty_skill_id": {SkillID: "", ContentHash: skillHash},
	} {
		r := req("hermes", verifiedCall, readCallPath())
		r.Skill = c
		d, err := fx.eng.Decide(r)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d.Receipt.SkillAttribution == nil || d.Receipt.SkillAttribution.Status != SkillAttributionUnknown {
			t.Fatalf("%s: malformed claim must record unknown: %+v", name, d.Receipt.SkillAttribution)
		}
		if d.Receipt.SkillAttribution.SkillID != "" || d.Receipt.SkillAttribution.ContentHash != "" {
			t.Fatalf("%s: malformed identity must not be echoed into the signed receipt: %+v", name, d.Receipt.SkillAttribution)
		}
	}
}

func TestNilLookupLeavesClaimsUnknown(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d, _ := fx.eng.Decide(r)
	if d.Action != ActionDeny {
		t.Fatalf("no lookup = fail closed: %+v", d)
	}
	if d.Receipt.SkillAttribution == nil || d.Receipt.SkillAttribution.Status != SkillAttributionUnknown {
		t.Fatalf("claim recorded unknown: %+v", d.Receipt.SkillAttribution)
	}
}

func TestAttributionLookupCannotReturnBogusStatus(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	// A misbehaving lookup claiming "verified" with an unrecognized status
	// value must not widen anything.
	fx.eng.opts.SkillAttribution = func(string, string, string, *SkillClaim) *SkillAttribution {
		return &SkillAttribution{SkillID: skillID, ContentHash: skillHash, Status: "totally-verified"}
	}
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d, _ := fx.eng.Decide(r)
	if d.Action != ActionDeny {
		t.Fatalf("bogus status must fail closed: %+v", d)
	}
}

func TestSkillAttributionLifetimeGuardedChain(t *testing.T) {
	g := skillGrant(t, "hermes", skillID, skillVersion, skillHash)
	fx := newFixture(t, "block", g, false)
	fx.eng.opts.SkillAttributionEnforced = true
	fx.eng.opts.SkillAttribution = allowedLookup("hermes", "inst_1", &SkillAttribution{SkillID: skillID, Version: skillVersion, ContentHash: skillHash, Status: SkillAttributionVerified})
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(skillID, skillVersion, skillHash)
	d1, err := fx.eng.Decide(r)
	if err != nil || d1.Action != ActionAllow {
		t.Fatalf("first decision: %v %+v", err, d1)
	}
	fx.clock = fx.clock.Add(2 * time.Second)
	d2, err := fx.eng.Decide(r)
	if err != nil {
		t.Fatal(err)
	}
	if d2.Receipt.Seq != d1.Receipt.Seq+1 {
		t.Fatalf("chain must append: seq %d then %d", d1.Receipt.Seq, d2.Receipt.Seq)
	}
	if d2.Receipt.SkillAttribution == nil || d2.Receipt.SkillAttribution.Status != SkillAttributionVerified {
		t.Fatalf("attribution carried into chained receipt: %+v", d2.Receipt.SkillAttribution)
	}
}

// TestMultiSkillBoundaryOnlyMatchingGrantServes pins the engine-side multi-
// skill boundary: when two skill-scoped grants are live for the same agent,
// only the grant whose skill identity matches the trusted attribution serves
// the call; the other grant must never be reached through it.
func TestMultiSkillBoundaryOnlyMatchingGrantServes(t *testing.T) {
	otherID, otherHash := "marketplace:skill:web-sum@1b2c3d4e5f6a", strings.Repeat("2", 64)
	grants := map[string]*grant.Grant{
		"a": skillGrant(t, "hermes", skillID, skillVersion, skillHash),
		"b": skillGrant(t, "hermes", otherID, "2.0.0", otherHash),
	}
	fx := newFixture(t, "block", nil, false)
	fx.eng.opts.SkillAttributionEnforced = true
	fx.eng.opts.Grants = func(p, a string) *grant.Grant {
		if p == "hermes" && a == "inst_1" {
			// The store would pick one live grant by its own rules; return
			// grant A so the test shows it serves only A-attributed calls.
			return grants["a"]
		}
		return nil
	}
	// Claim B while grant A is the live grant: attribution to B must not let
	// grant A (or B) authorize this call through a mismatched identity.
	fx.eng.opts.SkillAttribution = allowedLookup("hermes", "inst_1", &SkillAttribution{SkillID: otherID, Version: "2.0.0", ContentHash: otherHash, Status: SkillAttributionVerified})
	r := req("hermes", verifiedCall, readCallPath())
	r.Skill = claim(otherID, "2.0.0", otherHash)
	d, _ := fx.eng.Decide(r)
	if d.Action != ActionDeny || d.Receipt.ReasonCode != "skill_attribution_mismatch" {
		t.Fatalf("claim B must not be served by grant A: %+v", d)
	}
	// Same call claiming A verifies against grant A and allows.
	r2 := req("hermes", verifiedCall, readCallPath())
	r2.Skill = claim(skillID, skillVersion, skillHash)
	fx.eng.opts.SkillAttribution = allowedLookup("hermes", "inst_1", &SkillAttribution{SkillID: skillID, Version: skillVersion, ContentHash: skillHash, Status: SkillAttributionVerified})
	d2, err := fx.eng.Decide(r2)
	if err != nil || d2.Action != ActionAllow || d2.Receipt.MatchedGrantID == nil || *d2.Receipt.MatchedGrantID != grants["a"].GrantID {
		t.Fatalf("claim A must be served by grant A: %v %+v", err, d2)
	}
	a := d2.Receipt.SkillAttribution
	if a == nil || a.Status != SkillAttributionVerified || a.SkillID != skillID {
		t.Fatalf("receipt must record A's verified attribution: %+v", a)
	}
}
