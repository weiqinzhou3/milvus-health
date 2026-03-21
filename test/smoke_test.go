package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICommandsSmoke(t *testing.T) {
	t.Parallel()

	bin := filepath.Join(t.TempDir(), "milvus-health")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build error: %v\n%s", err, out)
	}

	tests := []struct {
		name     string
		args     []string
		exitCode int
	}{
		{name: "version", args: []string{"version"}, exitCode: 0},
		{name: "check help", args: []string{"check", "--help"}, exitCode: 0},
		{name: "validate help", args: []string{"validate", "--help"}, exitCode: 0},
		{name: "validate example config", args: []string{"validate", "--config", filepath.Join("examples", "config.example.yaml")}, exitCode: 0},
		{name: "check text", args: []string{"check", "--config", filepath.Join("examples", "config.example.yaml"), "--format", "text"}, exitCode: 1},
		{name: "check json", args: []string{"check", "--config", filepath.Join("examples", "config.example.yaml"), "--format", "json"}, exitCode: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(bin, tt.args...)
			cmd.Dir = filepath.Join("..")
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if got := exitCodeOf(err); got != tt.exitCode {
				t.Fatalf("run %v exit=%d want=%d\nstdout=%s\nstderr=%s", tt.args, got, tt.exitCode, stdout.String(), stderr.String())
			}
			if strings.Contains(strings.Join(tt.args, " "), "--format json") && !jsonLike(stdout.String()) {
				t.Fatalf("json check output is not json: %s", stdout.String())
			}
			if len(tt.args) >= 1 && tt.args[0] == "validate" && len(tt.args) > 1 && tt.args[1] == "--config" {
				if !strings.Contains(stdout.String(), "config validation succeeded") {
					t.Fatalf("validate stdout = %q", stdout.String())
				}
			}
		})
	}
}

func TestExamplesMatchCurrentOutputs(t *testing.T) {
	t.Parallel()

	bin := filepath.Join(t.TempDir(), "milvus-health")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build error: %v\n%s", err, out)
	}

	configPath := filepath.Join("examples", "config.example.yaml")
	textCmd := exec.Command(bin, "check", "--config", configPath, "--format", "text")
	textCmd.Dir = filepath.Join("..")
	textOut, err := textCmd.CombinedOutput()
	if got := exitCodeOf(err); got != 1 {
		t.Fatalf("text check exit=%d want=1\n%s", got, textOut)
	}
	wantText, err := os.ReadFile(filepath.Join("..", "examples", "output.text.example.txt"))
	if err != nil {
		t.Fatalf("read text example: %v", err)
	}
	if string(textOut) != string(wantText) {
		t.Fatalf("text example mismatch\nwant:\n%s\ngot:\n%s", wantText, textOut)
	}

	jsonCmd := exec.Command(bin, "check", "--config", configPath, "--format", "json")
	jsonCmd.Dir = filepath.Join("..")
	jsonOut, err := jsonCmd.CombinedOutput()
	if got := exitCodeOf(err); got != 1 {
		t.Fatalf("json check exit=%d want=1\n%s", got, jsonOut)
	}
	wantJSON, err := os.ReadFile(filepath.Join("..", "examples", "output.json.example.json"))
	if err != nil {
		t.Fatalf("read json example: %v", err)
	}
	if string(jsonOut) != string(wantJSON) {
		t.Fatalf("json example mismatch\nwant:\n%s\ngot:\n%s", wantJSON, jsonOut)
	}
}

func jsonLike(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
