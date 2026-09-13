package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func decodeServiceControlResponse(resp *http.Response, status int, out any) error {
	defer resp.Body.Close()
	media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if resp.StatusCode != status || err != nil || media != "application/json" {
		return errors.New("stop-request: unexpected response")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil || len(raw) > 4096 {
		return errors.New("stop-request: response exceeds limit or is unreadable")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("stop-request: invalid response")
	}
	return nil
}

func requestSignedLocalStop(client *http.Client, endpoint string, st *state.Store, key *signing.Key) (state.ServiceStopAcceptance, error) {
	var empty state.ServiceStopAcceptance
	if _, err := probeLocalInstance(client, endpoint, st); err != nil {
		return empty, err
	}
	directory, err := st.DirectoryID()
	if err != nil {
		return empty, err
	}
	get, err := http.NewRequest(http.MethodGet, endpoint+"/v1/service-control/challenge", nil)
	if err != nil {
		return empty, err
	}
	get.Header.Set("X-SIQ-Local-CLI", "1")
	resp, err := client.Do(get)
	if err != nil {
		return empty, errors.New("stop-request: challenge unavailable")
	}
	var challenge localcontrol.Message
	if err := decodeServiceControlResponse(resp, http.StatusOK, &challenge); err != nil {
		return empty, err
	}
	request, err := localcontrol.SignStop(challenge, key, directory, time.Now())
	if err != nil {
		return empty, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return empty, err
	}
	post, err := http.NewRequest(http.MethodPost, endpoint+"/v1/service-control/stop", bytes.NewReader(raw))
	if err != nil {
		return empty, err
	}
	post.Header.Set("X-SIQ-Local-CLI", "1")
	post.Header.Set("Content-Type", "application/json")
	resp, transportErr := client.Do(post)
	var response state.ServiceStopAcceptance
	if transportErr == nil {
		if err := decodeServiceControlResponse(resp, http.StatusAccepted, &response); err != nil {
			return empty, err
		}
	}
	saved, err := st.ReadServiceStopAcceptance(key, request.BootID)
	if err != nil {
		return empty, errors.New("stop-request: acceptance unconfirmed; inspect local service before retrying")
	}
	digest, err := localcontrol.StopDigest(request, key, time.Unix(saved.AcceptedAt, 0))
	if err != nil || saved.RequestSHA256 != digest || (transportErr == nil && response != saved) {
		return empty, errors.New("stop-request: acceptance does not match this request")
	}
	return saved, nil
}

func cmdStopRequest(args []string, out io.Writer) error {
	if len(args) != 1 || args[0] != "--confirm-stop" {
		return errors.New("stop-request: --confirm-stop required, no other arguments accepted")
	}
	return withLocalStopClient(func(st *state.Store, key *signing.Key, client *http.Client, endpoint string) error {
		accepted, err := requestSignedLocalStop(client, endpoint, st, key)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(accepted)
	})
}

func withLocalStopClient(apply func(*state.Store, *signing.Key, *http.Client, string) error) (resultErr error) {
	dir, err := state.DefaultDir()
	if err != nil {
		return err
	}
	st := &state.Store{Dir: dir}
	key, err := signing.LoadExisting(dir)
	if err != nil {
		return err
	}
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	lock, err := state.AcquireScopedWriter(dir, "service-control")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lock.Release()) }()
	client := localClient()
	defer client.CloseIdleConnections()
	return apply(st, key, client, fmt.Sprintf("http://127.0.0.1:%d", cfg.Port))
}
