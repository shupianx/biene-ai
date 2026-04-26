//go:build windows

package processes

import (
	"context"
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
)

// windowsPTY wraps github.com/UserExistsError/conpty (a thin wrapper over
// the Win10 1809+ ConPTY API) and assigns the spawned process to a Win32
// Job Object configured with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE. Closing
// the job handle then kills the immediate child *and* every descendant —
// without it, ClosePseudoConsole only stops the direct child and tools
// like `npm run dev` (npm → node) leak the node process on stop.
type windowsPTY struct {
	cp     *conpty.ConPty
	job    windows.Handle // 0 if assignment failed; cleanup falls back to ConPTY-only
	killed atomic.Bool
	closed atomic.Bool
}

func startPTY(opts ptyOptions) (ptySession, error) {
	cols, rows := int(opts.Cols), int(opts.Rows)
	if cols == 0 {
		cols = int(defaultCols)
	}
	if rows == 0 {
		rows = int(defaultRows)
	}

	cmdLine, err := prepareWindowsCommandLine(opts.Command, opts.Args, opts.Cwd, opts.Env)
	if err != nil {
		return nil, err
	}
	cpOpts := []conpty.ConPtyOption{conpty.ConPtyDimensions(cols, rows)}
	if opts.Cwd != "" {
		cpOpts = append(cpOpts, conpty.ConPtyWorkDir(opts.Cwd))
	}
	if len(opts.Env) > 0 {
		cpOpts = append(cpOpts, conpty.ConPtyEnv(opts.Env))
	}

	cp, err := conpty.Start(cmdLine, cpOpts...)
	if err != nil {
		return nil, err
	}

	job, jobErr := attachKillOnCloseJob(cp.Pid())
	if jobErr != nil {
		// Don't abort the start: the process is already running and the
		// user expects it to come up. We degrade to "ConPTY-only kill"
		// (no grandchild cleanup), the same behavior we had before this
		// path existed. Surface the failure as a wrapped error so the
		// controller's caller sees it once but the process keeps running.
		_ = cp.Close()
		return nil, fmt.Errorf("attach job object: %w", jobErr)
	}

	return &windowsPTY{cp: cp, job: job}, nil
}

func (p *windowsPTY) Read(b []byte) (int, error)  { return p.cp.Read(b) }
func (p *windowsPTY) Write(b []byte) (int, error) { return p.cp.Write(b) }

func (p *windowsPTY) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	return p.cp.Resize(int(cols), int(rows))
}

func (p *windowsPTY) Wait() (int, error) {
	code, err := p.cp.Wait(context.Background())
	// Kill closes ConPTY handles, which races with Wait's polling
	// WaitForSingleObject. When we initiated the kill we want a clean
	// (-1, nil) return regardless of which side won.
	if p.killed.Load() {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	return int(code), nil
}

func (p *windowsPTY) Kill() error {
	p.killed.Store(true)
	return p.closeOnce()
}

func (p *windowsPTY) Close() error {
	return p.closeOnce()
}

func (p *windowsPTY) closeOnce() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	// Close the job first: with KILL_ON_JOB_CLOSE this terminates every
	// process still in the job (the ConPTY child *and* its descendants).
	// Then ConPTY teardown frees the pseudo-console + pipe handles —
	// which is also what unblocks the pump goroutine.
	var jobErr error
	if p.job != 0 {
		jobErr = windows.CloseHandle(p.job)
		p.job = 0
	}
	cpErr := p.cp.Close()
	if cpErr != nil {
		return cpErr
	}
	return jobErr
}

func (p *windowsPTY) PID() int {
	return p.cp.Pid()
}

// attachKillOnCloseJob creates a Job Object with KILL_ON_JOB_CLOSE and
// assigns the given pid to it. Returns the job handle; the caller owns
// it and must CloseHandle on teardown.
func attachKillOnCloseJob(pid int) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("CreateJobObject: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("SetInformationJobObject: %w", err)
	}

	// Open a temporary handle to the child process with the rights
	// AssignProcessToJobObject requires; the assignment persists in the
	// job after we close this handle.
	procHandle, err := windows.OpenProcess(
		windows.PROCESS_TERMINATE|windows.PROCESS_SET_QUOTA,
		false,
		uint32(pid),
	)
	if err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer windows.CloseHandle(procHandle)

	if err := windows.AssignProcessToJobObject(job, procHandle); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return job, nil
}
