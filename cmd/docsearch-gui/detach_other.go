//go:build !windows

package main

import (
	"os/exec"
)

const flagBreakawayFromJob = 0x01000000

func configureDetachedProcess(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}

func getDetachedCreationFlags(cmd *exec.Cmd) uint32 {
	return 0
}

func removeBreakawayFlag(cmd *exec.Cmd) bool {
	return false
}

const expectedDetachedFlags = 0
