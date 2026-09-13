package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
	"time"
)

func cmdLaunchAgentStop(args []string, out io.Writer) error {
	err := withLaunchAgentCommand(args, "--confirm-stop", func(st *state.Store, key *signing.Key, plist []byte, home string, uid int, control userSystemctl) error {
		return stopRegisteredLaunchAgent(st, key, plist, home, uid, control, 35*time.Second)
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "当前用户域本实例未报告运行进程，台账写锁已释放；配置与历史已保留。block 模式下后续受控操作将被拒绝。")
	return err
}

// Read the exit status from the same XML response whose ownership was verified.
func stoppedLaunchAgent(control userSystemctl, uid int, plist []byte, requireCleanExit bool) error {
	var raw string
	pid, err := readLoadedLaunchAgent(func(args ...string) (string, error) {
		result, err := control(args...)
		if len(args) == 3 && args[0] == "list" && args[1] == "-x" {
			raw = result
		}
		return result, err
	}, uid, plist)
	if err != nil {
		return err
	}
	if pid != 0 {
		return errors.New("launch-agent: process still running")
	}
	if requireCleanExit {
		actual, err := decodeLaunchPlist(raw)
		if err != nil || actual["LastExitStatus"] != int64(0) {
			return errors.New("launch-agent: normal exit not confirmed; task preserved")
		}
	}
	return nil
}

func stopRegisteredLaunchAgent(st *state.Store, key *signing.Key, plist []byte, home string, uid int, control userSystemctl, wait time.Duration) (resultErr error) {
	record, err := st.VerifyLaunchAgent(key, plist)
	if err != nil {
		return err
	}
	source, err := filepath.Abs(filepath.Join(st.Dir, record.Label+".plist"))
	if err != nil {
		return err
	}
	directory := filepath.Join(home, "Library", "LaunchAgents")
	verify := func() error {
		for _, path := range []string{home, filepath.Dir(directory), directory} {
			if err := ordinaryLaunchDirectory(path, false); err != nil {
				return err
			}
		}
		if _, err := st.VerifyLaunchAgent(key, plist); err != nil {
			return err
		}
		return verifyLaunchRegistration(filepath.Join(directory, record.Label+".plist"), source)
	}
	if err := verify(); err != nil {
		return err
	}
	loaded, pid, err := inspectLaunchAgent(control, uid, record.Label, plist)
	if err != nil {
		return err
	}
	requested := loaded && pid > 0
	if requested {
		if err := verify(); err != nil {
			return err
		}
		if _, err := control("stop", record.Label); err != nil {
			return errors.New("launch-agent: stop failed or timed out; inspect launch-agent-status, configuration preserved")
		}
	}
	deadline := time.Now().Add(wait)
	if loaded {
		for {
			if err := verify(); err != nil {
				return err
			}
			pid, err = readLoadedLaunchAgent(control, uid, plist)
			if err != nil {
				return err
			}
			if pid == 0 {
				break
			}
			if !time.Now().Before(deadline) {
				return errors.New("launch-agent: stop not confirmed before timeout")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	writer, err := state.AcquireWriter(st.Dir)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, writer.Release()) }()
	if err := verify(); err != nil {
		return err
	}
	if loaded {
		return stoppedLaunchAgent(control, uid, plist, requested)
	}
	// Recheck absence after acquiring the writer; an externally loaded job is
	// not stopped based on an older observation.
	loaded, _, err = inspectLaunchAgent(control, uid, record.Label, plist)
	if err != nil {
		return err
	}
	if loaded {
		return errors.New("launch-agent: task appeared during stop verification; retry status")
	}
	return verify()
}
