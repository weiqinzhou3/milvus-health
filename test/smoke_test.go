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
		name string
		args []string
	}{
		{name: "version", args: []string{"version"}},
		{name: "check help", args: []string{"check", "--help"}},
		{name: "validate help", args: []string{"validate", "--help"}},
		{name: "validate example config", args: []string{"validate", "--config", filepath.Join("examples", "config.example.yaml")}},
		{name: "check text", args: []string{"check", "--config", filepath.Join("examples", "config.example.yaml"), "--format", "text"}},
		{name: "check json", args: []string{"check", "--config", filepath.Join("examples", "config.example.yaml"), "--format", "json"}},
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
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %v error: %v\nstdout=%s\nstderr=%s", tt.args, err, stdout.String(), stderr.String())
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
	if err != nil {
		t.Fatalf("text check error: %v\n%s", err, textOut)
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
	if err != nil {
		t.Fatalf("json check error: %v\n%s", err, jsonOut)
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
