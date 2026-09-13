package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"siq-agent-security/apps/agentshield/internal/intent"
	"siq-agent-security/apps/agentshield/internal/rawcontent"
	"siq-agent-security/apps/agentshield/internal/runtimeidentity"
)

type rawTaskContentStatus struct {
	SchemaVersion    string  `json:"schema_version"`
	Status           string  `json:"status"`
	DefaultCapture   bool    `json:"default_capture"`
	RetentionSeconds *int    `json:"retention_seconds"`
	BudgetBytes      *int64  `json:"budget_bytes"`
	ActivatedAt      *string `json:"activated_at"`
}

type rawTaskContentActivateRequest struct {
	SchemaVersion    string `json:"schema_version"`
	ActorID          string `json:"actor_id"`
	RetentionSeconds int    `json:"retention_seconds"`
	BudgetBytes      int64  `json:"budget_bytes"`
}

type rawTaskContentGrantCreateRequest struct {
	SchemaVersion     string   `json:"schema_version"`
	TaskID            string   `json:"task_id"`
	Kinds             []string `json:"kinds"`
	ActorID           string   `json:"actor_id"`
	DurationSeconds   int      `json:"duration_seconds"`
	RetentionSeconds  int      `json:"retention_seconds"`
	MaxPlaintextBytes int      `json:"max_plaintext_bytes"`
}

type rawTaskContentRevokeRequest struct {
	SchemaVersion          string `json:"schema_version"`
	ExpectedGrantSignature string `json:"expected_grant_signature"`
	ActorID                string `json:"actor_id"`
}

type rawTaskContentCapturePermitRequest struct {
	SchemaVersion          string `json:"schema_version"`
	Platform               string `json:"platform"`
	AgentID                string `json:"agent_id"`
	SessionID              string `json:"session_id"`
	TaskID                 string `json:"task_id"`
	GrantID                string `json:"grant_id"`
	ExpectedGrantSignature string `json:"expected_grant_signature"`
	Kind                   string `json:"kind"`
	TTLSeconds             int    `json:"ttl_seconds"`
}

type rawTaskContentCaptureRequest struct {
	SchemaVersion string                   `json:"schema_version"`
	Platform      string                   `json:"platform"`
	AgentID       string                   `json:"agent_id"`
	SessionID     string                   `json:"session_id"`
	TaskID        string                   `json:"task_id"`
	Permit        rawcontent.CapturePermit `json:"permit"`
	Fields        []rawcontent.Field       `json:"fields"`
}

type rawTaskContentNativeCaptureRequest struct {
	SchemaVersion string             `json:"schema_version"`
	Platform      string             `json:"platform"`
	AgentID       string             `json:"agent_id"`
	SessionID     string             `json:"session_id"`
	Kind          string             `json:"kind"`
	Fields        []rawcontent.Field `json:"fields"`
}

func (request *rawTaskContentCaptureRequest) UnmarshalJSON(raw []byte) error {
	var nested struct {
		Permit json.RawMessage   `json:"permit"`
		Fields []json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(raw, &nested); err != nil ||
		!exactJSONObject(
			nested.Permit,
			"schema_version", "permit_id", "grant_id", "expected_grant_signature", "runtime_identity_ref", "session_ref", "binding_ref", "task_ref", "kind", "issued_at", "expires_at", "signing_schema", "signature",
		) {
		return errors.New("invalid raw task content capture")
	}
	for _, field := range nested.Fields {
		if !exactJSONObject(field, "path", "value", "secret") {
			return errors.New("invalid raw task content field")
		}
	}
	type plainCaptureRequest rawTaskContentCaptureRequest
	var decoded plainCaptureRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*request = rawTaskContentCaptureRequest(decoded)
	return nil
}

func (request *rawTaskContentNativeCaptureRequest) UnmarshalJSON(raw []byte) error {
	var nested struct {
		Fields []json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(raw, &nested); err != nil {
		return err
	}
	for _, field := range nested.Fields {
		if !exactJSONObject(field, "path", "value", "secret") {
			return errors.New("invalid native raw task content capture")
		}
	}
	type plainNativeCaptureRequest rawTaskContentNativeCaptureRequest
	var decoded plainNativeCaptureRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*request = rawTaskContentNativeCaptureRequest(decoded)
	return nil
}

type rawTaskContentCaptureResult struct {
	SchemaVersion string `json:"schema_version"`
	RecordID      string `json:"record_id"`
	TaskRef       string `json:"task_ref"`
	Kind          string `json:"kind"`
	CreatedAt     string `json:"created_at"`
	ExpiresAt     string `json:"expires_at"`
	PlaintextHash string `json:"plaintext_sha256"`
	PlaintextSize int    `json:"plaintext_bytes"`
	OmittedCount  int    `json:"omitted_secret_count"`
}

type rawTaskContentRecordListRequest struct {
	SchemaVersion string `json:"schema_version"`
	TaskID        string `json:"task_id"`
}

type rawTaskContentRecordDeleteRequest struct {
	SchemaVersion   string `json:"schema_version"`
	TaskID          string `json:"task_id"`
	ConfirmRecordID string `json:"confirm_record_id"`
}

type rawTaskContentPurgeRequest struct {
	SchemaVersion      string `json:"schema_version"`
	ConfirmExpiredOnly bool   `json:"confirm_expired_only"`
}

type rawTaskContentRecordContent struct {
	SchemaVersion     string                     `json:"schema_version"`
	ContainsPlaintext bool                       `json:"contains_plaintext"`
	Record            rawcontent.Metadata        `json:"record"`
	Fields            []rawTaskContentPlainField `json:"fields"`
}

type rawTaskContentPlainField struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

// refreshRawContentLocked must be called with rawMu held after construction.
// It revalidates the signed activation and encryption-key binding on every
// management read instead of trusting an in-memory enabled flag.
func (s *Server) refreshRawContentLocked() string {
	store, authority, activation, err := rawcontent.OpenActivated(s.d.Store.Dir, s.d.Key)
	s.rawStore, s.rawAuthority, s.rawActivation = nil, nil, rawcontent.Activation{}
	if errors.Is(err, rawcontent.ErrDisabled) {
		return "disabled"
	}
	if err != nil {
		return "error"
	}
	s.rawStore, s.rawAuthority, s.rawActivation = store, authority, activation
	return "ready"
}

// PurgeExpiredRawContent is the serve lifecycle's maintenance entry point.
// A disabled optional store is a successful no-op. Invalid signed state fails
// this cleanup closed without making the default decision service unavailable.
func (s *Server) PurgeExpiredRawContent(now time.Time) error {
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	switch s.refreshRawContentLocked() {
	case "disabled":
		return nil
	case "error":
		return rawcontent.ErrState
	}
	_, err := s.rawStore.PurgeExpired(now)
	return err
}

func (s *Server) rawTaskContentStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	status := s.refreshRawContentLocked()
	response := rawTaskContentStatus{SchemaVersion: "local-raw-task-content-status/v1", Status: status, DefaultCapture: false}
	if status == "ready" {
		retention, budget, activatedAt := s.rawActivation.RetentionSeconds, s.rawActivation.BudgetBytes, s.rawActivation.ActivatedAt
		response.RetentionSeconds, response.BudgetBytes, response.ActivatedAt = &retention, &budget, &activatedAt
	}
	writeJSON(w, http.StatusOK, response)
}

func rawTaskContentError(w http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "raw_task_content_unavailable"
	switch {
	case errors.Is(err, rawcontent.ErrInvalid):
		status, code = http.StatusBadRequest, "raw_task_content_invalid_request"
	case errors.Is(err, rawcontent.ErrConflict):
		status, code = http.StatusConflict, "raw_task_content_activation_conflict"
	}
	writeJSON(w, status, map[string]string{"error": code, "reason_code": code})
}

func rawTaskContentAuthorityError(w http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "raw_task_content_unavailable"
	switch {
	case errors.Is(err, rawcontent.ErrInvalid):
		status, code = http.StatusBadRequest, "raw_task_content_invalid_request"
	case errors.Is(err, rawcontent.ErrDisabled):
		status, code = http.StatusConflict, "raw_task_content_disabled"
	case errors.Is(err, rawcontent.ErrConflict):
		status, code = http.StatusConflict, "raw_task_content_authority_conflict"
	case errors.Is(err, rawcontent.ErrDenied):
		status, code = http.StatusForbidden, "raw_task_content_authority_denied"
	case errors.Is(err, rawcontent.ErrExpired):
		status, code = http.StatusGone, "raw_task_content_authority_expired"
	case errors.Is(err, rawcontent.ErrRevoked):
		status, code = http.StatusConflict, "raw_task_content_authority_revoked"
	case errors.Is(err, os.ErrNotExist):
		status, code = http.StatusNotFound, "raw_task_content_grant_not_found"
	case errors.Is(err, rawcontent.ErrBudget):
		status, code = http.StatusInsufficientStorage, "raw_task_content_capacity"
	}
	writeJSON(w, status, map[string]string{"error": code, "reason_code": code})
}

func (s *Server) readyRawAuthorityLocked(w http.ResponseWriter) (*rawcontent.Authority, bool) {
	switch s.refreshRawContentLocked() {
	case "disabled":
		rawTaskContentAuthorityError(w, rawcontent.ErrDisabled)
		return nil, false
	case "error":
		rawTaskContentAuthorityError(w, rawcontent.ErrState)
		return nil, false
	default:
		return s.rawAuthority, true
	}
}

func (s *Server) rawTaskContentActivation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		s.rawMu.Lock()
		defer s.rawMu.Unlock()
		switch s.refreshRawContentLocked() {
		case "disabled":
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "raw_task_content_disabled"})
		case "error":
			rawTaskContentError(w, rawcontent.ErrState)
		default:
			writeJSON(w, http.StatusOK, s.rawActivation)
		}
	case http.MethodPost:
		var request rawTaskContentActivateRequest
		if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "actor_id", "retention_seconds", "budget_bytes") {
			return
		}
		if request.SchemaVersion != "local-raw-task-content-activate/v1" {
			rawTaskContentError(w, rawcontent.ErrInvalid)
			return
		}
		if request.RetentionSeconds < int(rawcontent.MinRetention/time.Second) || request.RetentionSeconds > int(rawcontent.MaxRetention/time.Second) ||
			request.BudgetBytes < rawcontent.MinBudget || request.BudgetBytes > rawcontent.MaxBudget {
			rawTaskContentError(w, rawcontent.ErrInvalid)
			return
		}
		limits := rawcontent.Limits{Retention: time.Duration(request.RetentionSeconds) * time.Second, Budget: request.BudgetBytes}
		s.rawMu.Lock()
		defer s.rawMu.Unlock()
		wasReady := s.refreshRawContentLocked() == "ready"
		store, authority, activation, err := rawcontent.Activate(s.d.Store.Dir, s.d.Key, limits, request.ActorID, time.Now())
		if err != nil {
			rawTaskContentError(w, err)
			return
		}
		s.rawStore, s.rawAuthority, s.rawActivation = store, authority, activation
		status := http.StatusCreated
		if wasReady {
			status = http.StatusOK
		}
		writeJSON(w, status, activation)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) rawTaskContentGrants(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	authority, ok := s.readyRawAuthorityLocked(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		views, err := authority.List(time.Now())
		if err != nil {
			rawTaskContentAuthorityError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"schema_version": "local-raw-task-content-grants/v1", "items": views})
	case http.MethodPost:
		var request rawTaskContentGrantCreateRequest
		if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "task_id", "kinds", "actor_id", "duration_seconds", "retention_seconds", "max_plaintext_bytes") {
			return
		}
		if request.SchemaVersion != "local-raw-task-content-grant-create/v1" ||
			request.DurationSeconds < int(rawcontent.MinGrantDuration/time.Second) || request.DurationSeconds > int(rawcontent.MaxGrantDuration/time.Second) ||
			request.RetentionSeconds < int(rawcontent.MinRetention/time.Second) || request.RetentionSeconds > s.rawActivation.RetentionSeconds ||
			request.MaxPlaintextBytes < 1 || request.MaxPlaintextBytes > rawcontent.MaxPlaintext {
			rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
			return
		}
		grant, err := authority.Issue(
			request.TaskID,
			request.Kinds,
			request.ActorID,
			time.Duration(request.DurationSeconds)*time.Second,
			time.Duration(request.RetentionSeconds)*time.Second,
			request.MaxPlaintextBytes,
			time.Now(),
		)
		if err != nil {
			rawTaskContentAuthorityError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, grant)
	}
}

func (s *Server) rawTaskContentGrant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tail := strings.TrimPrefix(r.URL.Path, "/v1/raw-task-content/grants/")
	revoke := strings.HasSuffix(tail, "/revoke")
	if revoke {
		tail = strings.TrimSuffix(tail, "/revoke")
	}
	if tail == "" || strings.Contains(tail, "/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	authority, ok := s.readyRawAuthorityLocked(w)
	if !ok {
		return
	}
	if !revoke {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		view, err := authority.Inspect(tail, time.Now())
		if err != nil {
			rawTaskContentAuthorityError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request rawTaskContentRevokeRequest
	if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "expected_grant_signature", "actor_id") {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-revoke/v1" {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	revocation, err := authority.Revoke(tail, request.ExpectedGrantSignature, request.ActorID, time.Now())
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revocation)
}

func (s *Server) authorizeRawTaskContentRuntime(w http.ResponseWriter, r *http.Request, platform, agent, session string) (runtimeidentity.Record, intent.Binding, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "runtime_identity_required", "reason_code": "runtime_identity_required"})
		return runtimeidentity.Record{}, intent.Binding{}, false
	}
	credential := strings.TrimPrefix(header, "Bearer ")
	record, binding, err := s.runtimeIdentities.AuthorizeSessionContext(credential, platform, agent, session)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "scoped_runtime_session_required", "reason_code": "scoped_runtime_session_required"})
		return runtimeidentity.Record{}, intent.Binding{}, false
	}
	return record, binding, true
}

func (s *Server) rawTaskContentCapturePermit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request rawTaskContentCapturePermitRequest
	if !readStrictFlatRequest(
		w, r, &request, "raw_task_content_invalid_request",
		"schema_version", "platform", "agent_id", "session_id", "task_id", "grant_id", "expected_grant_signature", "kind", "ttl_seconds",
	) {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-capture-permit-create/v1" ||
		request.TTLSeconds < int(rawcontent.MinPermitDuration/time.Second) || request.TTLSeconds > int(rawcontent.MaxPermitDuration/time.Second) {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	record, binding, ok := s.authorizeRawTaskContentRuntime(w, r, request.Platform, request.AgentID, request.SessionID)
	if !ok {
		return
	}
	if binding.TaskID != request.TaskID {
		rawTaskContentAuthorityError(w, rawcontent.ErrDenied)
		return
	}
	runtimeExpires, err := time.Parse(time.RFC3339Nano, binding.ExpiresAt)
	if err != nil {
		rawTaskContentAuthorityError(w, rawcontent.ErrState)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	authority, ready := s.readyRawAuthorityLocked(w)
	if !ready {
		return
	}
	permit, err := authority.IssueCapturePermit(
		request.GrantID, request.ExpectedGrantSignature, record.IdentityID, request.SessionID, binding.BindingID,
		request.TaskID, request.Kind, runtimeExpires, time.Duration(request.TTLSeconds)*time.Second, time.Now(),
	)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, permit)
}

func (s *Server) rawTaskContentCapture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request rawTaskContentCaptureRequest
	if !readStrictRequestLimit(
		w, r, &request, "raw_task_content_invalid_request", 2<<20,
		"schema_version", "platform", "agent_id", "session_id", "task_id", "permit", "fields",
	) {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-capture/v1" {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	record, binding, ok := s.authorizeRawTaskContentRuntime(w, r, request.Platform, request.AgentID, request.SessionID)
	if !ok {
		return
	}
	if binding.TaskID != request.TaskID {
		rawTaskContentAuthorityError(w, rawcontent.ErrDenied)
		return
	}
	prepared, err := rawcontent.Prepare(request.Permit.Kind, request.Fields)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	authority, ready := s.readyRawAuthorityLocked(w)
	if !ready {
		return
	}
	envelope, err := authority.CaptureWithPermit(
		request.Permit, record.IdentityID, request.SessionID, binding.BindingID, request.TaskID, prepared, time.Now(),
	)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rawCaptureResult(envelope))
}

func rawCaptureResult(envelope rawcontent.Envelope) rawTaskContentCaptureResult {
	return rawTaskContentCaptureResult{
		SchemaVersion: "local-raw-task-content-capture-result/v1",
		RecordID:      envelope.RecordID,
		TaskRef:       envelope.TaskRef,
		Kind:          envelope.Kind,
		CreatedAt:     envelope.CreatedAt,
		ExpiresAt:     envelope.ExpiresAt,
		PlaintextHash: envelope.PlaintextHash,
		PlaintextSize: envelope.PlaintextSize,
		OmittedCount:  envelope.OmittedCount,
	}
}

func (s *Server) rawTaskContentNativeCapture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request rawTaskContentNativeCaptureRequest
	if !readStrictRequestLimit(
		w, r, &request, "raw_task_content_invalid_request", 2<<20,
		"schema_version", "platform", "agent_id", "session_id", "kind", "fields",
	) {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-native-capture/v1" {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	record, binding, ok := s.authorizeRawTaskContentRuntime(w, r, request.Platform, request.AgentID, request.SessionID)
	if !ok {
		return
	}
	prepared, err := rawcontent.Prepare(request.Kind, request.Fields)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	runtimeExpires, err := time.Parse(time.RFC3339Nano, binding.ExpiresAt)
	if err != nil {
		rawTaskContentAuthorityError(w, rawcontent.ErrState)
		return
	}
	now := time.Now()
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	authority, ready := s.readyRawAuthorityLocked(w)
	if !ready {
		return
	}
	grant, err := authority.ResolveActive(binding.TaskID, request.Kind, now)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	permit, err := authority.IssueCapturePermit(
		grant.GrantID, grant.Signature, record.IdentityID, request.SessionID, binding.BindingID,
		binding.TaskID, request.Kind, runtimeExpires, rawcontent.MinPermitDuration, now,
	)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	envelope, err := authority.CaptureWithPermit(
		permit, record.IdentityID, request.SessionID, binding.BindingID, binding.TaskID, prepared, now,
	)
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rawCaptureResult(envelope))
}

func (s *Server) readyRawStoreLocked(w http.ResponseWriter) (*rawcontent.Store, bool) {
	if _, ok := s.readyRawAuthorityLocked(w); !ok {
		return nil, false
	}
	return s.rawStore, true
}

func (s *Server) rawTaskContentRecordSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request rawTaskContentRecordListRequest
	if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "task_id") {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-record-list/v1" {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	store, ready := s.readyRawStoreLocked(w)
	if !ready {
		return
	}
	items, err := store.ListMetadata(request.TaskID, time.Now())
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schema_version": "local-raw-task-content-records/v1", "items": items})
}

func (s *Server) rawTaskContentRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tail := strings.TrimPrefix(r.URL.Path, "/v1/raw-task-content/records/")
	operation := ""
	for _, suffix := range []string{"/read", "/delete"} {
		if strings.HasSuffix(tail, suffix) {
			tail, operation = strings.TrimSuffix(tail, suffix), strings.TrimPrefix(suffix, "/")
			break
		}
	}
	if tail == "" || strings.Contains(tail, "/") || operation == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if operation == "read" {
		var request rawTaskContentRecordListRequest
		if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "task_id") {
			return
		}
		if request.SchemaVersion != "local-raw-task-content-record-read/v1" {
			rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
			return
		}
		s.rawMu.Lock()
		defer s.rawMu.Unlock()
		store, ready := s.readyRawStoreLocked(w)
		if !ready {
			return
		}
		fields, envelope, err := store.Read(request.TaskID, tail, time.Now())
		if err != nil {
			rawTaskContentAuthorityError(w, err)
			return
		}
		plainFields := make([]rawTaskContentPlainField, len(fields))
		for i, field := range fields {
			plainFields[i] = rawTaskContentPlainField{Path: field.Path, Value: field.Value}
		}
		writeJSON(w, http.StatusOK, rawTaskContentRecordContent{
			SchemaVersion: "local-raw-task-content-record-content/v1", ContainsPlaintext: true,
			Record: rawcontent.Metadata{
				SchemaVersion: "local-raw-task-content-record/v1", Status: "active", RecordID: envelope.RecordID,
				TaskRef: envelope.TaskRef, Kind: envelope.Kind, CreatedAt: envelope.CreatedAt, ExpiresAt: envelope.ExpiresAt,
				PlaintextHash: envelope.PlaintextHash, PlaintextSize: envelope.PlaintextSize, OmittedCount: envelope.OmittedCount,
			},
			Fields: plainFields,
		})
		return
	}
	var request rawTaskContentRecordDeleteRequest
	if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "task_id", "confirm_record_id") {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-record-delete/v1" || request.ConfirmRecordID != tail {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	store, ready := s.readyRawStoreLocked(w)
	if !ready {
		return
	}
	if err := store.Delete(request.TaskID, tail); err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schema_version": "local-raw-task-content-record-deleted/v1", "record_id": tail, "deleted": true})
}

func (s *Server) rawTaskContentPurgeExpired(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request rawTaskContentPurgeRequest
	if !readStrictFlatRequest(w, r, &request, "raw_task_content_invalid_request", "schema_version", "confirm_expired_only") {
		return
	}
	if request.SchemaVersion != "local-raw-task-content-purge-expired/v1" || !request.ConfirmExpiredOnly {
		rawTaskContentAuthorityError(w, rawcontent.ErrInvalid)
		return
	}
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	store, ready := s.readyRawStoreLocked(w)
	if !ready {
		return
	}
	result, err := store.PurgeExpired(time.Now())
	if err != nil {
		rawTaskContentAuthorityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version": "local-raw-task-content-purge-result/v1", "deleted_records": result.Deleted, "released_bytes": result.Bytes,
	})
}
