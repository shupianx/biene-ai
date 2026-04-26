package builtins

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// ReadOnlyForInput reports whether this run_command invocation is a pure
// inspection call (e.g. `ls`, `git status`, `grep`) and is therefore safe
// to run without prompting the user for the execute permission.
//
// The check is intentionally conservative: only commands and flag patterns
// known to have no side effects return true. Anything ambiguous falls
// through to the normal permission flow.
func (t *RunCommandTool) ReadOnlyForInput(raw json.RawMessage) bool {
	in, err := parseRunCommandInput(raw)
	if err != nil {
		return false
	}
	return isReadOnlyCommand(in.Command, in.Args)
}

// readOnlyBinaries lists commands whose every documented mode is read-only,
// so we accept them regardless of arguments.
var readOnlyBinaries = map[string]struct{}{
	"ls": {}, "pwd": {}, "cat": {}, "head": {}, "tail": {},
	"file": {}, "stat": {}, "wc": {}, "du": {}, "df": {},
	"which": {}, "where": {}, "whoami": {}, "id": {}, "date": {},
	"echo": {}, "printf": {}, "tree": {}, "env": {}, "uname": {},
	"hostname": {}, "basename": {}, "dirname": {}, "realpath": {},
	"true": {}, "false": {},
	"grep": {}, "egrep": {}, "fgrep": {}, "rg": {}, "ag": {},
}

// gitReadOnlySubcommands are git subcommands that never modify the repo.
// Verbs like `branch` / `tag` / `config` / `remote` share a name with
// destructive variants and are intentionally excluded.
var gitReadOnlySubcommands = map[string]struct{}{
	"status": {}, "log": {}, "diff": {}, "show": {},
	"rev-parse": {}, "ls-files": {}, "ls-tree": {}, "ls-remote": {},
	"diff-tree": {}, "blame": {}, "describe": {}, "shortlog": {},
	"reflog": {}, "for-each-ref": {}, "cat-file": {}, "name-rev": {},
	"whatchanged": {}, "show-ref": {}, "show-branch": {},
	"check-ignore": {}, "check-attr": {}, "var": {}, "grep": {},
	"version": {},
}

// findUnsafeFlags turn find/fd into destructive commands.
var findUnsafeFlags = map[string]struct{}{
	"-delete": {}, "-exec": {}, "-execdir": {}, "-ok": {}, "-okdir": {},
	"--exec": {},
}

func isReadOnlyCommand(command string, args []string) bool {
	bin := strings.TrimSuffix(filepath.Base(command), ".exe")
	if _, ok := readOnlyBinaries[bin]; ok {
		return true
	}
	switch bin {
	case "find", "fd", "fdfind":
		for _, a := range args {
			if _, bad := findUnsafeFlags[a]; bad {
				return false
			}
		}
		return true
	case "git":
		sub := gitSubcommand(args)
		if sub == "" {
			return true
		}
		_, ok := gitReadOnlySubcommands[sub]
		return ok
	case "go":
		if len(args) == 0 {
			return true
		}
		switch args[0] {
		case "version", "list", "doc":
			return true
		case "env":
			for _, a := range args[1:] {
				if a == "-w" || a == "-u" {
					return false
				}
			}
			return true
		}
		return false
	case "node", "deno", "bun":
		return len(args) == 1 && (args[0] == "--version" || args[0] == "-v")
	case "python", "python3", "ruby", "perl":
		return len(args) == 1 && (args[0] == "--version" || args[0] == "-v" || args[0] == "-V")
	case "npm", "pnpm", "yarn":
		if len(args) == 0 {
			return true
		}
		switch args[0] {
		case "list", "ls", "info", "view", "outdated", "--version", "-v":
			return true
		case "config":
			if len(args) >= 2 && (args[1] == "get" || args[1] == "list" || args[1] == "ls") {
				return true
			}
			return false
		}
		return false
	}
	return false
}

// gitSubcommand finds the first non-flag argument after skipping git's
// global options (-c key=val, -C path, --git-dir=…, --work-tree=…). Returns
// "" if no subcommand is present (bare `git` or only flags).
func gitSubcommand(args []string) string {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-c" || a == "-C" || a == "--git-dir" || a == "--work-tree":
			i += 2
		case strings.HasPrefix(a, "--git-dir=") || strings.HasPrefix(a, "--work-tree="):
			i++
		case strings.HasPrefix(a, "-"):
			i++
		default:
			return a
		}
	}
	return ""
}
