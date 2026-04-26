package builtins

import (
	"encoding/json"
	"testing"
)

func TestRunCommandReadOnlyForInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"ls bare", `{"command":"ls"}`, true},
		{"ls flags", `{"command":"ls","args":["-la"]}`, true},
		{"absolute path ls", `{"command":"/usr/bin/ls"}`, true},
		{"grep", `{"command":"grep","args":["-r","foo","."]}`, true},
		{"rg", `{"command":"rg","args":["needle"]}`, true},
		{"git status", `{"command":"git","args":["status"]}`, true},
		{"git log oneline", `{"command":"git","args":["log","--oneline"]}`, true},
		{"git diff", `{"command":"git","args":["diff","HEAD~1"]}`, true},
		{"git with global flag", `{"command":"git","args":["-C","foo","status"]}`, true},
		{"git commit", `{"command":"git","args":["commit","-m","hi"]}`, false},
		{"git branch (ambiguous)", `{"command":"git","args":["branch","-d","old"]}`, false},
		{"git config (ambiguous)", `{"command":"git","args":["config","user.name","x"]}`, false},
		{"find with args", `{"command":"find","args":[".","-name","*.go"]}`, true},
		{"find -delete", `{"command":"find","args":[".","-delete"]}`, false},
		{"find -exec", `{"command":"find","args":[".","-exec","rm","{}",";"]}`, false},
		{"go version", `{"command":"go","args":["version"]}`, true},
		{"go env GOOS", `{"command":"go","args":["env","GOOS"]}`, true},
		{"go env -w", `{"command":"go","args":["env","-w","GOFLAGS=-mod=vendor"]}`, false},
		{"go build", `{"command":"go","args":["build","./..."]}`, false},
		{"go test", `{"command":"go","args":["test","./..."]}`, false},
		{"npm list", `{"command":"npm","args":["list"]}`, true},
		{"npm install", `{"command":"npm","args":["install","react"]}`, false},
		{"npm config get", `{"command":"npm","args":["config","get","registry"]}`, true},
		{"npm config set", `{"command":"npm","args":["config","set","registry","x"]}`, false},
		{"node --version", `{"command":"node","args":["--version"]}`, true},
		{"node script", `{"command":"node","args":["script.js"]}`, false},
		{"rm", `{"command":"rm","args":["foo"]}`, false},
		{"unknown binary", `{"command":"mkdir","args":["foo"]}`, false},
		{"empty input", `{"command":""}`, false},
	}

	tool := NewRunCommandTool()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tool.ReadOnlyForInput(json.RawMessage(tc.in))
			if got != tc.want {
				t.Fatalf("ReadOnlyForInput(%s) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
