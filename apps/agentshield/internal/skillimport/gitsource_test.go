package skillimport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"

	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

// TLS fixture serving both admitted hosts from one local server. Production
// still performs ordinary certificate and hostname verification; only the
// test dialer maps the validated public addresses here.
func hostedFetcher(t *testing.T, handler http.HandlerFunc) archiveFetcher {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"api.github.com", "codeload.github.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	return archiveFetcher{roots: roots, lookup: func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		if host != "api.github.com" && host != "codeload.github.com" {
			t.Errorf("unexpected DNS lookup: %s", host)
		}
		return []netip.Addr{netip.MustParseAddr("140.82.112.3")}, nil
	}, dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "140.82.112.3:443" {
			t.Errorf("unvalidated dial: %s %s", network, address)
			return nil, ErrURLBlocked
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
}

const hostedTestCommit = "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d"

func hostedZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func hostedHandler(t *testing.T, calls *atomic.Int32, sha string, archive []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case r.Host == "api.github.com" && r.URL.Path == "/repos/octocat/hello/commits/HEAD":
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("unexpected API accept: %q", r.Header.Get("Accept"))
			}
			fmt.Fprintf(w, `{"sha":%q,"commit":{"message":"x"}}`, sha)
		case r.Host == "api.github.com" && r.URL.Path == "/repos/octocat/hello/commits/v1.0":
			fmt.Fprintf(w, `{"sha":%q}`, sha)
		case r.Host == "codeload.github.com" && r.URL.Path == "/octocat/hello/zip/"+sha:
			if r.Header.Get("Accept") != "application/zip" {
				t.Errorf("unexpected archive accept: %q", r.Header.Get("Accept"))
			}
			_, _ = w.Write(archive)
		default:
			t.Errorf("unexpected request: %s%s", r.Host, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func TestHostedGitFetchEndToEnd(t *testing.T) {
	for _, ref := range []string{"", "v1.0"} {
		t.Run("ref="+ref, func(t *testing.T) {
			var calls atomic.Int32
			archive := hostedZip(t, map[string]string{"hello-" + hostedTestCommit[:7] + "/SKILL.md": "---\nname: hosted\ndescription: d.\n---\n"})
			f := hostedFetcher(t, hostedHandler(t, &calls, hostedTestCommit, archive))
			dst := filepath.Join(t.TempDir(), "unpacked")
			if err := os.Mkdir(dst, 0700); err != nil {
				t.Fatal(err)
			}
			commit, err := f.resolveCommit(context.Background(), hostedRepo{owner: "octocat", repo: "hello"}, ref)
			if err != nil || commit != hostedTestCommit {
				t.Fatal(commit, err)
			}
			got, err := f.fetchWith(context.Background(), "https://codeload.github.com/octocat/hello/zip/"+commit, fetchOptions{accept: "application/zip", maxRedirects: 0, maxBytes: maxArchiveBytes, status: hostedStatusError})
			if err != nil {
				t.Fatal(err)
			}
			if err := materializeHostedTree(context.Background(), got.raw, dst); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
			if err != nil || !bytes.Contains(raw, []byte("name: hosted")) {
				t.Fatal(string(raw), err)
			}
			if calls.Load() != 2 {
				t.Fatal("unexpected request count", calls.Load())
			}
		})
	}
}

func TestHostedResolveFailuresAreSourceErrors(t *testing.T) {
	for _, mode := range []string{"status_404", "status_500", "malformed", "duplicate_keys", "wrong_sha", "empty_sha", "oversize", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			f := hostedFetcher(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "status_404":
					w.WriteHeader(404)
				case "status_500":
					w.WriteHeader(500)
				case "malformed":
					_, _ = w.Write([]byte("{not json"))
				case "duplicate_keys":
					_, _ = w.Write([]byte(`{"sha":"` + hostedTestCommit + `","sha":"` + hostedTestCommit + `"}`))
				case "wrong_sha":
					_, _ = w.Write([]byte(`{"sha":"ABCDEF1234"}`))
				case "empty_sha":
					_, _ = w.Write([]byte(`{}`))
				case "oversize":
					_, _ = w.Write(make([]byte, 1<<20+1))
				case "redirect":
					http.Redirect(w, r, "https://api.github.com/elsewhere", 302)
				}
			})
			_, err := f.resolveCommit(context.Background(), hostedRepo{owner: "octocat", repo: "hello"}, "")
			want := ErrSourceUnavailable
			if mode == "oversize" {
				want = ErrLimit
			}
			if mode == "redirect" {
				want = ErrURLBlocked
			}
			if !errors.Is(err, want) {
				t.Fatal(mode, err)
			}
			if mode == "redirect" && calls.Load() != 1 {
				t.Fatal("redirect followed for controlled request", calls.Load())
			}
		})
	}
}

func TestHostedArchiveFailures(t *testing.T) {
	for _, mode := range []string{"status_404", "two_top_dirs", "top_file", "oversize_declared"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			archive := []byte("not used")
			switch mode {
			case "two_top_dirs":
				archive = hostedZip(t, map[string]string{"one/a": "a", "two/b": "b"})
			case "top_file":
				archive = hostedZip(t, map[string]string{"one/a": "a", "root.txt": "x"})
			}
			f := hostedFetcher(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if mode == "status_404" {
					w.WriteHeader(404)
					return
				}
				if mode == "oversize_declared" {
					w.Header().Set("Content-Length", fmt.Sprint(maxArchiveBytes+1))
					w.WriteHeader(200)
					return
				}
				_, _ = w.Write(archive)
			})
			dst := filepath.Join(t.TempDir(), "unpacked")
			commit := hostedTestCommit
			got, err := f.fetchWith(context.Background(), "https://codeload.github.com/octocat/hello/zip/"+commit, fetchOptions{accept: "application/zip", maxRedirects: 0, maxBytes: maxArchiveBytes, status: hostedStatusError})
			want := ErrSourceUnavailable
			if mode == "oversize_declared" {
				want = ErrLimit
			}
			if mode != "status_404" && mode != "oversize_declared" {
				if err != nil {
					t.Fatal(err)
				}
				err = materializeHostedTree(context.Background(), got.raw, dst)
				want = ErrInvalid
			}
			if !errors.Is(err, want) {
				t.Fatal(mode, err)
			}
			if _, statErr := os.Lstat(dst); !os.IsNotExist(statErr) {
				t.Fatal("failed materialization created target", statErr)
			}
		})
	}
}

func TestHostedRepoParseTable(t *testing.T) {
	for _, raw := range []string{"https://github.com/octocat/hello", "https://github.com/octocat/hello.git", "https://github.com/octocat/hello/", "https://github.com/Octo-Cat/hello.world"} {
		repo, err := parseHostedRepo(raw)
		if err != nil || repo.owner == "" || repo.repo == "" {
			t.Errorf("rejected valid repo %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"https://github.com/octocat", "https://github.com/octocat/hello/extra", "https://github.com/octocat//hello", "https://github.com/-o/hello", "https://github.com/o-/hello", "https://github.com/o/../hello", "https://github.com/o/.", "https://github.com/o/..", "https://github.com/o/.git", "https://github.com//hello", "https://gitlab.com/o/hello", "https://github.com:8443/o/hello", "https://user:pw@github.com/o/hello", "https://github.com/o/hello?x=1"} {
		if _, err := parseHostedRepo(raw); err == nil {
			t.Errorf("accepted invalid repo %q", raw)
		}
	}
	// Unsupported hosts are a distinct, stable refusal wrapped on ErrURLBlocked.
	if _, err := parseHostedRepo("https://gitlab.com/o/hello"); !errors.Is(err, ErrGitHostUnsupported) || !errors.Is(err, ErrURLBlocked) {
		t.Fatal("unsupported host classification", err)
	}
}
