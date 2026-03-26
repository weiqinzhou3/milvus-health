package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/cli"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/render"
)

type fakeCheckRunner struct {
	result *model.AnalysisResult
	err    error
}

func (f fakeCheckRunner) Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error) {
	return f.result, f.err
}

type fakeValidateRunner struct {
	err error
}

func (f fakeValidateRunner) Run(ctx context.Context, opts model.ValidateOptions) error {
	return f.err
}

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

func TestExecute_Check_ReturnsWarnExitCode(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, dependencies{
		checkRunner: fakeCheckRunner{
			result: &model.AnalysisResult{Result: model.FinalResultWARN},
		},
		validateRunner:  fakeValidateRunner{},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	})

	exitCode := app.Execute([]string{"check", "--config", "test.yaml", "--format", "text"})
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCheckHelp_DescribesSafeAndDangerousModes(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, dependencies{
		checkRunner:     fakeCheckRunner{},
		validateRunner:  fakeValidateRunner{},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	})

	exitCode := app.Execute([]string{"check", "--help"})
	if exitCode != 0 {
		t.Fatalf("Execute() = %d, want 0", exitCode)
	}
	out := stdout.String()
	for _, token := range []string{
		"Default mode is safe",
		"probe.rw.enabled=true",
		"does not delete historical prefixed test databases",
		"override probe.rw.cleanup for current-run RW resources only",
	} {
		if !strings.Contains(out, token) {
			t.Fatalf("help output missing %q: %s", token, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestExecute_Check_ReturnsFailExitCode(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, dependencies{
		checkRunner: fakeCheckRunner{
			result: &model.AnalysisResult{Result: model.FinalResultFAIL},
		},
		validateRunner:  fakeValidateRunner{},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	})

	exitCode := app.Execute([]string{"check", "--config", "test.yaml", "--format", "text"})
	if exitCode != 2 {
		t.Fatalf("Execute() = %d, want 2", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestExecute_Validate_ReturnsZero_OnSuccess(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteArgs([]string{"validate", "--config", filepath.Join("..", "examples", "config.example.yaml")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("ExecuteArgs() = %d, want 0", exitCode)
	}
}

func TestExecute_Validate_Returns3_OnConfigInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.yaml")
	content := "cluster:\n  name: test\n  milvus:\n    uri: tcp://bad:19530\noutput:\n  format: text\nprobe:\n  read:\n    min_success_targets: 1\n    targets:\n      - database: default\n        collection: book\n        query_expr: \"id >= 0\"\n  rw:\n    enabled: false\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteArgs([]string{"validate", "--config", configPath}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("ExecuteArgs() = %d, want 3", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr should contain validation error")
	}
}

func TestExecute_Validate_Returns3_OnUnknownField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.yaml")
	content := "cluster:\n  name: test\n  milvus:\n    uri: localhost:19530\noutput:\n  format: text\n  unexpected: true\nprobe:\n  read:\n    min_success_targets: 1\n    targets:\n      - database: default\n        collection: book\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteArgs([]string{"validate", "--config", configPath}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("ExecuteArgs() = %d, want 3", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unexpected") {
		t.Fatalf("stderr = %q, want unknown field message", stderr.String())
	}
}

func TestValidate_Success_WritesExpectedStdout(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteArgs([]string{"validate", "--config", filepath.Join("..", "examples", "config.example.yaml")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("ExecuteArgs() = %d, want 0", exitCode)
	}
	if stdout.String() != "config validation succeeded\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCheck_JSON_WritesOnlyJSONToStdout(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteArgs([]string{"check", "--config", filepath.Join("..", "examples", "config.example.yaml"), "--format", "json"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("ExecuteArgs() = %d, want 2", exitCode)
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(out, "{") || !strings.HasSuffix(out, "}") {
		t.Fatalf("stdout is not pure JSON: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCheck_Error_WritesToStderr(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, dependencies{
		checkRunner: fakeCheckRunner{
			err: &model.AppError{Code: model.ErrCodeRuntime, Message: "boom"},
		},
		validateRunner:  fakeValidateRunner{},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	})

	exitCode := app.Execute([]string{"check", "--config", "test.yaml"})
	if exitCode != 4 {
		t.Fatalf("Execute() = %d, want 4", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNoLegacyModuleImports(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..")
	var legacy []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.Contains(text, "\n\t\"milvus-health/internal/") || strings.Contains(text, "\n\t\"milvus-health/cmd") {
			legacy = append(legacy, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(legacy) > 0 {
		t.Fatalf("found legacy imports: %v", legacy)
	}
}

func TestExecute_Validate_Returns4_OnUnexpectedRunnerError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, dependencies{
		checkRunner:     fakeCheckRunner{},
		validateRunner:  fakeValidateRunner{err: errors.New("unexpected")},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	})

	exitCode := app.Execute([]string{"validate", "--config", "test.yaml"})
	if exitCode != 4 {
		t.Fatalf("Execute() = %d, want 4", exitCode)
	}
}
