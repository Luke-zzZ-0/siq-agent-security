// Package statefs preserves os APIs while checking state compatibility before
// file access. Migration and lock ownership have explicitly audited exceptions.
package statefs

import (
	"os"
	"siq-agent-security/apps/agentshield/internal/stateformat"
)

func WriteFile(p string, b []byte, m os.FileMode) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	return os.WriteFile(p, b, m)
}
func OpenFile(p string, f int, m os.FileMode) (*os.File, error) {
	if e := stateformat.RequirePath(p, f&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0); e != nil {
		return nil, e
	}
	return os.OpenFile(p, f, m)
}
func Create(p string) (*os.File, error) {
	if e := stateformat.RequirePath(p, true); e != nil {
		return nil, e
	}
	return os.Create(p)
}
func CreateTemp(p, s string) (*os.File, error) {
	if e := stateformat.RequirePath(p, true); e != nil {
		return nil, e
	}
	return os.CreateTemp(p, s)
}
func Mkdir(p string, m os.FileMode) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	return os.Mkdir(p, m)
}
func MkdirAll(p string, m os.FileMode) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	return os.MkdirAll(p, m)
}
func MkdirTemp(p, s string) (string, error) {
	if e := stateformat.RequirePath(p, true); e != nil {
		return "", e
	}
	return os.MkdirTemp(p, s)
}
func Remove(p string) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	return os.Remove(p)
}
func RemoveAll(p string) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	return os.RemoveAll(p)
}
func Rename(p, q string) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	if e := stateformat.RequirePath(q, true); e != nil {
		return e
	}
	return os.Rename(p, q)
}
func Link(p, q string) error {
	if e := stateformat.RequirePath(p, false); e != nil {
		return e
	}
	if e := stateformat.RequirePath(q, true); e != nil {
		return e
	}
	return os.Link(p, q)
}
func Symlink(p, q string) error {
	if e := stateformat.RequirePath(q, true); e != nil {
		return e
	}
	return os.Symlink(p, q)
}
func Chmod(p string, m os.FileMode) error {
	if e := stateformat.RequirePath(p, true); e != nil {
		return e
	}
	return os.Chmod(p, m)
}
func ReadFile(p string) ([]byte, error) {
	if e := stateformat.RequirePath(p, false); e != nil {
		return nil, e
	}
	return os.ReadFile(p)
}
func Open(p string) (*os.File, error) {
	if e := stateformat.RequirePath(p, false); e != nil {
		return nil, e
	}
	return os.Open(p)
}
func ReadDir(p string) ([]os.DirEntry, error) {
	if e := stateformat.RequirePath(p, false); e != nil {
		return nil, e
	}
	return os.ReadDir(p)
}
