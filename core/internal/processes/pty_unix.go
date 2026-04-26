//go:build !windows

package processes

import (
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"

	"github.com/creack/pty"
)

// unixPTY wraps creack/pty. pty.StartWithSize sets Setsid=true and Setctty
// on the child, so the child is the leader of a fresh session whose
// controlling tty is the PTY slave. Killing -pid then delivers SIGKILL
// to every descendant — that's the only practical way to stop e.g.
// `npm run dev` cleanly, since npm spawns node and orphan node would
// otherwise survive an npm-only kill.
type unixPTY struct {
	cmd    *exec.Cmd
	master *os.File
	closed atomic.Bool
}

func startPTY(opts ptyOptions) (ptySession, error) {
	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.Cwd
	cmd.Env = opts.Env

	cols, rows := opts.Cols, opts.Rows
	if cols == 0 {
		cols = defaultCols
	}
	if rows == 0 {
		rows = defaultRows
	}

	master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	return &unixPTY{cmd: cmd, master: master}, nil
}

func (p *unixPTY) Read(b []byte) (int, error)  { return p.master.Read(b) }
func (p *unixPTY) Write(b []byte) (int, error) { return p.master.Write(b) }

func (p *unixPTY) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	return pty.Setsize(p.master, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *unixPTY) Wait() (int, error) {
	err := p.cmd.Wait()
	code := -1
	if p.cmd.ProcessState != nil {
		code = p.cmd.ProcessState.ExitCode()
	}
	if err == nil {
		return code, nil
	}
	// *exec.ExitError just means the child exited non-zero or was
	// signalled — that's a process outcome, not a wait-system failure,
	// so we surface it through the exit code rather than err.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return code, nil
	}
	return code, err
}

func (p *unixPTY) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(p.cmd.Process.Pid)
	if err != nil {
		// Group lookup failed (race with exit); fall back to single-process kill.
		return p.cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

func (p *unixPTY) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	return p.master.Close()
}

func (p *unixPTY) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
