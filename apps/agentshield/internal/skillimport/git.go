package skillimport

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"siq-agent-security/apps/agentshield/internal/statefs"
	"strings"
	"time"

	"siq-agent-security/apps/agentshield/internal/canon"
)

// The hosted Git component resolves a fixed public commit (ADR-0051).
// Production remains closed by productionGitFetch pending live HTTPS acceptance.
// Like every skill-import source this fixes an installation candidate; it
// never grants runtime authority and never installs into a platform directory.
//
// There is no git protocol and no process execution in this path: a controlled
// HTTPS flow resolves the ref to an immutable commit and downloads the archive
// for that exact commit. Refs are a strict allowlist (no options, no "..", no
// leading dashes) and the resolved commit is pinned into the signed record.

var gitRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)
var commitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

type GitCreateRequest struct {
	SchemaVersion  string `json:"schema_version"`
	ImportID       string `json:"import_id"`
	URL            string `json:"url"`
	Ref            string `json:"ref"`
	SubDir         string `json:"sub_dir"`
	ExpectedCommit string `json:"expected_commit"`
	ActorID        string `json:"actor_id"`
}
type GitMetadata struct {
	URL            string `json:"url"`
	Ref            string `json:"ref"`
	SubDir         string `json:"sub_dir"`
	ExpectedCommit string `json:"expected_commit"`
	CommitSHA      string `json:"commit_sha"`
}

func gitRefValid(ref string) bool {
	if !gitRefPattern.MatchString(ref) || commitPattern.MatchString(ref) {
		return false
	}
	if strings.Contains(ref, "..") || strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for _, part := range strings.Split(ref, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}
func gitValid(m *GitMetadata) bool {
	if m == nil || !commitPattern.MatchString(m.CommitSHA) || !archivePathValid(m.SubDir) {
		return false
	}
	if m.Ref != "" && !gitRefValid(m.Ref) {
		return false
	}
	parsed, err := downloadURL(m.URL)
	if err != nil || parsed.String() != m.URL {
		return false
	}
	return m.ExpectedCommit == "" || m.ExpectedCommit == m.CommitSHA
}
func (s *Store) CreateGit(ctx context.Context, req GitCreateRequest) (*Record, *Analysis, bool, error) {
	if req.SchemaVersion != "local-skill-import-git-create/v1" || !importID.MatchString(req.ImportID) || !actorValid(req.ActorID) || !archivePathValid(req.SubDir) || (req.Ref != "" && !gitRefValid(req.Ref)) || (req.ExpectedCommit != "" && !commitPattern.MatchString(req.ExpectedCommit)) {
		return nil, nil, false, ErrInvalid
	}
	parsed, err := downloadURL(req.URL)
	if err != nil {
		return nil, nil, false, err
	}
	canonical, err := canon.Marshal(map[string]any{"url": parsed.String(), "ref": req.Ref, "sub_dir": req.SubDir, "expected_commit": req.ExpectedCommit})
	if err != nil {
		return nil, nil, false, ErrInvalid
	}
	local := CreateRequest{ImportID: req.ImportID, SourceKind: "git", ActorID: req.ActorID}
	return s.create(ctx, local, parsed.String(), sum(canonical), createExtra{git: &req})
}

// gitTree clones into a private staging worktree and snapshots it into the
// payload directory. .git metadata is excluded by the same directoryTree rule
// as local_dir; the snapshot is re-verified against admission afterwards.
func (s *Store) gitTree(ctx context.Context, source, blob, payload string, req *GitCreateRequest) (tree, bool, *GitMetadata, error) {
	none := emptyTree()
	fetch := s.gitFetch
	if fetch == nil {
		fetch = productionGitFetch
	}
	worktree := filepath.Join(blob, "unpacked")
	if err := statefs.Mkdir(worktree, 0700); err != nil {
		return none, false, nil, ErrUnavailable
	}
	defer statefs.RemoveAll(worktree)
	commit, err := fetch(ctx, source, req.Ref, worktree)
	if err != nil {
		return none, false, nil, err
	}
	if !commitPattern.MatchString(commit) {
		return none, false, nil, ErrDownloadFailed
	}
	if req.ExpectedCommit != "" && req.ExpectedCommit != commit {
		return none, false, nil, ErrArchiveMismatch
	}
	root := worktree
	if req.SubDir != "" {
		root = filepath.Join(worktree, filepath.FromSlash(req.SubDir))
	}
	snapshot, excluded, err := directoryTree(ctx, root, payload, true)
	if err != nil {
		return none, false, nil, err
	}
	return snapshot, excluded, &GitMetadata{URL: source, Ref: req.Ref, SubDir: req.SubDir, ExpectedCommit: req.ExpectedCommit, CommitSHA: commit}, nil
}

// cloneGit runs the hardened clone; fileAllow exists only for tests driving a
// local file:// fixture through the same argv and environment. Production git
// imports never reach it: the store's gitFetch seam is nil in production and
// defaults to the closed productionGitFetch gate (ADR-0051).
func cloneGit(ctx context.Context, cleanURL, ref, dst, fileAllow string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	bin, err := exec.LookPath("git")
	if err != nil {
		return "", ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	home, err := statefs.MkdirTemp("", "skill-import-git-*")
	if err != nil {
		return "", ErrUnavailable
	}
	defer statefs.RemoveAll(home)
	// An empty hooks directory replaces the repository's .git/hooks for every
	// hook invocation during clone and checkout.
	hooks := filepath.Join(home, "empty-hooks")
	if err = statefs.Mkdir(hooks, 0700); err != nil {
		return "", ErrUnavailable
	}
	config := filepath.Join(home, "git.config")
	if err = statefs.WriteFile(config, nil, 0600); err != nil {
		return "", ErrUnavailable
	}
	// The protocol allowlist mirrors fileAllow: production stays https-only,
	// including for redirect targets.
	allowed := "https"
	if fileAllow == "user" {
		allowed = "file"
	}
	env := []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + config,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=echo",
		"GIT_ALLOW_PROTOCOL=" + allowed,
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"LC_ALL=C",
		"TMPDIR=" + home,
	}
	argv := []string{bin,
		"-c", "core.hooksPath=" + hooks,
		"-c", "core.fsmonitor=false",
		"-c", "gc.auto=0",
		"-c", "protocol.file.allow=" + fileAllow,
		"clone", "--depth", "1", "--single-branch",
	}
	if ref != "" {
		argv = append(argv, "--branch", ref)
	}
	argv = append(argv, "--", cleanURL, dst)
	if _, err = runGit(ctx, env, "", argv...); err != nil {
		return "", err
	}
	rev := []string{bin, "-c", "core.hooksPath=" + hooks, "rev-parse", "--verify", "HEAD^{commit}"}
	stdout, err := runGit(ctx, env, dst, rev...)
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(stdout))
	if !commitPattern.MatchString(commit) {
		return "", ErrDownloadFailed
	}
	return commit, nil
}
func runGit(ctx context.Context, env []string, dir string, argv ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Dir = dir
	// Stderr is deliberately discarded: remote-controlled text must never
	// enter local logs or error strings. Stdout is trusted only after the
	// caller applies a strict pattern.
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrDownloadFailed
	}
	return stdout.Bytes(), nil
}
