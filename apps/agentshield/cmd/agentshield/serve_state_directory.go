package main

import (
	"errors"
	"os"
	"path/filepath"
)

func serveStateDirectory(selected string, explicit bool) (string, error) {
	if !explicit {
		return stateDir()
	}
	invalid := errors.New("serve: --state-dir requires an existing canonical absolute directory")
	if selected == "" || !filepath.IsAbs(selected) || filepath.Clean(selected) != selected {
		return "", invalid
	}
	info, err := os.Lstat(selected)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", invalid
	}
	canonical, err := filepath.EvalSymlinks(selected)
	if err != nil || canonical != selected {
		return "", invalid
	}
	return selected, nil
}
