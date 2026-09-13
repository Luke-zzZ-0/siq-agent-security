package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestNativeServeSignedStop(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("SIQ_TEST_SERVE_STOP") != "1" {
		t.Skip("isolated Linux subprocess opt-in required")
	}
	binary := os.Getenv("SIQ_TEST_BINARY")
	if binary == "" {
		t.Fatal("binary required")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Initialize(w, port); err != nil {
		t.Fatal(err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "serve", "--state-dir", st.Dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if !finished {
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	client := localClient()
	defer client.CloseIdleConnections()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := probeLocalInstance(client, endpoint, st); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("service not ready")
		}
		time.Sleep(50 * time.Millisecond)
	}
	get, err := http.NewRequest("GET", endpoint+"/v1/service-control/challenge", nil)
	if err != nil {
		t.Fatal(err)
	}
	get.Header.Set("X-SIQ-Local-CLI", "1")
	resp, err := client.Do(get)
	if err != nil {
		t.Fatal(err)
	}
	var challenge localcontrol.Message
	if err := decodeLocalResponse(resp, &challenge); err != nil {
		t.Fatal(err)
	}
	key, err := signing.LoadExisting(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := st.DirectoryID()
	if err != nil {
		t.Fatal(err)
	}
	request, err := localcontrol.SignStop(challenge, key, directory, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	post, err := http.NewRequest("POST", endpoint+"/v1/service-control/stop", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	post.Header.Set("X-SIQ-Local-CLI", "1")
	post.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	var accepted state.ServiceStopAcceptance
	err = json.NewDecoder(resp.Body).Decode(&accepted)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != 202 || accepted.BootID != request.BootID {
		t.Fatal("stop not accepted", resp.StatusCode, err)
	}
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatal("serve did not exit cleanly", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop")
	}
	saved, err := st.ReadServiceStopAcceptance(key, request.BootID)
	if err != nil || saved != accepted {
		t.Fatal("acceptance not retained", err)
	}
	result, err := st.ReadServiceStopResult(key, request.BootID)
	if err != nil || result.Status != "drained" {
		t.Fatal("drain result missing", err)
	}
	w, err = state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal("writer not released", err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeStopRequestCLI(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("SIQ_TEST_SERVE_STOP") != "1" {
		t.Skip("isolated Linux subprocess opt-in required")
	}
	binary := os.Getenv("SIQ_TEST_BINARY")
	if binary == "" {
		t.Fatal("binary required")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Initialize(w, port); err != nil {
		t.Fatal(err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "serve", "--state-dir", st.Dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if !finished {
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	client := localClient()
	defer client.CloseIdleConnections()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := probeLocalInstance(client, endpoint, st); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("service not ready")
		}
		time.Sleep(50 * time.Millisecond)
	}
	key, err := signing.LoadExisting(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	stop := exec.Command(binary, "stop-request", "--confirm-stop")
	stop.Env = append(os.Environ(), "SIQ_AGENT_SECURITY_STATE_DIR="+st.Dir)
	raw, err := stop.Output()
	if err != nil {
		t.Fatal("stop-request CLI failed", err)
	}
	var accepted state.ServiceStopAcceptance
	if err := json.Unmarshal(raw, &accepted); err != nil || accepted.Action != "stop_accepted" {
		t.Fatal("invalid CLI acceptance", err)
	}
	request := localcontrol.Message{BootID: accepted.BootID}
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatal("serve did not exit cleanly", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop")
	}
	saved, err := st.ReadServiceStopAcceptance(key, request.BootID)
	if err != nil || saved != accepted {
		t.Fatal("acceptance not retained", err)
	}
	result, err := st.ReadServiceStopResult(key, request.BootID)
	if err != nil || result.Status != "drained" {
		t.Fatal("drain result missing", err)
	}
	w, err = state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal("writer not released", err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeStopCLI(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("SIQ_TEST_SERVE_STOP") != "1" {
		t.Skip("isolated Linux subprocess opt-in required")
	}
	binary := os.Getenv("SIQ_TEST_BINARY")
	if binary == "" {
		t.Fatal("binary required")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Initialize(w, port); err != nil {
		t.Fatal(err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "serve", "--state-dir", st.Dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if !finished {
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	client := localClient()
	defer client.CloseIdleConnections()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := probeLocalInstance(client, endpoint, st); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("service not ready")
		}
		time.Sleep(50 * time.Millisecond)
	}
	key, err := signing.LoadExisting(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	stop := exec.Command(binary, "stop", "--confirm-stop")
	stop.Env = append(os.Environ(), "SIQ_AGENT_SECURITY_STATE_DIR="+st.Dir)
	raw, err := stop.Output()
	if err != nil {
		t.Fatal("stop-request CLI failed", err)
	}
	var completed state.ServiceStopResult
	if err := json.Unmarshal(raw, &completed); err != nil || completed.Status != "drained" {
		t.Fatal("invalid CLI acceptance", err)
	}
	request := localcontrol.Message{BootID: completed.BootID}
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatal("serve did not exit cleanly", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop")
	}
	_, err = st.ReadServiceStopAcceptance(key, request.BootID)
	if err != nil {
		t.Fatal("acceptance not retained", err)
	}
	result, err := st.ReadServiceStopResult(key, request.BootID)
	if err != nil || result != completed {
		t.Fatal("drain result missing", err)
	}
	w, err = state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal("writer not released", err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
	recoverCmd := exec.Command(binary, "stop", "--confirm-stop", "--recover", request.BootID)
	recoverCmd.Env = stop.Env
	recovered, err := recoverCmd.Output()
	if err != nil {
		t.Fatal("recovery CLI failed", err)
	}
	var recoveredResult state.ServiceStopResult
	if err := json.Unmarshal(recovered, &recoveredResult); err != nil || recoveredResult != completed {
		t.Fatal("recovery changed result", err)
	}

}
