package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func taskUTF16(value string, big bool) []byte {
	var order binary.ByteOrder = binary.LittleEndian
	b := []byte{0xff, 0xfe}
	if big {
		order, b = binary.BigEndian, []byte{0xfe, 0xff}
	}
	for _, u := range utf16.Encode([]rune(value)) {
		var encoded [2]byte
		order.PutUint16(encoded[:], u)
		b = append(b, encoded[:]...)
	}
	return b
}

func TestWindowsTaskOutputEncoding(t *testing.T) {
	source := `<?xml version="1.0" encoding="UTF-8"?><Task xmlns="` + windowsTaskNamespace + `"><Command>C:\中文\😀.exe</Command></Task>`
	for name, raw := range map[string][]byte{
		"UTF8":     []byte(source),
		"UTF8 BOM": append([]byte{0xef, 0xbb, 0xbf}, []byte(source)...),
		"UTF16LE":  taskUTF16(strings.Replace(source, "UTF-8", "UTF-16", 1), false),
		"UTF16BE":  taskUTF16(strings.Replace(source, "UTF-8", "UTF-16", 1), true),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := decodeWindowsTaskOutput(raw)
			if err != nil || verifyWindowsTaskXML(got, []byte(source)) != nil {
				t.Fatalf("valid output refused: %v", err)
			}
		})
	}
	for name, raw := range map[string][]byte{
		"odd bytes":                  {0xff, 0xfe, 0},
		"high surrogate":             {0xff, 0xfe, 0x00, 0xd8},
		"low surrogate":              {0xff, 0xfe, 0x00, 0xdc},
		"wrong pair":                 {0xff, 0xfe, 0x00, 0xd8, 0x41, 0},
		"UTF8 invalid":               {0xff, 0xff},
		"UTF16 declaration mismatch": taskUTF16(source, false),
		"UTF8 declaration mismatch":  []byte(strings.Replace(source, "UTF-8", "UTF-16", 1)),
		"empty":                      {},
		"oversized":                  bytes.Repeat([]byte{' '}, 65537),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeWindowsTaskOutput(raw); err == nil {
				t.Fatal("invalid encoding accepted")
			}
		})
	}
}

func TestWindowsOwnedTaskQuery(t *testing.T) {
	for _, scenario := range []string{"match", "UTF16", "failed", "drift", "source changed", "unowned"} {
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
			if _, err := st.Initialize(w, 0); err != nil {
				t.Fatal(err)
			}
			instance, err := st.ReadLocalInstance()
			if err != nil {
				t.Fatal(err)
			}
			key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
			if err != nil {
				t.Fatal(err)
			}
			const sid = "S-1-5-21-100-200-300-1001"
			expected, err := renderWindowsTask(`C:\SIQ\siq.exe`, `C:\SIQ\state`, instance.InstanceID, sid)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "unowned" {
				if _, err := st.PrepareWindowsTask(w, key, []byte(expected), sid); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			query := func(name string) ([]byte, error) {
				calls++
				if name != `\SIQ-Agent-Security-`+instance.InstanceID {
					t.Fatal("wrong task queried")
				}
				switch scenario {
				case "failed":
					return []byte(expected), errors.New("private system error")
				case "drift":
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				case "UTF16":
					return taskUTF16(strings.Replace(expected, "UTF-8", "UTF-16", 1), false), nil
				case "source changed":
					if err := os.WriteFile(filepath.Join(st.Dir, "SIQ-Agent-Security-"+instance.InstanceID+".xml"), []byte("changed"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return []byte(expected), nil
			}
			var out bytes.Buffer
			err = queryOwnedWindowsTask(st, key, []byte(expected), sid, query, &out)
			if scenario == "match" || scenario == "UTF16" {
				if err != nil || out.String() != expected {
					t.Fatalf("valid query: %v", err)
				}
			} else if err == nil || out.Len() != 0 {
				t.Fatal("unconfirmed query produced success output")
			}
			if scenario == "unowned" && calls != 0 {
				t.Fatal("queried unowned task")
			}
		})
	}
}
