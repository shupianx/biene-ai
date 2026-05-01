//go:build !windows

package builtins

import "os/exec"

// applyPlatformSysProcAttr is a no-op outside Windows: Unix shells don't
// allocate a new visible console for child processes.
func applyPlatformSysProcAttr(_ *exec.Cmd) {}
