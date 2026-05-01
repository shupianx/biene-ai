//go:build windows

package builtins

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// applyPlatformSysProcAttr suppresses the console window Windows would
// otherwise allocate for a console-subsystem child whose parent has no
// console of its own. biene-core is launched by Electron (a GUI app
// without a console), so without CREATE_NO_WINDOW every run_command
// invocation pops up a fresh cmd-style window before exiting.
func applyPlatformSysProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
