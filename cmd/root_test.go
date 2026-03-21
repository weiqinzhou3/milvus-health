package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecute_ReturnsMappedExitCode_OnCommandError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteArgs([]string{"validate"}, &stdout, &stderr)

	if exitCode != 3 {
		t.Fatalf("ExecuteArgs() = %d, want 3", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty on error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--config is required") {
		t.Fatalf("stderr = %q, want missing config message", stderr.String())
	}
}
