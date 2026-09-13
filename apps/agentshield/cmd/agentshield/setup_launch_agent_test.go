package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSetupLaunchAgentStagesAndReuse(t *testing.T) {
	testUserSetupStages(t, setupLaunchAgent)
}

func TestSetupWindowsTaskStagesAndReuse(t *testing.T) {
	testUserSetupStages(t, setupWindowsTask)
}

func testUserSetupStages(t *testing.T, setup func(int, bool, bool, io.Writer, userSetupActions) error) {
	for _, mode := range []string{"fresh", "reuse", "open", "domain failure", "config failure", "init failure", "register failure", "start failure", "health failure", "port mismatch", "browser failure"} {
		t.Run(mode, func(t *testing.T) {
			var calls []string
			healthCalls := 0
			failure := errors.New("injected failure")
			action := func(name string) error {
				calls = append(calls, name)
				if mode == name+" failure" {
					return failure
				}
				return nil
			}
			actions := userSetupActions{
				preflight: func() error { return action("domain") },
				port:      func() (int, error) { err := action("config"); return 12345, err },
				health: func() error {
					healthCalls++
					calls = append(calls, "health")
					if mode == "health failure" || (healthCalls == 1 && mode != "reuse" && mode != "port mismatch") {
						return failure
					}
					return nil
				},
				initialize: func(args []string) error {
					if strings.Join(args, " ") != "--port 12345" {
						t.Fatal("wrong initialization arguments", args)
					}
					return action("init")
				},
				register: func() error { return action("register") },
				start:    func() error { return action("start") },
				open:     func(io.Writer) error { return action("browser") },
			}
			port := 12345
			if mode == "port mismatch" {
				port++
			}
			var out bytes.Buffer
			err := setup(port, true, mode == "open" || mode == "browser failure", &out, actions)
			valid := mode == "fresh" || mode == "reuse" || mode == "open"
			if (err == nil) != valid {
				t.Fatal("unexpected result", err)
			}
			sequence := strings.Join(calls, ",")
			want := map[string]string{
				"fresh":            "domain,config,health,init,register,start,config,health",
				"reuse":            "domain,config,health,start,config,health",
				"open":             "domain,config,health,init,register,start,config,health,browser",
				"domain failure":   "domain",
				"config failure":   "domain,config",
				"init failure":     "domain,config,health,init",
				"register failure": "domain,config,health,init,register",
				"start failure":    "domain,config,health,init,register,start",
				"health failure":   "domain,config,health,init,register,start,config,health",
				"port mismatch":    "domain,config,health",
				"browser failure":  "domain,config,health,init,register,start,config,health,browser",
			}[mode]
			if sequence != want {
				t.Fatal("unsafe stage ordering", sequence, want)
			}
			ready := valid || mode == "browser failure"
			if strings.Contains(out.String(), "已就绪") != ready {
				t.Fatal("incorrect readiness claim", out.String())
			}
			if ready && !strings.Contains(out.String(), "127.0.0.1:12345/") {
				t.Fatal("wrong URL", out.String())
			}
		})
	}
}
