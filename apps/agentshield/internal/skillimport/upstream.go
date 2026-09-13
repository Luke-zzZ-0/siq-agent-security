package skillimport

import (
	"context"
	"os"
	"path/filepath"

	"siq-agent-security/apps/agentshield/internal/canon"
)

// Upstream checks re-fetch the recorded source and answer "has the skill
// moved since the signed import?" They are strictly read-only: no record is
// created or modified, no blob is published, and no authority is granted.
// The snapshot is compared against the signed record by the caller.
type UpstreamSnapshot struct {
	SourceKind          string   `json:"source_kind"`
	URL                 string   `json:"url"`
	Ref                 string   `json:"ref,omitempty"`
	SubDir              string   `json:"sub_dir,omitempty"`
	CommitSHA           string   `json:"commit_sha,omitempty"`
	ArchiveSHA256       string   `json:"archive_sha256,omitempty"`
	ArchiveBytes        int64    `json:"archive_bytes,omitempty"`
	Directories         []string `json:"directories"`
	Files               []File   `json:"files"`
	ExcludedGitMetadata bool     `json:"excluded_git_metadata"`
}

// ReadRecord validates and returns the signed import record without
// requiring the blob payload. Update checks and install-record binding use
// this so they keep answering even after a staged copy has been cleaned up.
func (s *Store) ReadRecord(ctx context.Context, id string) (*Record, error) {
	return s.readRecord(ctx, id)
}

// CheckUpstream re-fetches the source of an existing import. Git sources are
// re-fetched from the URL recorded in the signed record; the resolved commit
// is reported but NOT pinned, so a moved upstream is an answer, not an error.
// Zip sources do not retain their URL in the record, so the caller supplies
// one; it is bound to the original import by recomputing the source locator
// digest over the record's archive path and expected digest — a different
// URL is ErrChanged rather than a fetch of unbound content.
func (s *Store) CheckUpstream(ctx context.Context, id, remoteURL string) (*UpstreamSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	record, err := s.readRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.SchemaVersion != "local-skill-import/v2" {
		return nil, ErrInvalid
	}
	staging, err := os.MkdirTemp(s.dir, "upstream-*")
	if err != nil {
		return nil, ErrUnavailable
	}
	defer os.RemoveAll(staging)
	blob := filepath.Join(staging, "blob")
	payload := filepath.Join(staging, "payload")
	for _, dir := range []string{blob, payload} {
		if err = os.Mkdir(dir, 0700); err != nil {
			return nil, ErrUnavailable
		}
	}
	switch record.SourceKind {
	case "git":
		if record.Git == nil {
			return nil, ErrInvalid
		}
		// ExpectedCommit is left empty: the check observes the current
		// upstream instead of asserting the pinned copy.
		snapshot, excluded, metadata, err := s.gitTree(ctx, record.Git.URL, blob, payload, &GitCreateRequest{Ref: record.Git.Ref, SubDir: record.Git.SubDir})
		if err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		return &UpstreamSnapshot{SourceKind: record.SourceKind, URL: metadata.URL, Ref: metadata.Ref, SubDir: metadata.SubDir, CommitSHA: metadata.CommitSHA, Directories: snapshot.Directories, Files: snapshot.Files, ExcludedGitMetadata: excluded}, nil
	case "https_zip":
		if record.Remote == nil {
			return nil, ErrInvalid
		}
		parsed, err := downloadURL(remoteURL)
		if err != nil {
			return nil, err
		}
		canonical, err := canon.Marshal(map[string]any{"url": parsed.String(), "archive_path": record.Remote.ArchivePath, "expected_sha256": record.Remote.ExpectedSHA256})
		if err != nil {
			return nil, ErrInvalid
		}
		if sum(canonical) != record.SourceLocatorDigest {
			return nil, ErrChanged
		}
		// ExpectedSHA256 is left empty: the check observes the current
		// archive instead of asserting the pinned copy.
		snapshot, excluded, metadata, err := s.remoteTree(ctx, parsed.String(), blob, payload, &RemoteCreateRequest{ArchivePath: record.Remote.ArchivePath})
		if err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		return &UpstreamSnapshot{SourceKind: record.SourceKind, URL: parsed.String(), SubDir: metadata.ArchivePath, ArchiveSHA256: metadata.ArchiveSHA256, ArchiveBytes: metadata.ArchiveBytes, Directories: snapshot.Directories, Files: snapshot.Files, ExcludedGitMetadata: excluded}, nil
	default:
		// local_dir/local_zip imports have no fetchable upstream.
		return nil, ErrInvalid
	}
}
