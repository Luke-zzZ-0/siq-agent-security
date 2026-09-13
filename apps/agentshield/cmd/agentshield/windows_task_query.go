package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func cmdWindowsTaskQuery(args []string, out io.Writer) error {
	return cmdWindowsTaskRead(args, out, false)
}

func cmdWindowsTaskRead(args []string, out io.Writer, presence bool) error {
	return withWindowsTaskIdentity(args, func(st *state.Store, key *signing.Key, expected []byte, sid string) error {
		if presence {
			return inspectOwnedWindowsTask(st, key, expected, sid, runWindowsTaskPresence, runWindowsTaskQuery, out)
		}
		return queryOwnedWindowsTask(st, key, expected, sid, runWindowsTaskQuery, out)
	})
}

func withWindowsTaskIdentity(args []string, apply func(*state.Store, *signing.Key, []byte, string) error) error {
	var expected bytes.Buffer
	if err := cmdTaskXML(args, &expected); err != nil {
		return err
	}
	var identity struct {
		SID string `xml:"Principals>Principal>UserId"`
	}
	if err := xml.Unmarshal(expected.Bytes(), &identity); err != nil {
		return err
	}
	dir, err := state.DefaultDir()
	if err != nil {
		return err
	}
	st := &state.Store{Dir: dir}
	key, err := signing.LoadExisting(dir)
	if err != nil {
		return err
	}
	return apply(st, key, expected.Bytes(), identity.SID)
}

func queryOwnedWindowsTask(st *state.Store, key *signing.Key, expected []byte, sid string, query func(string) ([]byte, error), out io.Writer) error {
	record, err := st.VerifyWindowsTask(key, expected, sid)
	if err != nil {
		return err
	}
	raw, err := query(record.TaskName)
	if err != nil {
		return errors.New("task-query: system query failed; task presence is unconfirmed")
	}
	decoded, err := decodeWindowsTaskOutput(raw)
	if err != nil {
		return err
	}
	if err := verifyWindowsTaskXML(decoded, expected); err != nil {
		return err
	}
	if _, err := st.VerifyWindowsTask(key, expected, sid); err != nil {
		return err
	}
	_, err = out.Write(expected)
	return err
}

func runWindowsTaskQuery(name string) ([]byte, error) {
	executable, err := windowsTaskExecutable()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "/Query", "/TN", name, "/XML")
	var stdout, stderr serviceOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil || stderr.Len() != 0 {
		return nil, errors.New("task-query: system command failed or timed out")
	}
	return []byte(stdout.String()), nil
}

var taskEncodingDeclaration = regexp.MustCompile(`(?i)encoding\s*=\s*["']([^"']+)["']`)

func decodeWindowsTaskOutput(raw []byte) ([]byte, error) {
	invalid := errors.New("task-query: unsupported or invalid output encoding")
	if len(raw) == 0 || len(raw) > 64*1024 {
		return nil, invalid
	}
	isUTF16 := bytes.HasPrefix(raw, []byte{0xff, 0xfe}) || bytes.HasPrefix(raw, []byte{0xfe, 0xff})
	if isUTF16 {
		var order binary.ByteOrder = binary.LittleEndian
		if raw[0] == 0xfe {
			order = binary.BigEndian
		}
		raw = raw[2:]
		if len(raw)%2 != 0 {
			return nil, invalid
		}
		units := make([]uint16, len(raw)/2)
		for i := range units {
			units[i] = order.Uint16(raw[i*2:])
		}
		for i := 0; i < len(units); i++ {
			u := units[i]
			if u >= 0xd800 && u <= 0xdbff {
				if i+1 == len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return nil, invalid
				}
				i++
			} else if u >= 0xdc00 && u <= 0xdfff {
				return nil, invalid
			}
		}
		raw = []byte(string(utf16.Decode(units)))
	} else {
		raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
		if !utf8.Valid(raw) {
			return nil, invalid
		}
	}
	if bytes.HasPrefix(raw, []byte("<?xml")) {
		end := bytes.Index(raw, []byte("?>"))
		if end < 0 {
			return nil, invalid
		}
		decl := raw[:end]
		matches := taskEncodingDeclaration.FindAllSubmatch(decl, -1)
		if len(matches) > 1 {
			return nil, invalid
		}
		if len(matches) == 1 {
			encoding := strings.ToLower(string(matches[0][1]))
			if (isUTF16 && encoding != "utf-16") || (!isUTF16 && encoding != "utf-8") {
				return nil, invalid
			}
			if isUTF16 {
				raw = append(taskEncodingDeclaration.ReplaceAll(decl, []byte(`encoding="UTF-8"`)), raw[end:]...)
			}
		}
	}
	if len(raw) > 64*1024 {
		return nil, invalid
	}
	return raw, nil
}
