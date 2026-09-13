package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestStopRequestClient(t *testing.T) {
	for _, scenario := range []string{"accepted", "lost response", "unrecorded", "bad challenge", "wrong response", "denied"} {
		t.Run(scenario, func(t *testing.T) {
			st, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			w, err := state.AcquireWriter(st.Dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Release()
			key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
			if err != nil {
				t.Fatal(err)
			}
			directory, err := st.DirectoryID()
			if err != nil {
				t.Fatal(err)
			}
			control, err := localcontrol.New(key, directory, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
				out.Header().Set("Content-Type", "application/json")
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("unexpected credential")
				}
				switch r.URL.Path {
				case "/healthz/instance":
					_ = json.NewEncoder(out).Encode(localHealth{SchemaVersion: "local-service-instance-health/v1", Product: "siq-agent-security", Version: "test", LocalMode: true, Status: "ready", StateDirectoryID: directory})
				case "/v1/service-control/challenge":
					challenge, _ := control.Challenge()
					if scenario == "bad challenge" {
						challenge.Signature = strings.Repeat("0", 128)
					}
					_ = json.NewEncoder(out).Encode(challenge)
				case "/v1/service-control/stop":
					posts++
					if r.Header.Get("X-SIQ-Local-CLI") != "1" {
						t.Error("missing CLI header")
					}
					var request localcontrol.Message
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					if scenario == "denied" {
						out.WriteHeader(403)
						return
					}
					var accepted state.ServiceStopAcceptance
					if scenario != "unrecorded" {
						err := control.Accept(request, func() error {
							var err error
							accepted, err = st.RecordServiceStopAcceptance(w, key, request, time.Now())
							return err
						})
						if err != nil {
							t.Error(err)
							return
						}
					}
					if scenario == "lost response" {
						conn, _, err := out.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					if scenario == "wrong response" {
						accepted.BootID = strings.Repeat("f", 64)
					}
					out.WriteHeader(202)
					_ = json.NewEncoder(out).Encode(accepted)
				default:
					out.WriteHeader(404)
				}
			}))
			defer srv.Close()
			client := localClient()
			defer client.CloseIdleConnections()
			accepted, err := requestSignedLocalStop(client, srv.URL, st, key)
			if scenario == "accepted" || scenario == "lost response" {
				if err != nil || accepted.Action != "stop_accepted" {
					t.Fatal("acceptance lost", err)
				}
			} else if err == nil {
				t.Fatal("unconfirmed acceptance returned")
			}
			if scenario == "bad challenge" {
				if posts != 0 {
					t.Fatal("posted after failed challenge")
				}
			} else if posts != 1 {
				t.Fatal("unexpected retries", posts)
			}
		})
	}
}

func TestStopRequestConfirmationAndResponseBounds(t *testing.T) {
	for _, args := range [][]string{nil, {"--confirm-stop=false"}, {"--confirm-stop", "extra"}} {
		if cmdStopRequest(args, &bytes.Buffer{}) == nil {
			t.Fatal("confirmation omitted")
		}
	}
	for _, body := range []string{`{"unknown":1}`, `{} {}`, strings.Repeat(" ", 4097)} {
		resp := httptest.NewRecorder()
		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(202)
		_, _ = resp.WriteString(body)
		if decodeServiceControlResponse(resp.Result(), 202, &state.ServiceStopAcceptance{}) == nil {
			t.Fatal("invalid response accepted")
		}
	}
}
