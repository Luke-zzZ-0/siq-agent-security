package grant

import (
	"bytes"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/signing"
)

func skillAdmission(skillID, version, contentHash string) admission.Admission {
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
	return adm
}

func skillKey(t *testing.T) *signing.Key {
	t.Helper()
	k, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestBuildCarriesSkillRefAndVerifies(t *testing.T) {
	ver := "1.2.0"
	adm := skillAdmission("marketplace:skill:report-gen@"+strings.Repeat("ab", 6), ver, strings.Repeat("c", 64))
	res, err := Build(adm, Options{Subject: Subject{Type: "agent_instance", ID: "inst_1"}, Platform: "hermes", Key: skillKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Grant.Skill == nil {
		t.Fatal("grant must carry the admitted skill identity")
	}
	sk := res.Grant.Skill
	if sk.SkillID != adm.SkillID || sk.ContentHash != adm.ContentHash || sk.Version == nil || *sk.Version != ver {
		t.Fatalf("skill ref mismatch: %+v", sk)
	}
	// Deployment path re-signs; the skill ref must survive signing round-trips.
	g, err := Approve(res.Grant, Approval{ActorType: "human", ActorID: "u", ApprovedAt: "2026-09-13T00:00:00Z"}, skillKey(t))
	if err != nil {
		t.Fatal(err)
	}
	g, err = MarkDeployed(g, skillKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if g.Skill == nil || g.Skill.SkillID != adm.SkillID {
		t.Fatal("skill ref lost across signed transitions")
	}
	if !Verify(skillKey(t).Public(), g) {
		t.Fatal("signature must verify with skill ref present")
	}
}

func TestBuildWithoutSkillIdentityStaysBaseline(t *testing.T) {
	adm := skillAdmission("", "", strings.Repeat("c", 64))
	res, err := Build(adm, Options{Subject: Subject{Type: "agent_instance", ID: "inst_1"}, Platform: "hermes", Key: skillKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Grant.Skill != nil {
		t.Fatal("admission without skill id must not produce a skill-scoped grant")
	}
}

func TestSkillRefOfRequiresFullIdentity(t *testing.T) {
	adm := skillAdmission("marketplace:skill:report-gen@abcdef", "1.0.0", strings.Repeat("c", 64))
	if skillRefOf(adm) == nil {
		t.Fatal("full identity must produce a skill ref")
	}
	adm.ContentHash = ""
	if skillRefOf(adm) != nil {
		t.Fatal("missing content hash must not produce a skill ref")
	}
	adm2 := skillAdmission("", "", strings.Repeat("c", 64))
	if skillRefOf(adm2) != nil {
		t.Fatal("missing skill id must not produce a skill ref")
	}
}
