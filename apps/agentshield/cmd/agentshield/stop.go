package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func cmdStop(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	confirm := fs.Bool("confirm-stop", false, "confirm graceful stop")
	recoverBoot := fs.String("recover", "", "inspect a previously accepted boot without sending another stop")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*confirm || fs.NArg() != 0 {
		return errors.New("stop: --confirm-stop required; unexpected arguments refused")
	}
	explicitRecovery := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "recover" {
			explicitRecovery = true
		}
	})
	if explicitRecovery && *recoverBoot == "" {
		return errors.New("stop: nonempty recovery boot required")
	}
	return withLocalStopClient(func(st *state.Store, key *signing.Key, client *http.Client, endpoint string) error {
		var accepted state.ServiceStopAcceptance
		var err error
		if explicitRecovery {
			accepted, err = st.ReadServiceStopAcceptance(key, *recoverBoot)
		} else {
			accepted, err = requestSignedLocalStop(client, endpoint, st, key)
		}
		if err != nil {
			return err
		}
		result, err := waitLocalStopResult(st, key, accepted, 35*time.Second)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(result)
	})
}

func waitLocalStopResult(st *state.Store, key *signing.Key, accepted state.ServiceStopAcceptance, wait time.Duration) (state.ServiceStopResult, error) {
	var empty state.ServiceStopResult
	saved, err := st.ReadServiceStopAcceptance(key, accepted.BootID)
	if err != nil || saved != accepted {
		return empty, errors.New("stop: acceptance is missing or changed")
	}
	deadline := time.Now().Add(wait)
	for {
		result, err := st.ReadServiceStopResult(key, accepted.BootID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return empty, err
		}
		if err == nil {
			if result.Status != "drained" {
				return empty, errors.New("stop: service reported drain failure; inspect state before retrying")
			}
			writer, lockErr := state.AcquireWriter(st.Dir)
			if lockErr == nil {
				current, acceptErr := st.ReadServiceStopAcceptance(key, accepted.BootID)
				final, readErr := st.ReadServiceStopResult(key, accepted.BootID)
				releaseErr := writer.Release()
				if acceptErr != nil || readErr != nil || current != accepted || final != result {
					return empty, errors.Join(errors.New("stop: records changed during final verification"), releaseErr)
				}
				if releaseErr != nil {
					return empty, releaseErr
				}
				return result, nil
			}
			if !errors.Is(lockErr, state.ErrWriterBusy) {
				return empty, lockErr
			}
		}
		if !time.Now().Before(deadline) {
			return empty, fmt.Errorf("stop: completion unconfirmed; retry with --confirm-stop --recover %s", accepted.BootID)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
