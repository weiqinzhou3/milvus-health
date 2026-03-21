package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/cli"
	"github.com/weiqinzhou3/milvus-health/internal/collectors"
	"github.com/weiqinzhou3/milvus-health/internal/config"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
	"github.com/weiqinzhou3/milvus-health/internal/render"
)

const Version = "0.1.0-skeleton"

type dependencies struct {
	checkRunner     cli.CheckRunner
	validateRunner  cli.ValidateRunner
	rendererFactory render.RendererFactory
	exitMapper      cli.ExitCodeMapper
}

type executionState struct {
	exitCode int
}

type app struct {
	root       *cobra.Command
	state      *executionState
	exitMapper cli.ExitCodeMapper
}

func NewRootCmd(stdout, stderr io.Writer) *cobra.Command {
	return newApp(stdout, stderr, defaultDependencies()).root
}

func newApp(stdout, stderr io.Writer, deps dependencies) *app {
	state := &executionState{}
	root := &cobra.Command{
		Use:           "milvus-health",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newVersionCmd(stdout))
	root.AddCommand(newValidateCmd(stdout, stderr, deps.validateRunner))
	root.AddCommand(newCheckCmd(stdout, stderr, deps.checkRunner, deps.rendererFactory, deps.exitMapper, state))
	return &app{
		root:       root,
		state:      state,
		exitMapper: deps.exitMapper,
	}
}

func newVersionCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(stdout, Version)
			return err
		},
	}
}

func newValidateCmd(stdout, stderr io.Writer, runner cli.ValidateRunner) *cobra.Command {
	_ = stderr
	var opts model.ValidateOptions
	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate config statically",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.ConfigPath == "" {
				return &model.AppError{Code: model.ErrCodeConfigInvalid, Message: "--config is required"}
			}
			if err := runner.Run(context.Background(), opts); err != nil {
				return err
			}
			_, err := fmt.Fprintln(stdout, "config validation succeeded")
			return err
		},
	}
	command.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	command.Flags().BoolVar(&opts.Verbose, "verbose", false, "enable verbose stderr logs")
	return command
}

func newCheckCmd(stdout, stderr io.Writer, runner cli.CheckRunner, factory render.RendererFactory, exitMapper cli.ExitCodeMapper, state *executionState) *cobra.Command {
	_ = stderr
	var opts model.CheckOptions
	command := &cobra.Command{
		Use:   "check",
		Short: "Run read-only health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.ConfigPath == "" {
				return &model.AppError{Code: model.ErrCodeConfigInvalid, Message: "--config is required"}
			}
			result, err := runner.Run(context.Background(), opts)
			if err != nil {
				return err
			}
			result.ExitCode = exitMapper.FromAnalysis(result)
			format := opts.Format
			if format == "" {
				format = model.OutputFormatText
			}
			renderer, err := factory.Get(format)
			if err != nil {
				return &model.AppError{Code: model.ErrCodeRender, Cause: err}
			}
			out, err := renderer.Render(result, render.RenderOptions{Detail: opts.Detail})
			if err != nil {
				return &model.AppError{Code: model.ErrCodeRender, Cause: err}
			}
			state.exitCode = result.ExitCode
			if _, err := stdout.Write(out); err != nil {
				return err
			}
			if len(out) == 0 || out[len(out)-1] != '\n' {
				_, _ = fmt.Fprintln(stdout)
			}
			return nil
		},
	}
	command.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	command.Flags().BoolVar(&opts.Verbose, "verbose", false, "enable verbose stderr logs")
	command.Flags().IntVar(&opts.TimeoutSec, "timeout", 0, "timeout in seconds")
	command.Flags().StringVar((*string)(&opts.Format), "format", "", "output format: text|json")
	command.Flags().BoolVar(&opts.Detail, "detail", false, "render detailed checks")
	command.Flags().Bool("cleanup", false, "override probe.rw.cleanup")
	command.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("cleanup") {
			value, err := cmd.Flags().GetBool("cleanup")
			if err != nil {
				return err
			}
			opts.Cleanup = &value
		}
		return nil
	}
	return command
}

func Execute() int {
	return ExecuteArgs(os.Args[1:], os.Stdout, os.Stderr)
}

func ExecuteArgs(args []string, stdout, stderr io.Writer) int {
	return newApp(stdout, stderr, defaultDependencies()).Execute(args)
}

func defaultDependencies() dependencies {
	return dependencies{
		checkRunner: cli.DefaultCheckRunner{
			Loader:          config.YAMLLoader{},
			Validator:       config.ConfigValidator{},
			DefaultApplier:  config.DefaultValueApplier{},
			OverrideApplier: config.CLIOverrideApplier{},
			Analyzer: analyzers.InventoryAnalyzer{
				MilvusCollector: collectors.DefaultMilvusInventoryCollector{
					Factory: platform.SDKMilvusClientFactory{},
				},
				K8sCollector: collectors.DefaultK8sInventoryCollector{
					Factory: platform.ClientGoK8sClientFactory{},
				},
			},
		},
		validateRunner: cli.DefaultValidateRunner{
			Loader:         config.YAMLLoader{},
			Validator:      config.ConfigValidator{},
			DefaultApplier: config.DefaultValueApplier{},
		},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	}
}

func (a *app) Execute(args []string) int {
	a.root.SetArgs(args)
	a.root.SetOut(a.root.OutOrStdout())
	a.root.SetErr(a.root.ErrOrStderr())
	if err := a.root.Execute(); err != nil {
		_, _ = fmt.Fprintln(a.root.ErrOrStderr(), err.Error())
		return a.exitMapper.FromError(err)
	}
	return a.state.exitCode
}
