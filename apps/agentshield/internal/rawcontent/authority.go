package rawcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"siq-agent-security/apps/agentshield/internal/statefs"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"siq-agent-security/apps/agentshield/internal/fileopen"
	"siq-agent-security/apps/agentshield/internal/signing"
)

const (
	MinGrantDuration    = time.Minute
	MaxGrantDuration    = 24 * time.Hour
	maxAuthorityRecords = 4096
	maxAuthorityBytes   = 32 << 10
)

var (
	authorityMu      sync.Mutex
	grantIDPattern   = regexp.MustCompile(`^rawgrant-[a-f0-9]{32}$`)
	digestRefPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

// Grant is immutable local authority for optional capture of selected content
// kinds for one task. It contains only hashes of task and actor identifiers.
type Grant struct {
	SchemaVersion     string   `json:"schema_version"`
	GrantID           string   `json:"grant_id"`
	TaskRef           string   `json:"task_ref"`
	Kinds             []string `json:"kinds"`
	ActorRef          string   `json:"actor_ref"`
	IssuedAt          string   `json:"issued_at"`
	ExpiresAt         string   `json:"expires_at"`
	RetentionSeconds  int      `json:"retention_seconds"`
	MaxPlaintextBytes int      `json:"max_plaintext_bytes"`
	SigningSchema     string   `json:"signing_schema"`
	Signature         string   `json:"signature"`
}

// Revocation is a terminal immutable withdrawal of a Grant. Binding the exact
// Grant signature prevents a tombstone from applying to different authority.
type Revocation struct {
	SchemaVersion          string `json:"schema_version"`
	GrantID                string `json:"grant_id"`
	ExpectedGrantSignature string `json:"expected_grant_signature"`
	ActorRef               string `json:"actor_ref"`
	RevokedAt              string `json:"revoked_at"`
	ReasonCode             string `json:"reason_code"`
	SigningSchema          string `json:"signing_schema"`
	Signature              string `json:"signature"`
}

type Authority struct {
	dir   string
	key   *signing.Key
	store *Store
}

type GrantView struct {
	SchemaVersion string      `json:"schema_version"`
	Status        string      `json:"status"`
	Grant         Grant       `json:"grant"`
	Revocation    *Revocation `json:"revocation"`
}

func authorityDir(stateDir string) string {
	return filepath.Join(stateDir, "raw-task-content-authority")
}

func ownsStore(stateDir string, store *Store) bool {
	if stateDir == "" || store == nil {
		return false
	}
	want, err := filepath.Abs(contentDir(stateDir))
	if err != nil {
		return false
	}
	got, err := filepath.Abs(store.dir)
	return err == nil && filepath.Clean(got) == filepath.Clean(want)
}

// OpenAuthorityExisting never creates state. The caller must already have an
// explicitly initialized raw-content Store and a local signing identity.
func OpenAuthorityExisting(stateDir string, key *signing.Key, store *Store) (*Authority, error) {
	if key == nil || !ownsStore(stateDir, store) {
		return nil, ErrInvalid
	}
	dir := authorityDir(stateDir)
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, ErrDisabled
	}
	if err := privateDir(dir); err != nil {
		return nil, err
	}
	return &Authority{dir: dir, key: key, store: store}, nil
}

// InitializeAuthority is reserved for the explicit management operation that
// enables task-level raw-content capture. It does not issue any Grant.
func InitializeAuthority(stateDir string, key *signing.Key, store *Store) (*Authority, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	if key == nil || !ownsStore(stateDir, store) || privateDir(store.dir) != nil {
		return nil, ErrInvalid
	}
	dir := authorityDir(stateDir)
	if err := statefs.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, ErrState
	}
	return OpenAuthorityExisting(stateDir, key, store)
}

func hashRef(value string) (string, bool) {
	if !safeText(value, 1024) || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > 256 {
		return "", false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:]), true
}

func allowedKind(kind string) bool {
	return kind == "input" || kind == "parameters" || kind == "output" || kind == "note"
}

func normalizeKinds(kinds []string) ([]string, bool) {
	if len(kinds) == 0 || len(kinds) > 4 {
		return nil, false
	}
	out := append([]string(nil), kinds...)
	sort.Strings(out)
	for i, kind := range out {
		if !allowedKind(kind) || i > 0 && kind == out[i-1] {
			return nil, false
		}
	}
	return out, true
}

func lowerHex(value string, bytes int) bool {
	if len(value) != bytes*2 || value != strings.ToLower(value) {
		return false
	}
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == bytes
}

func grantMap(grant Grant) map[string]any {
	kinds := make([]any, len(grant.Kinds))
	for i, kind := range grant.Kinds {
		kinds[i] = kind
	}
	return map[string]any{
		"schema_version":      grant.SchemaVersion,
		"grant_id":            grant.GrantID,
		"task_ref":            grant.TaskRef,
		"kinds":               kinds,
		"actor_ref":           grant.ActorRef,
		"issued_at":           grant.IssuedAt,
		"expires_at":          grant.ExpiresAt,
		"retention_seconds":   grant.RetentionSeconds,
		"max_plaintext_bytes": grant.MaxPlaintextBytes,
		"signing_schema":      grant.SigningSchema,
	}
}

func revocationMap(revocation Revocation) map[string]any {
	return map[string]any{
		"schema_version":           revocation.SchemaVersion,
		"grant_id":                 revocation.GrantID,
		"expected_grant_signature": revocation.ExpectedGrantSignature,
		"actor_ref":                revocation.ActorRef,
		"revoked_at":               revocation.RevokedAt,
		"reason_code":              revocation.ReasonCode,
		"signing_schema":           revocation.SigningSchema,
	}
}

func (a *Authority) grantPath(id string) (string, error) {
	if !grantIDPattern.MatchString(id) {
		return "", ErrInvalid
	}
	return filepath.Join(a.dir, id+".grant.json"), nil
}

func (a *Authority) revocationPath(id string) (string, error) {
	if !grantIDPattern.MatchString(id) {
		return "", ErrInvalid
	}
	return filepath.Join(a.dir, id+".revocation.json"), nil
}

func readAuthorityRecord(path string, out any) error {
	before, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() <= 0 || before.Size() > maxAuthorityBytes ||
		(runtime.GOOS != "windows" && before.Mode().Perm()&0077 != 0) {
		return ErrState
	}
	f, err := fileopen.Regular(path)
	if err != nil {
		return ErrState
	}
	after, statErr := f.Stat()
	if statErr != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() {
		_ = f.Close()
		return ErrState
	}
	decoder := json.NewDecoder(io.LimitReader(f, maxAuthorityBytes+1))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(out)
	var extra any
	extraErr := decoder.Decode(&extra)
	closeErr := f.Close()
	if err != nil || extraErr != io.EOF || closeErr != nil {
		return ErrState
	}
	return nil
}

func publishAuthority(path string, raw []byte) error {
	if len(raw) == 0 || len(raw) > maxAuthorityBytes || privateDir(filepath.Dir(path)) != nil {
		return ErrState
	}
	f, err := statefs.CreateTemp(filepath.Dir(path), ".pending-authority-*")
	if err != nil {
		return ErrState
	}
	defer statefs.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return ErrState
	}
	if err = statefs.Link(f.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrConflict
		}
		return ErrState
	}
	return nil
}

func (a *Authority) authorityRecordCount() (int, error) {
	entries, err := statefs.ReadDir(a.dir)
	if err != nil {
		return 0, ErrState
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pending-authority-") {
			continue
		}
		if entry.IsDir() || !(entry.Name() == "activation.json" || strings.HasSuffix(entry.Name(), ".grant.json") || strings.HasSuffix(entry.Name(), ".revocation.json")) {
			return 0, ErrState
		}
		count++
		if count > maxAuthorityRecords {
			return 0, ErrBudget
		}
	}
	return count, nil
}

func (a *Authority) Issue(taskID string, kinds []string, actorID string, duration, retention time.Duration, maxPlaintextBytes int, now time.Time) (Grant, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	var grant Grant
	task, taskOK := taskRef(taskID)
	actor, actorOK := hashRef(actorID)
	normalizedKinds, kindsOK := normalizeKinds(kinds)
	if !taskOK || !actorOK || !kindsOK || now.IsZero() || duration < MinGrantDuration || duration > MaxGrantDuration ||
		retention < MinRetention || retention > a.store.limits.Retention || retention%time.Second != 0 ||
		maxPlaintextBytes < 1 || maxPlaintextBytes > MaxPlaintext || privateDir(a.dir) != nil {
		return grant, ErrInvalid
	}
	count, err := a.authorityRecordCount()
	if err != nil || count >= maxAuthorityRecords {
		if err != nil {
			return grant, err
		}
		return grant, ErrBudget
	}
	idBytes := make([]byte, 16)
	if _, err := io.ReadFull(random, idBytes); err != nil {
		return grant, ErrState
	}
	grant = Grant{
		SchemaVersion:     "local-raw-task-content-grant/v1",
		GrantID:           "rawgrant-" + hex.EncodeToString(idBytes),
		TaskRef:           task,
		Kinds:             normalizedKinds,
		ActorRef:          actor,
		IssuedAt:          now.UTC().Format(time.RFC3339Nano),
		ExpiresAt:         now.Add(duration).UTC().Format(time.RFC3339Nano),
		RetentionSeconds:  int(retention / time.Second),
		MaxPlaintextBytes: maxPlaintextBytes,
		SigningSchema:     signing.SchemaLocalCanonicalV1,
	}
	grant.Signature, err = a.key.SignCanonical(grantMap(grant))
	if err != nil {
		return Grant{}, ErrState
	}
	path, _ := a.grantPath(grant.GrantID)
	revocationPath, _ := a.revocationPath(grant.GrantID)
	if _, err := os.Lstat(revocationPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return Grant{}, ErrConflict
	}
	raw, err := json.MarshalIndent(grant, "", "  ")
	if err != nil {
		return Grant{}, ErrState
	}
	raw = append(raw, '\n')
	if err := publishAuthority(path, raw); err != nil {
		return Grant{}, err
	}
	return grant, nil
}

func validGrant(grant Grant, key *signing.Key) bool {
	issued, issuedErr := time.Parse(time.RFC3339Nano, grant.IssuedAt)
	expires, expiresErr := time.Parse(time.RFC3339Nano, grant.ExpiresAt)
	kinds, kindsOK := normalizeKinds(grant.Kinds)
	if issuedErr != nil || expiresErr != nil || !kindsOK || len(kinds) != len(grant.Kinds) {
		return false
	}
	for i := range kinds {
		if kinds[i] != grant.Kinds[i] {
			return false
		}
	}
	duration := expires.Sub(issued)
	return key != nil && grant.SchemaVersion == "local-raw-task-content-grant/v1" && grantIDPattern.MatchString(grant.GrantID) &&
		digestRefPattern.MatchString(grant.TaskRef) && digestRefPattern.MatchString(grant.ActorRef) &&
		duration >= MinGrantDuration && duration <= MaxGrantDuration && grant.RetentionSeconds >= int(MinRetention/time.Second) &&
		grant.RetentionSeconds <= int(MaxRetention/time.Second) && grant.MaxPlaintextBytes >= 1 && grant.MaxPlaintextBytes <= MaxPlaintext &&
		grant.SigningSchema == signing.SchemaLocalCanonicalV1 && lowerHex(grant.Signature, 64) &&
		signing.VerifyCanonical(key.Public(), grantMap(grant), grant.Signature)
}

func (a *Authority) readGrant(id string) (Grant, error) {
	var grant Grant
	path, err := a.grantPath(id)
	if err != nil {
		return grant, err
	}
	if err := readAuthorityRecord(path, &grant); err != nil {
		return grant, err
	}
	if grant.GrantID != id || !validGrant(grant, a.key) || time.Duration(grant.RetentionSeconds)*time.Second > a.store.limits.Retention {
		return Grant{}, ErrState
	}
	return grant, nil
}

func validRevocation(revocation Revocation, grant Grant, key *signing.Key) bool {
	revoked, err := time.Parse(time.RFC3339Nano, revocation.RevokedAt)
	issued, issuedErr := time.Parse(time.RFC3339Nano, grant.IssuedAt)
	return err == nil && issuedErr == nil && !revoked.Before(issued) && key != nil &&
		revocation.SchemaVersion == "local-raw-task-content-revocation/v1" && revocation.GrantID == grant.GrantID &&
		revocation.ExpectedGrantSignature == grant.Signature && revocation.ReasonCode == "raw_content_capture_revoked" &&
		digestRefPattern.MatchString(revocation.ActorRef) &&
		revocation.SigningSchema == signing.SchemaLocalCanonicalV1 && lowerHex(revocation.Signature, 64) &&
		signing.VerifyCanonical(key.Public(), revocationMap(revocation), revocation.Signature)
}

func (a *Authority) readRevocation(grant Grant) (Revocation, error) {
	var revocation Revocation
	path, err := a.revocationPath(grant.GrantID)
	if err != nil {
		return revocation, err
	}
	if err := readAuthorityRecord(path, &revocation); err != nil {
		return revocation, err
	}
	if !validRevocation(revocation, grant, a.key) {
		return Revocation{}, ErrState
	}
	return revocation, nil
}

func (a *Authority) inspectGrant(id string, now time.Time) (GrantView, error) {
	var view GrantView
	if now.IsZero() {
		return view, ErrInvalid
	}
	grant, err := a.readGrant(id)
	if err != nil {
		return view, err
	}
	view = GrantView{SchemaVersion: "local-raw-task-content-grant-view/v1", Grant: grant}
	if revocation, err := a.readRevocation(grant); err == nil {
		view.Status, view.Revocation = "revoked", &revocation
		return view, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return GrantView{}, err
	}
	issued, _ := time.Parse(time.RFC3339Nano, grant.IssuedAt)
	expires, _ := time.Parse(time.RFC3339Nano, grant.ExpiresAt)
	if now.Before(issued) {
		return GrantView{}, ErrDenied
	}
	if !now.Before(expires) {
		view.Status = "expired"
		return view, nil
	}
	view.Status = "active"
	return view, nil
}

func (a *Authority) activeGrant(id string, now time.Time) (Grant, error) {
	view, err := a.inspectGrant(id, now)
	if err != nil {
		return Grant{}, err
	}
	switch view.Status {
	case "active":
		return view.Grant, nil
	case "revoked":
		return view.Grant, ErrRevoked
	default:
		return view.Grant, ErrExpired
	}
}

// Get returns only currently active authority. Expired and revoked records
// remain immutable on disk and return their distinct fail-closed errors.
func (a *Authority) Get(id string, now time.Time) (Grant, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	return a.activeGrant(id, now)
}

// Inspect returns a verified management view without treating expired or
// revoked authority as active.
func (a *Authority) Inspect(id string, now time.Time) (GrantView, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	return a.inspectGrant(id, now)
}

// List validates the complete bounded authority directory before returning any
// management views. One corrupt Grant or Revocation fails the whole read.
func (a *Authority) List(now time.Time) ([]GrantView, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	if now.IsZero() || privateDir(a.dir) != nil {
		return nil, ErrInvalid
	}
	if _, err := a.authorityRecordCount(); err != nil {
		return nil, err
	}
	entries, err := statefs.ReadDir(a.dir)
	if err != nil {
		return nil, ErrState
	}
	ids := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".grant.json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".grant.json"))
		}
	}
	sort.Strings(ids)
	views := make([]GrantView, 0, len(ids))
	for _, id := range ids {
		view, err := a.inspectGrant(id, now)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// ResolveActive returns the only active Grant for one task and content kind.
// Native adapters do not receive raw task identifiers or management authority;
// their runtime-bound server handler uses this complete verified projection.
// Ambiguous overlapping authority fails closed until the user revokes one.
func (a *Authority) ResolveActive(taskID, kind string, now time.Time) (Grant, error) {
	ref, taskOK := taskRef(taskID)
	if !taskOK || !allowedKind(kind) || now.IsZero() {
		return Grant{}, ErrInvalid
	}
	views, err := a.List(now)
	if err != nil {
		return Grant{}, err
	}
	var matched *Grant
	for _, view := range views {
		if view.Status != "active" || view.Grant.TaskRef != ref || !grantAllowsKind(view.Grant, kind) {
			continue
		}
		if matched != nil {
			return Grant{}, ErrConflict
		}
		grant := view.Grant
		matched = &grant
	}
	if matched == nil {
		return Grant{}, ErrDenied
	}
	return *matched, nil
}

// Capture is the sole exported raw-content write path. It revalidates the
// immutable Grant and revocation tombstone while holding the authority lock.
func (a *Authority) Capture(grantID, taskID string, content Prepared, now time.Time) (Envelope, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	grant, err := a.activeGrant(grantID, now)
	if err != nil {
		return Envelope{}, err
	}
	ref, ok := taskRef(taskID)
	if !ok || ref != grant.TaskRef || len(content.raw) == 0 || len(content.raw) > grant.MaxPlaintextBytes {
		return Envelope{}, ErrDenied
	}
	var prepared payload
	if json.Unmarshal(content.raw, &prepared) != nil || !allowedKind(prepared.Kind) {
		return Envelope{}, ErrInvalid
	}
	kindAllowed := false
	for _, kind := range grant.Kinds {
		if kind == prepared.Kind {
			kindAllowed = true
			break
		}
	}
	if !kindAllowed {
		return Envelope{}, ErrDenied
	}
	return a.store.write(taskID, content, time.Duration(grant.RetentionSeconds)*time.Second, now)
}

// Revoke appends a signed terminal tombstone. expectedGrantSignature is a CAS
// precondition; a retry with the same value returns the first record.
func (a *Authority) Revoke(id, expectedGrantSignature, actorID string, now time.Time) (Revocation, error) {
	authorityMu.Lock()
	defer authorityMu.Unlock()
	var revocation Revocation
	actor, actorOK := hashRef(actorID)
	if now.IsZero() || !lowerHex(expectedGrantSignature, 64) || !actorOK {
		return revocation, ErrInvalid
	}
	grant, err := a.readGrant(id)
	if err != nil {
		return revocation, err
	}
	if expectedGrantSignature != grant.Signature {
		return revocation, ErrConflict
	}
	if existing, err := a.readRevocation(grant); err == nil {
		return existing, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return revocation, err
	}
	issued, _ := time.Parse(time.RFC3339Nano, grant.IssuedAt)
	if now.Before(issued) {
		return revocation, ErrDenied
	}
	count, err := a.authorityRecordCount()
	if err != nil || count >= maxAuthorityRecords {
		if err != nil {
			return revocation, err
		}
		return revocation, ErrBudget
	}
	revocation = Revocation{
		SchemaVersion:          "local-raw-task-content-revocation/v1",
		GrantID:                grant.GrantID,
		ExpectedGrantSignature: grant.Signature,
		ActorRef:               actor,
		RevokedAt:              now.UTC().Format(time.RFC3339Nano),
		ReasonCode:             "raw_content_capture_revoked",
		SigningSchema:          signing.SchemaLocalCanonicalV1,
	}
	revocation.Signature, err = a.key.SignCanonical(revocationMap(revocation))
	if err != nil {
		return Revocation{}, ErrState
	}
	raw, err := json.MarshalIndent(revocation, "", "  ")
	if err != nil {
		return Revocation{}, ErrState
	}
	raw = append(raw, '\n')
	path, _ := a.revocationPath(grant.GrantID)
	if err := publishAuthority(path, raw); errors.Is(err, ErrConflict) {
		existing, readErr := a.readRevocation(grant)
		if readErr == nil {
			return existing, nil
		}
		return Revocation{}, ErrState
	} else if err != nil {
		return Revocation{}, err
	}
	return revocation, nil
}
