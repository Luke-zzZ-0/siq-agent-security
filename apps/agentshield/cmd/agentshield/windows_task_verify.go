package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"strings"
)

const windowsTaskNamespace = "http://schemas.microsoft.com/windows/2004/02/mit/task"

type windowsTaskElement struct {
	Name     xml.Name
	Attrs    map[xml.Name]string
	Text     string
	Children map[xml.Name]*windowsTaskElement
}

// verifyWindowsTaskXML compares every field, including unknown additions.
// The transport must supply UTF-8, never truncated or guessed-decoded output.
func verifyWindowsTaskXML(actual, expected []byte) error {
	want, err := parseWindowsTaskXML(expected)
	if err != nil {
		return errors.New("task: invalid expected configuration")
	}
	got, err := parseWindowsTaskXML(actual)
	if err != nil {
		return errors.New("task: system configuration does not match owned configuration")
	}
	removeWindowsTaskReadbackDefaults(got, want)
	if !reflect.DeepEqual(got, want) {
		return errors.New("task: system configuration does not match owned configuration")
	}
	return nil
}

// Task Scheduler serializes these exact defaults even when the signed source
// omits them. Remove only an equivalent default absent from that source; keep
// non-default values, attributes, children and all unknown additions for the
// full comparison below. The saved source and its signature never change.
func removeWindowsTaskReadbackDefaults(got, want *windowsTaskElement) {
	name := func(local string) xml.Name { return xml.Name{Space: windowsTaskNamespace, Local: local} }
	remove := func(actual, expected *windowsTaskElement, local, value string) {
		if actual == nil || expected == nil || expected.Children[name(local)] != nil {
			return
		}
		n := actual.Children[name(local)]
		if n != nil && n.Text == value && len(n.Attrs) == 0 && len(n.Children) == 0 {
			delete(actual.Children, name(local))
		}
	}
	remove(got, want, "Triggers", "")
	remove(got.Children[name("Settings")], want.Children[name("Settings")], "DisallowStartOnRemoteAppSession", "false")
}

func parseWindowsTaskXML(raw []byte) (*windowsTaskElement, error) {
	invalid := errors.New("task: invalid or unsupported task XML")
	if len(raw) == 0 || len(raw) > 64*1024 {
		return nil, invalid
	}
	d := xml.NewDecoder(bytes.NewReader(raw))
	var root *windowsTaskElement
	var stack []*windowsTaskElement
	nodes, declarations := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			if root == nil || len(stack) != 0 {
				return nil, invalid
			}
			return root, nil
		}
		if err != nil {
			return nil, invalid
		}
		switch v := token.(type) {
		case xml.StartElement:
			nodes++
			if nodes > 256 || len(stack) >= 16 || v.Name.Space != windowsTaskNamespace {
				return nil, invalid
			}
			n := &windowsTaskElement{Name: v.Name, Attrs: map[xml.Name]string{}, Children: map[xml.Name]*windowsTaskElement{}}
			seen := map[xml.Name]bool{}
			for _, a := range v.Attr {
				if seen[a.Name] {
					return nil, invalid
				}
				seen[a.Name] = true
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				n.Attrs[a.Name] = a.Value
			}
			if len(stack) == 0 {
				if root != nil || v.Name.Local != "Task" {
					return nil, invalid
				}
				root = n
			} else {
				parent := stack[len(stack)-1]
				if parent.Children[v.Name] != nil {
					return nil, invalid
				}
				parent.Children[v.Name] = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, invalid
			}
			n := stack[len(stack)-1]
			if n.Name != v.Name {
				return nil, invalid
			}
			if len(n.Children) != 0 {
				if strings.TrimSpace(n.Text) != "" {
					return nil, invalid
				}
				n.Text = ""
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(v)) != "" {
					return nil, invalid
				}
			} else {
				stack[len(stack)-1].Text += string(v)
			}
		case xml.ProcInst:
			declarations++
			if v.Target != "xml" || root != nil || declarations != 1 {
				return nil, invalid
			}
		case xml.Comment:
			// Comments have no Task Scheduler configuration semantics.
		default:
			return nil, invalid
		}
	}
}
