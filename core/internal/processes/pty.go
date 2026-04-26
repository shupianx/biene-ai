package processes

// ptySession owns a child process attached to a pseudo-terminal. Two
// implementations exist: POSIX PTYs via creack/pty on Unix, ConPTY on
// Windows. Read/Write carries raw terminal bytes (including ANSI control
// sequences) — no translation happens at this layer.
type ptySession interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows uint16) error

	// Wait blocks until the child has exited and returns the exit code.
	// A return of -1 means the child was terminated (signal on Unix,
	// forced termination on Windows). err is non-nil only for system
	// errors during wait — process failure (non-zero exit) is reported
	// via exitCode, not err.
	Wait() (exitCode int, err error)

	// Kill terminates the child. On Unix this signals the whole process
	// group so descendants die too. On Windows the ConPTY teardown kills
	// the immediate child; deeply nested grandchildren may leak until we
	// add a Win32 Job Object.
	Kill() error

	// Close releases the PTY master and any associated handles. Safe to
	// call after Kill or after Wait. Idempotent.
	Close() error

	PID() int
}

type ptyOptions struct {
	Command string
	Args    []string
	Cwd     string
	Env     []string
	Cols    uint16
	Rows    uint16
}
