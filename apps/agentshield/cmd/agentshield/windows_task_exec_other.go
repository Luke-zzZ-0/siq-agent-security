//go:build !windows

package main

import "errors"

func windowsTaskExecutable() (string, error) {
	return "", errors.New("task-query: Windows required")
}
