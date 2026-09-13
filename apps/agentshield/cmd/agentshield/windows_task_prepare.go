package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func cmdWindowsTaskPrepare(args []string, out io.Writer) error {
	return withPreparedWindowsTask(args, func(_ *state.Store, _ *signing.Key, _ []byte, record state.WindowsTaskRecord) error {
		return json.NewEncoder(out).Encode(record)
	})
}
func withPreparedWindowsTask(args []string, apply func(*state.Store, *signing.Key, []byte, state.WindowsTaskRecord) error) (resultErr error) {
	var taskXML bytes.Buffer
	if err := cmdTaskXML(args, &taskXML); err != nil {
		return err
	}
	var task struct {
		UserSID string `xml:"Principals>Principal>UserId"`
	}
	if err := xml.Unmarshal(taskXML.Bytes(), &task); err != nil {
		return err
	}
	dir, err := state.DefaultDir()
	if err != nil {
		return err
	}
	lifecycle, err := state.AcquireWriter(filepath.Join(dir, "service-control"))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lifecycle.Release()) }()
	writer, err := state.AcquireWriter(dir)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, writer.Release()) }()
	st, err := state.Open(dir)
	if err != nil {
		return err
	}
	key, err := signing.Load(dir)
	if err != nil {
		return err
	}
	record, err := st.PrepareWindowsTask(writer, key, taskXML.Bytes(), task.UserSID)
	if err != nil {
		return err
	}
	return apply(st, key, taskXML.Bytes(), record)
}
