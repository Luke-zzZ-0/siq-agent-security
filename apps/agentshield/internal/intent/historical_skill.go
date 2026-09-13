package intent

import (
	"unicode"
	"unicode/utf8"

	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/importsource"
)

// HistoricalSkillSource is an internal metadata projection. Version is a
// manifest declaration; neither digest proves installation or execution.
type HistoricalSkillSource struct {
	Grant           GrantReference
	SkillName       string
	DeclaredVersion *string
	ContentHash     string
	Import          *importsource.Source
}

type HistoricalAdmissionLookup func(string) (*admission.Admission, error)

func historicalLabel(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 256 && len(value) <= 1024 && !containsControl(value)
}
func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// HistoricalSkillSource resolves only the immutable admission selected by the
// signed binding. The lookup must bound disk reads; no installed files are read.
func (s *Store) HistoricalSkillSource(subject HistoricalGrantSubject, lookup HistoricalAdmissionLookup) (HistoricalSkillSource, error) {
	ref, err := s.HistoricalGrantReference(subject)
	if err != nil {
		return HistoricalSkillSource{}, err
	}
	if lookup == nil {
		return HistoricalSkillSource{}, violation("history_admission_unavailable")
	}
	a, err := lookup(ref.AdmissionID)
	if err != nil || a == nil {
		return HistoricalSkillSource{}, violation("history_admission_unavailable")
	}
	if a.AdmissionID != ref.AdmissionID || !admission.Verify(s.key.Public(), *a) || !lowerHex(a.ContentHash, 32) || !historicalLabel(a.SkillName) {
		return HistoricalSkillSource{}, violation("history_admission_invalid")
	}
	out := HistoricalSkillSource{Grant: ref, SkillName: a.SkillName, ContentHash: a.ContentHash}
	if a.SkillVersion != nil {
		if !historicalLabel(*a.SkillVersion) {
			return HistoricalSkillSource{}, violation("history_admission_invalid")
		}
		version := *a.SkillVersion
		out.DeclaredVersion = &version
	}
	if importsource.Reserved(a.AdmissionID) {
		source, err := importsource.Parse(*a)
		if err != nil {
			return HistoricalSkillSource{}, violation("history_admission_invalid")
		}
		out.Import = &source
	}
	return out, nil
}
