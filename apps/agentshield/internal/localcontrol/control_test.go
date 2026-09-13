package localcontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/signing"
)

func fixture(t *testing.T) (*Control, *signing.Key, *time.Time) {
	t.Helper()
	key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	c, err := New(key, strings.Repeat("b", 64), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return c, key, &now
}

func requestFor(t *testing.T, c *Control, key *signing.Key, now time.Time) Message {
	t.Helper()
	challenge, err := c.Challenge()
	if err != nil {
		t.Fatal(err)
	}
	r, err := SignStop(challenge, key, c.directory, now)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStopExpiryBindingsAndSignature(t *testing.T) {
	for _, scenario := range []string{"valid", "expired", "future", "other boot", "other directory", "tampered", "challenge replay"} {
		t.Run(scenario, func(t *testing.T) {
			c, key, now := fixture(t)
			r := requestFor(t, c, key, *now)
			switch scenario {
			case "expired":
				*now = now.Add(30 * time.Second)
			case "future":
				*now = now.Add(-time.Second)
			case "other boot":
				r.BootID = strings.Repeat("c", 64)
				r.Signature, _ = key.SignCanonical(r.unsigned())
			case "other directory":
				r.DirectoryID = strings.Repeat("c", 64)
				r.Signature, _ = key.SignCanonical(r.unsigned())
			case "tampered":
				r.Signature = strings.Repeat("0", 128)
			case "challenge replay":
				r, _ = c.Challenge()
			}
			calls := 0
			err := c.Accept(r, func() error { calls++; return nil })
			if scenario == "valid" {
				if err != nil || calls != 1 {
					t.Fatal(err, calls)
				}
				if c.Accept(r, func() error { calls++; return nil }) == nil || calls != 1 {
					t.Fatal("replay accepted")
				}
				if _, err := c.Challenge(); err == nil {
					t.Fatal("new challenge after acceptance")
				}
			} else if err == nil || calls != 0 {
				t.Fatal("unauthorized callback")
			}
		})
	}
}

func TestStopRecordFailureAndConcurrency(t *testing.T) {
	c, key, now := fixture(t)
	r := requestFor(t, c, key, *now)
	if c.Accept(r, nil) == nil {
		t.Fatal("nil record accepted")
	}
	if c.Accept(r, func() error { return errors.New("storage failed") }) == nil {
		t.Fatal("record failure ignored")
	}
	var calls, accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.Accept(r, func() error { calls.Add(1); return nil }) == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || accepted.Load() != 1 {
		t.Fatal("duplicate acceptance", calls.Load(), accepted.Load())
	}
}

func TestChallengeOriginAndRestart(t *testing.T) {
	c, key, now := fixture(t)
	challenge, err := c.Challenge()
	if err != nil {
		t.Fatal(err)
	}
	wrong, _ := signing.FromSeed(bytes.Repeat([]byte{8}, 32))
	if _, err := SignStop(challenge, wrong, c.directory, *now); err == nil {
		t.Fatal("wrong signing origin accepted")
	}
	if _, err := SignStop(challenge, key, strings.Repeat("c", 64), *now); err == nil {
		t.Fatal("wrong selected directory accepted")
	}
	if _, err := SignStop(challenge, key, c.directory, now.Add(30*time.Second)); err == nil {
		t.Fatal("expired challenge signed")
	}
	r := requestFor(t, c, key, *now)
	restarted, _, _ := fixture(t)
	if c.boot == restarted.boot {
		t.Fatal("boot nonce reused")
	}
	if restarted.Accept(r, func() error { t.Fatal("old boot authorized"); return nil }) == nil {
		t.Fatal("restart replay accepted")
	}
}

func TestControlContracts(t *testing.T) {
	c, key, now := fixture(t)
	c.boot = strings.Repeat("a", 64)
	challenge, err := c.Challenge()
	if err != nil {
		t.Fatal(err)
	}
	r, err := SignStop(challenge, key, c.directory, *now)
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]Message{"local-service-control-challenge": challenge, "local-service-stop-request": r} {
		raw, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		path := filepath.Join("../../testdata/contracts", name+".json")
		if os.Getenv("SIQ_UPDATE_CONTROL_FIXTURES") == "1" {
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		saved, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(saved, raw) {
			t.Fatal("contract fixture differs", name)
		}
	}
}
