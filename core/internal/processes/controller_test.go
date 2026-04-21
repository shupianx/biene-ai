package processes

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestControllerStartAndExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	root := t.TempDir()
	c := New(root)
	defer c.Close()

	res, err := c.Start(StartOptions{
		Command: "sh",
		Args:    []string{"-c", "echo hello; echo world"},
		Cwd:     root,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !res.State.Active || res.State.Status != StatusRunning {
		t.Fatalf("expected running state, got %+v", res.State)
	}
	if res.Replaced {
		t.Fatalf("unexpected Replaced=true on first start")
	}

	waitForStatus(t, c, StatusExited, 3*time.Second)

	out, err := c.ReadOutput(ReadOptions{})
	if err != nil {
		t.Fatalf("ReadOutput: %v", err)
	}
	if !strings.Contains(out.Content, "hello") || !strings.Contains(out.Content, "world") {
		t.Fatalf("expected output to contain hello+world, got %q", out.Content)
	}
	if out.State.ExitCode == nil || *out.State.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %v", out.State.ExitCode)
	}
}

func TestControllerAutoReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	root := t.TempDir()
	c := New(root)
	defer c.Close()

	_, err := c.Start(StartOptions{
		Command: "sh",
		Args:    []string{"-c", "sleep 30"},
		Cwd:     root,
	})
	if err != nil {
		t.Fatalf("Start 1: %v", err)
	}

	res, err := c.Start(StartOptions{
		Command: "sh",
		Args:    []string{"-c", "echo replaced"},
		Cwd:     root,
	})
	if err != nil {
		t.Fatalf("Start 2: %v", err)
	}
	if !res.Replaced {
		t.Fatalf("expected Replaced=true on second start")
	}

	waitForStatus(t, c, StatusExited, 3*time.Second)
	out, _ := c.ReadOutput(ReadOptions{})
	if !strings.Contains(out.Content, "replaced") {
		t.Fatalf("expected replaced output, got %q", out.Content)
	}
}

func TestControllerStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	root := t.TempDir()
	c := New(root)
	defer c.Close()

	_, err := c.Start(StartOptions{
		Command: "sh",
		Args:    []string{"-c", "sleep 30"},
		Cwd:     root,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	waitForStatus(t, c, StatusKilled, 3*time.Second)
}

func TestControllerSubscribeReceivesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	root := t.TempDir()
	c := New(root)
	defer c.Close()

	ch, unsub := c.Subscribe()
	defer unsub()

	_, err := c.Start(StartOptions{
		Command: "sh",
		Args:    []string{"-c", "echo one; echo two"},
		Cwd:     root,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	gotStarted, gotOne, gotTwo, gotStopped := false, false, false, false
	deadline := time.After(3 * time.Second)
	for !(gotStarted && gotOne && gotTwo && gotStopped) {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber closed early; got started=%v one=%v two=%v stopped=%v", gotStarted, gotOne, gotTwo, gotStopped)
			}
			switch ev.Kind {
			case "started":
				gotStarted = true
			case "output":
				if strings.TrimSpace(ev.Line) == "one" {
					gotOne = true
				}
				if strings.TrimSpace(ev.Line) == "two" {
					gotTwo = true
				}
			case "stopped":
				gotStopped = true
			}
		case <-deadline:
			t.Fatalf("timed out; started=%v one=%v two=%v stopped=%v", gotStarted, gotOne, gotTwo, gotStopped)
		}
	}
}

func TestControllerReadTail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	root := t.TempDir()
	c := New(root)
	defer c.Close()

	_, err := c.Start(StartOptions{
		Command: "sh",
		Args:    []string{"-c", "for i in 1 2 3 4 5; do echo line-$i; done"},
		Cwd:     root,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, c, StatusExited, 3*time.Second)

	out, err := c.ReadOutput(ReadOptions{TailLines: 2})
	if err != nil {
		t.Fatalf("ReadOutput: %v", err)
	}
	if !strings.Contains(out.Content, "line-4") || !strings.Contains(out.Content, "line-5") {
		t.Fatalf("tail should contain line-4 and line-5, got %q", out.Content)
	}
	if strings.Contains(out.Content, "line-1") {
		t.Fatalf("tail should not contain line-1, got %q", out.Content)
	}
}

func waitForStatus(t *testing.T, c *Controller, want Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.State().Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waiting for status %q; got %q", want, c.State().Status)
}
