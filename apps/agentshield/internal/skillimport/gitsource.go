package skillimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"siq-agent-security/apps/agentshield/internal/statefs"
)

// ADR-0051: production git imports fetch from explicitly admitted public
// hosted sources over controlled HTTPS. There is no git protocol and no git
// CLI in this path: metadata resolves a ref to an immutable commit, then the
// archive for that exact commit is downloaded, bounded, and materialized
// through the same checked pipeline as every other import source.
//
// The resolved commit is part of both request URLs, so branch drift between
// the two requests cannot change the content that lands in the worktree, and
// the commit pinned into the signed record is exactly what was downloaded.
// Controlled requests refuse every redirect: the admitted endpoint set is
// closed, and a redirect could otherwise move the fetch outside it.

var ErrSourceUnavailable = errors.New("skill_import_source_unavailable")

// Sources outside the admitted host list are refused with a stable error so
// the interface can point users at HTTPS ZIP or local imports. It is a URL
// policy failure: the destination was never contacted.
var ErrGitHostUnsupported = fmt.Errorf("%w: git_host_unsupported", ErrURLBlocked)

const hostedGitHost = "github.com"

var gitOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var gitRepoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

type hostedRepo struct {
	owner, repo string
}

// parseHostedRepo accepts https://github.com/{owner}/{repo} with one optional
// trailing slash and an optional .git suffix. Any other shape is refused
// before any DNS lookup or connection.
func parseHostedRepo(rawURL string) (hostedRepo, error) {
	parsed, err := downloadURL(rawURL)
	if err != nil {
		return hostedRepo{}, err
	}
	if parsed.Host != hostedGitHost {
		return hostedRepo{}, ErrGitHostUnsupported
	}
	if parsed.RawQuery != "" {
		return hostedRepo{}, ErrURLBlocked
	}
	segments := strings.Split(strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), "/"), "/")
	if len(segments) != 2 {
		return hostedRepo{}, ErrURLBlocked
	}
	owner, repo := segments[0], strings.TrimSuffix(segments[1], ".git")
	if !gitOwnerPattern.MatchString(owner) || strings.HasSuffix(owner, "-") || !gitRepoPattern.MatchString(repo) {
		return hostedRepo{}, ErrURLBlocked
	}
	return hostedRepo{owner: owner, repo: repo}, nil
}

func hostedStatusError(code int) error {
	return fmt.Errorf("%w: upstream_status_%d", ErrSourceUnavailable, code)
}

// resolveCommit maps a ref (or the host default branch when empty) to its
// 40-hex commit. Source-side failures — unknown ref, unknown repository,
// upstream 5xx, malformed or unparseable metadata — are
// ErrSourceUnavailable; the remote never supplies error text.
func (f archiveFetcher) resolveCommit(ctx context.Context, r hostedRepo, ref string) (string, error) {
	endpoint := "https://api.github.com/repos/" + r.owner + "/" + r.repo + "/commits/"
	if ref == "" {
		endpoint += "HEAD"
	} else {
		endpoint += ref
	}
	got, err := f.fetchWith(ctx, endpoint, fetchOptions{accept: "application/json", maxRedirects: 0, maxBytes: 1 << 20, status: hostedStatusError})
	if err != nil {
		return "", err
	}
	if !uniqueKeys(json.NewDecoder(bytes.NewReader(got.raw)), 0) {
		return "", fmt.Errorf("%w: commit_metadata", ErrSourceUnavailable)
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(got.raw, &payload); err != nil || !commitPattern.MatchString(payload.SHA) {
		return "", fmt.Errorf("%w: commit_metadata", ErrSourceUnavailable)
	}
	return payload.SHA, nil
}

// materializeHostedTree unpacks a codeload-style archive into dst. Exactly one
// top-level directory and no top-level files are accepted; the extraction and
// the bounded copy into dst reuse the shared zip and tree pipelines, so
// budgets, symlink and git-metadata rules are identical to zip imports.
func materializeHostedTree(ctx context.Context, raw []byte, dst string) error {
	tmp, err := statefs.MkdirTemp("", "skill-import-gitzip-*")
	if err != nil {
		return ErrUnavailable
	}
	defer statefs.RemoveAll(tmp)
	if _, _, err := extractZip(ctx, raw, tmp); err != nil {
		return err
	}
	entries, err := statefs.ReadDir(tmp)
	if err != nil {
		return ErrUnavailable
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return ErrInvalid
	}
	_, _, err = directoryTree(ctx, filepath.Join(tmp, entries[0].Name()), dst, true)
	return err
}

// fetchHostedGit is the production git source fetch: resolve, download by
// commit, materialize. It performs no process execution and reads no
// environment configuration that could redirect the transport.
func fetchHostedGit(ctx context.Context, rawURL, ref, dst string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if ref != "" && !gitRefValid(ref) {
		return "", ErrInvalid
	}
	repo, err := parseHostedRepo(rawURL)
	if err != nil {
		return "", err
	}
	f := httpsFetcher()
	commit, err := f.resolveCommit(ctx, repo, ref)
	if err != nil {
		return "", err
	}
	archiveURL := "https://codeload.github.com/" + repo.owner + "/" + repo.repo + "/zip/" + commit
	archive, err := f.fetchWith(ctx, archiveURL, fetchOptions{accept: "application/zip", maxRedirects: 0, maxBytes: maxArchiveBytes, status: hostedStatusError})
	if err != nil {
		return "", err
	}
	if err := materializeHostedTree(ctx, archive.raw, dst); err != nil {
		return "", err
	}
	return commit, nil
}

// Production stays closed until a fixed candidate passes real hosted-network
// acceptance. There is no environment override and no configurable test seam.
var ErrGitTransportUnavailable = fmt.Errorf("%w: git_transport_unavailable", ErrURLBlocked)

func productionGitFetch(ctx context.Context, rawURL, ref, dst string) (string, error) {
	if ctx != nil && ctx.Err() != nil {
		return "", ctx.Err()
	}
	if _, err := parseHostedRepo(rawURL); err != nil {
		return "", err
	}
	return "", ErrGitTransportUnavailable
}
