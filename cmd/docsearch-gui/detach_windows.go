//go:build windows

package main

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

const flagBreakawayFromJob = 0x01000000

func configureDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | flagBreakawayFromJob,
	}
}

func getDetachedCreationFlags(cmd *exec.Cmd) uint32 {
	if cmd.SysProcAttr != nil {
		return cmd.SysProcAttr.CreationFlags
	}
	return 0
}

func removeBreakawayFlag(cmd *exec.Cmd) bool {
	if cmd.SysProcAttr != nil && (cmd.SysProcAttr.CreationFlags&flagBreakawayFromJob) != 0 {
		cmd.SysProcAttr.CreationFlags &^= flagBreakawayFromJob
		return true
	}
	return false
}

const expectedDetachedFlags = uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | flagBreakawayFromJob)
