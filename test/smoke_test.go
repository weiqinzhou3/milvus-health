package test

import (
	"bytes"
	"os/exec"
	"path/filepath"
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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(bin, tt.args...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %v error: %v\nstdout=%s\nstderr=%s", tt.args, err, stdout.String(), stderr.String())
			}
		})
	}
}
