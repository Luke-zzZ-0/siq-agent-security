package intent

// HistoricalGrantSubject must be derived from a verified receipt, never an
// untrusted request body. It is an internal lookup key, not a wire contract.
type HistoricalGrantSubject struct {
	Platform, SessionID, AgentID, TaskID, IntentID, IntentDigest, AuthorityRevision, MatchedGrantID string
}

// HistoricalGrantReference describes a past signed selection only. It never
// resolves current permissions, authorizes execution, or proves Skill execution.
func (s *Store) HistoricalGrantReference(subject HistoricalGrantSubject) (GrantReference, error) {
	for _, value := range []string{subject.Platform, subject.SessionID, subject.AgentID, subject.TaskID, subject.IntentID, subject.IntentDigest, subject.AuthorityRevision, subject.MatchedGrantID} {
		if value == "" {
			return GrantReference{}, violation("history_binding_incomplete")
		}
	}
	b, err := s.GetBinding(bindingID(subject.Platform, subject.SessionID, subject.AgentID))
	if err != nil {
		return GrantReference{}, err
	}
	if b.Platform != subject.Platform || b.SessionID != subject.SessionID || b.AgentID != subject.AgentID || b.TaskID != subject.TaskID || b.IntentID != subject.IntentID || b.IntentDigest != subject.IntentDigest || b.AuthorityRevision != subject.AuthorityRevision {
		return GrantReference{}, violation("history_binding_mismatch")
	}
	if b.GrantRef == nil || b.GrantRef.GrantID != subject.MatchedGrantID || b.GrantRef.AdmissionID == "" || b.GrantRef.PermissionDigest == "" {
		return GrantReference{}, violation("history_grant_selection_missing")
	}
	c, err := s.Get(b.IntentID)
	if err != nil {
		return GrantReference{}, err
	}
	if c.Agent.Platform != b.Platform || c.Agent.ID != b.AgentID || c.TaskID != b.TaskID || c.Digest != b.IntentDigest || c.Authority.Revision != b.AuthorityRevision {
		return GrantReference{}, violation("history_intent_mismatch")
	}
	return *b.GrantRef, nil
}
