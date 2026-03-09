package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chapsuk/wait-jobs/internal/k8s"
	"github.com/chapsuk/wait-jobs/internal/printer"
	"github.com/chapsuk/wait-jobs/internal/runner"
)

type options struct {
	namespace  string
	selector   string
	timeout    time.Duration
	logs       string
	kubeconfig string
	context    string
	noColor    bool
}

func Execute() int {
	opts := &options{}
	cmd := newRootCmd(opts)
	if err := cmd.Execute(); err != nil {
		var codeErr exitCodeError
		if errors.As(err, &codeErr) {
			fmt.Fprintln(os.Stderr, codeErr.err)
			return codeErr.code
		}
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	return 0
}

func newRootCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "wait-jobs [job-name...]",
		Short:         "Wait for multiple Kubernetes jobs to finish",
		Long:          "wait-jobs watches Kubernetes Jobs in parallel and exits when all selected jobs are complete/failed/deleted. If no job names and no selector are provided, it watches all jobs in the namespace.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := runner.LogMode(opts.logs)
			if mode != runner.LogModeAll && mode != runner.LogModeFailed && mode != runner.LogModeNone {
				return fmt.Errorf("invalid --logs value %q (allowed: all|failed|none)", opts.logs)
			}

			init, err := k8s.InitClientset(opts.kubeconfig, opts.context)
			if err != nil {
				return exitCodeError{code: 3, err: err}
			}

			ns := opts.namespace
			if ns == "" {
				ns = init.Namespace
			}
			if ns == "" {
				ns = "default"
			}

			if init.Source == "in-cluster" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Using in-cluster config (ServiceAccount)")
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Using kubeconfig: %s\n", init.KubeconfigPath)
			}

			p := printer.New(cmd.OutOrStdout(), opts.noColor)
			res, runErr := runner.Run(context.Background(), init.Clientset, p, runner.Options{
				Namespace: ns,
				Selector:  opts.selector,
				JobNames:  args,
				Timeout:   opts.timeout,
				LogMode:   mode,
			})

			if errors.Is(runErr, context.DeadlineExceeded) {
				return exitCodeError{code: 2, err: runErr}
			}
			if runErr != nil {
				return exitCodeError{code: 3, err: runErr}
			}
			if res.Failed > 0 {
				return exitCodeError{code: 1, err: fmt.Errorf("one or more jobs failed")}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().StringVarP(&opts.selector, "selector", "l", "", "Label selector for jobs")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 5*time.Minute, "Maximum wait time")
	cmd.Flags().StringVar(&opts.logs, "logs", "none", "Logs output mode: all|failed|none")
	cmd.Flags().StringVar(&opts.kubeconfig, "kubeconfig", "", "Path to kubeconfig")
	cmd.Flags().StringVar(&opts.context, "context", "", "Kubernetes context")
	cmd.Flags().BoolVar(&opts.noColor, "no-color", false, "Disable ANSI colors")

	return cmd
}

type exitCodeError struct {
	code int
	err  error
}

func (e exitCodeError) Error() string {
	return e.err.Error()
}
