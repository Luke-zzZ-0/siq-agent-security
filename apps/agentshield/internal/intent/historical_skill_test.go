package intent

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/grant"
	"siq-agent-security/apps/agentshield/internal/importsource"
)

func TestHistoricalSkillSource(t *testing.T) {
	for _, imported := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "import"}[imported], func(t *testing.T) {
			s, input, g := grantSelectionFixture(t)
			version := "1.2.3"
			a := admission.Admission{AdmissionID: g.AdmissionID, SkillName: "example", SkillVersion: &version, ContentHash: strings.Repeat("a", 64), Verdict: "admit", SigningSchema: "local_canonical/v1"}
			sign := func(a *admission.Admission) {
				t.Helper()
				raw, _ := json.Marshal(a)
				value, err := canon.Decode(raw)
				if err != nil {
					t.Fatal(err)
				}
				doc := value.(map[string]any)
				delete(doc, "signature")
				a.Signature, err = s.key.SignCanonical(doc)
				if err != nil {
					t.Fatal(err)
				}
			}
			sign(&a)
			if imported {
				source := importsource.Source{SchemaVersion: "local-skill-import-permission-source/v1", ImportID: "si-" + strings.Repeat("1", 32), ArtifactDigest: strings.Repeat("b", 64), AnalysisSHA256: strings.Repeat("c", 64)}
				var err error
				a, err = source.Bind(a, s.key)
				if err != nil {
					t.Fatal(err)
				}
				g.AdmissionID = a.AdmissionID
				raw, _ := json.Marshal(g)
				value, err := canon.Decode(raw)
				if err != nil {
					t.Fatal(err)
				}
				doc := value.(map[string]any)
				delete(doc, "signature")
				g.Signature, err = s.key.SignCanonical(doc)
				if err != nil {
					t.Fatal(err)
				}
			}
			b, err := s.BindWithGrant(input, g.GrantID, 3)
			if err != nil {
				t.Fatal(err)
			}
			subject := HistoricalGrantSubject{b.Platform, b.SessionID, b.AgentID, b.TaskID, b.IntentID, b.IntentDigest, b.AuthorityRevision, g.GrantID}
			s.grants = func(string) (*grant.Grant, int, error) { t.Fatal("read current grant"); return nil, 0, nil }
			reads := 0
			lookup := func(id string) (*admission.Admission, error) {
				reads++
				if id != a.AdmissionID {
					t.Fatal("wrong admission requested")
				}
				return &a, nil
			}
			out, err := s.HistoricalSkillSource(subject, lookup)
			if err != nil || out.ContentHash != a.ContentHash || out.DeclaredVersion == nil || *out.DeclaredVersion != version || reads != 1 {
				t.Fatal("historical source missing", err)
			}
			if imported && (out.Import == nil || out.Import.ArtifactDigest == out.ContentHash) {
				t.Fatal("conflated digest domains")
			}
			if !imported && out.Import != nil {
				t.Fatal("invented import")
			}
			*out.DeclaredVersion = "changed"
			if *a.SkillVersion != "1.2.3" {
				t.Fatal("mutable alias")
			}
			original := a
			a.SkillName = "tampered"
			if got, err := s.HistoricalSkillSource(subject, lookup); err == nil || got.ContentHash != "" {
				t.Fatal("tampered admission accepted")
			}
			a = original
			a.AdmissionID = "other"
			sign(&a)
			if _, err := s.HistoricalSkillSource(subject, func(string) (*admission.Admission, error) { return &a, nil }); err == nil {
				t.Fatal("wrong signed admission accepted")
			}
			a = original
			a.SkillName = "bad\nname"
			sign(&a)
			if _, err := s.HistoricalSkillSource(subject, lookup); err == nil {
				t.Fatal("control label accepted")
			}
			a = original
			a.SkillVersion = nil
			sign(&a)
			if got, err := s.HistoricalSkillSource(subject, lookup); err != nil || got.DeclaredVersion != nil {
				t.Fatal("missing version invented", err)
			}
			if imported {
				a = original
				bad := "{}"
				a.Source.Ref = &bad
				sign(&a)
				if _, err := s.HistoricalSkillSource(subject, lookup); err == nil {
					t.Fatal("invalid signed import source accepted")
				}
			}
			for _, missing := range []HistoricalAdmissionLookup{nil, func(string) (*admission.Admission, error) { return nil, errors.New("missing") }, func(string) (*admission.Admission, error) { return nil, nil }} {
				if _, err := s.HistoricalSkillSource(subject, missing); err == nil {
					t.Fatal("missing admission accepted")
				}
			}
		})
	}
}
