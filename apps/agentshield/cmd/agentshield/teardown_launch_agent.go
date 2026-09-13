package main

import (
	"errors"
	"os"
	"path/filepath"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
	"time"
)

// withLaunchAgentCommand retains the lifecycle lock across both stages.
func teardownLaunchAgent(st *state.Store, key *signing.Key, plist []byte, home string, uid int, control userSystemctl) error {
	record, err := st.VerifyLaunchAgent(key, plist)
	if err != nil {
		return err
	}
	if err := st.CheckServiceSwitchPending(); err != nil {
		return err
	}
	link := filepath.Join(home, "Library", "LaunchAgents", record.Label+".plist")
	if _, err := os.Lstat(link); err == nil {
		if err := stopRegisteredLaunchAgent(st, key, plist, home, uid, control, 35*time.Second); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Unregister alone verifies the interrupted state where both the system job
	// and registration link are already absent, without recreating either.
	return unregisterLaunchAgent(st, key, plist, home, uid, control)
}
