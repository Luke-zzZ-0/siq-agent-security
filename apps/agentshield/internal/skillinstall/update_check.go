package skillinstall

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Update checks answer "has the installed skill's upstream moved?" for an
// already installed import. The check is read-only: it creates no record, no
// staging slot and no authority, and never pins a new upstream. Acting on a
// new version — including the permission comparison — stays with the explicit
// update-comparison flow, which requires a signed candidate grant.
type UpdateCheckRequest struct {
	SchemaVersion string `json:"schema_version"`
	RemoteURL     string `json:"remote_url"`
	ActorID       string `json:"actor_id"`
}

// Zip imports do not retain their source URL in the signed record, so the
// caller supplies it; git imports keep it and require this field to be empty.
const updateCheckPermissionDeferred = "deferred_to_update_comparison"

type UpdateCheckResult struct {
	SchemaVersion           string                `json:"schema_version"`
	InstallID               string                `json:"install_id"`
	CheckedAt               string                `json:"checked_at"`
	Status                  string                `json:"status"`
	SourceKind              string                `json:"source_kind"`
	UpstreamCommitSHA       string                `json:"upstream_commit_sha,omitempty"`
	UpstreamArchiveSHA256   string                `json:"upstream_archive_sha256,omitempty"`
	ContentChanges          []UpdateContentChange `json:"content_changes"`
	ContentChangesTotal     int                   `json:"content_changes_total"`
	ContentChangesTruncated bool                  `json:"content_changes_truncated"`
	RequiresConfirmation    bool                  `json:"requires_confirmation"`
	PermissionComparison    string                `json:"permission_comparison"`
}

func (s *Store) CheckUpdate(ctx context.Context, id string, req UpdateCheckRequest) (*UpdateCheckResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.SchemaVersion != "local-skill-update-check/v1" || len(req.RemoteURL) > 2048 || !actorIDValid(req.ActorID) {
		return nil, ErrInvalid
	}
	select {
	case stageSlot <- struct{}{}:
		defer func() { <-stageSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.checkUpdate(ctx, id, req)
}

func (s *Store) checkUpdate(ctx context.Context, id string, req UpdateCheckRequest) (*UpdateCheckResult, error) {
	record, installed, err := s.historicalRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	// Only a completed, not-yet-verified installation has a meaningful
	// upstream baseline; recovery and removed states are refused.
	if record.RecordedStatus != "installed_unverified" || record.Operation == nil {
		return nil, ErrChanged
	}
	if err := s.removalStarted(id); err != nil {
		return nil, err
	}
	imported, err := s.imports.ReadRecord(ctx, installed.Plan.Source.ImportID)
	if err != nil {
		return nil, sourceError(ctx, err)
	}
	if imported.ArtifactDigest != installed.Plan.Source.ArtifactDigest || imported.AnalysisSHA256 != installed.Plan.Source.AnalysisSHA256 {
		return nil, ErrChanged
	}
	switch imported.SourceKind {
	case "git":
		// The signed record already carries the URL; a caller-supplied one
		// must not be able to redirect the check.
		if req.RemoteURL != "" {
			return nil, ErrInvalid
		}
	case "https_zip":
		if req.RemoteURL == "" {
			return nil, ErrInvalid
		}
	default:
		// local_dir/local_zip imports have no fetchable upstream.
		return nil, ErrInvalid
	}
	snapshot, err := s.upstream(ctx, imported, req.RemoteURL)
	if err != nil {
		return nil, sourceError(ctx, err)
	}
	out := &UpdateCheckResult{SchemaVersion: "local-skill-update-check-result/v1", InstallID: id, Status: "up_to_date", SourceKind: snapshot.SourceKind, UpstreamCommitSHA: snapshot.CommitSHA, UpstreamArchiveSHA256: snapshot.ArchiveSHA256, ContentChanges: []UpdateContentChange{}, PermissionComparison: updateCheckPermissionDeferred}
	delta := compareContentDelta(updateTree(installed.Directories, installed.Files), updateTree(snapshot.Directories, snapshot.Files))
	out.ContentChanges, out.ContentChangesTotal, out.ContentChangesTruncated = delta.Changes, delta.Total, delta.Truncated
	if out.ContentChangesTotal > 0 {
		out.Status = "new_version"
		out.RequiresConfirmation = true
	}
	if err := s.boundary("update_checked"); err != nil {
		return nil, ErrUnavailable
	}
	// The record must not have been replaced while the upstream was fetched.
	current, err := s.imports.ReadRecord(ctx, imported.ImportID)
	if err != nil {
		return nil, sourceError(ctx, err)
	}
	if current.Signature != imported.Signature {
		return nil, ErrChanged
	}
	if err := s.removalStarted(id); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.CheckedAt = s.now().UTC().Format(time.RFC3339Nano)
	return out, nil
}

func actorIDValid(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 128 && strings.IndexFunc(value, unicode.IsControl) < 0
}
