//go:build windows

package util

import (
	"os"
	"os/exec"
)

func Run(command string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "cmd"
	}
	return exec.Command(shell, "/C", command)
}
