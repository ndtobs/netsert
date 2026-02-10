package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/runner"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	// Global flags
	verbose bool
	timeout time.Duration
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "netsert",
		Short:   "Declarative network state assertions using gNMI",
		Version: version,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "timeout per assertion")

	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(validateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	var (
		parallel int
		failFast bool
	)

	cmd := &cobra.Command{
		Use:   "run <assertions.yaml>",
		Short: "Run assertions against targets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssertions(args[0], parallel, failFast)
		},
	}

	cmd.Flags().IntVarP(&parallel, "parallel", "p", 1, "number of parallel assertions per target")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop on first failure")

	return cmd
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <assertions.yaml>",
		Short: "Validate assertion file syntax",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			af, err := assertion.LoadFile(args[0])
			if err != nil {
				return err
			}

			totalAssertions := 0
			for _, t := range af.Targets {
				totalAssertions += len(t.Assertions)
			}

			fmt.Printf("✓ Valid: %d targets, %d assertions\n", len(af.Targets), totalAssertions)
			return nil
		},
	}
}

func runAssertions(path string, parallel int, failFast bool) error {
	af, err := assertion.LoadFile(path)
	if err != nil {
		return fmt.Errorf("load assertions: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nInterrupted, stopping...")
		cancel()
	}()

	r := runner.NewRunner(os.Stdout)
	r.Timeout = timeout
	r.Parallel = parallel
	r.Verbose = verbose

	fmt.Printf("Running assertions from %s\n\n", path)

	result, err := r.Run(ctx, af)
	if err != nil {
		return err
	}

	// Print summary
	fmt.Println()
	fmt.Printf("Completed in %s\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("  Total:  %d\n", result.TotalAssertions)
	fmt.Printf("  Passed: %d\n", result.Passed)
	fmt.Printf("  Failed: %d\n", result.Failed)
	if result.Errors > 0 {
		fmt.Printf("  Errors: %d\n", result.Errors)
	}

	// Exit with failure if any assertions failed
	if result.Failed > 0 || result.Errors > 0 {
		os.Exit(1)
	}

	return nil
}
