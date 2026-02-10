package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/config"
	"github.com/ndtobs/netsert/pkg/gnmiclient"
	"github.com/ndtobs/netsert/pkg/runner"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	// Global flags
	verbose bool
	timeout time.Duration
	output  string
)

// JSONOutput is the structure for JSON output
type JSONOutput struct {
	Summary   JSONSummary    `json:"summary"`
	Results   []JSONResult   `json:"results"`
}

type JSONSummary struct {
	File     string  `json:"file"`
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Errors   int     `json:"errors"`
	Duration string  `json:"duration"`
	Success  bool    `json:"success"`
}

type JSONResult struct {
	Target      string `json:"target"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Status      string `json:"status"` // "pass", "fail", "error"
	Actual      string `json:"actual,omitempty"`
	Expected    string `json:"expected,omitempty"`
	Error       string `json:"error,omitempty"`
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "netsert",
		Short:   "Declarative network state assertions using gNMI",
		Version: version,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "timeout per assertion")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "text", "output format (text, json)")

	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(getCmd())

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

			if output == "json" {
				out := map[string]interface{}{
					"valid":      true,
					"targets":    len(af.Targets),
					"assertions": totalAssertions,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
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

	// Load config (credentials, defaults)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	// For JSON output, suppress text output from runner
	var runnerOutput io.Writer = os.Stdout
	if output == "json" {
		runnerOutput = io.Discard
	}

	r := runner.NewRunner(runnerOutput)
	r.Timeout = timeout
	r.Parallel = parallel
	r.Verbose = verbose
	r.Config = cfg

	if output != "json" {
		fmt.Printf("Running assertions from %s\n\n", path)
	}

	result, err := r.Run(ctx, af)
	if err != nil {
		return err
	}

	if output == "json" {
		return outputJSON(path, result)
	}

	// Text output
	fmt.Println()
	fmt.Printf("Completed in %s\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("  Total:  %d\n", result.TotalAssertions)
	fmt.Printf("  Passed: %d\n", result.Passed)
	fmt.Printf("  Failed: %d\n", result.Failed)
	if result.Errors > 0 {
		fmt.Printf("  Errors: %d\n", result.Errors)
	}

	if result.Failed > 0 || result.Errors > 0 {
		os.Exit(1)
	}

	return nil
}

func getCmd() *cobra.Command {
	var (
		username string
		password string
		insecure bool
	)

	cmd := &cobra.Command{
		Use:   "get <target> <path>",
		Short: "Query a gNMI path on a device",
		Long: `Query a single gNMI path on a device to discover available data.

Examples:
  netsert get spine1:6030 /interfaces/interface[name=Ethernet1]/state/oper-status
  netsert get spine1:6030 /system/config/hostname
  netsert get spine1:6030 /interfaces/interface --insecure

Use this to explore what paths are available and what values they return.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(args[0], args[1], username, password, insecure)
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "username (or use config file)")
	cmd.Flags().StringVarP(&password, "password", "P", "", "password (or use config file)")
	cmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "skip TLS verification")

	return cmd
}

func runGet(target, path, username, password string, insecure bool) error {
	// Load config for credentials if not provided
	cfg, _ := config.Load()
	if cfg != nil && (username == "" || password == "") {
		cfgUser, cfgPass, cfgInsecure := cfg.GetCredentials(target)
		if username == "" {
			username = cfgUser
		}
		if password == "" {
			password = cfgPass
		}
		if !insecure {
			insecure = cfgInsecure
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := gnmiclient.NewClient(gnmiclient.Config{
		Address:  target,
		Username: username,
		Password: password,
		Insecure: insecure,
		Timeout:  timeout,
	})
	if err != nil {
		return fmt.Errorf("connect to %s: %w", target, err)
	}
	defer client.Close()

	value, exists, err := client.Get(ctx, path, username, password)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}

	if output == "json" {
		out := map[string]interface{}{
			"target": target,
			"path":   path,
			"exists": exists,
			"value":  value,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if !exists {
		fmt.Printf("Path: %s\n", path)
		fmt.Printf("Exists: false\n")
		return nil
	}

	fmt.Printf("Path: %s\n", path)
	fmt.Printf("Value: %s\n", value)

	return nil
}

func outputJSON(path string, result *runner.RunResult) error {
	out := JSONOutput{
		Summary: JSONSummary{
			File:     path,
			Total:    result.TotalAssertions,
			Passed:   result.Passed,
			Failed:   result.Failed,
			Errors:   result.Errors,
			Duration: result.Duration.Round(time.Millisecond).String(),
			Success:  result.Failed == 0 && result.Errors == 0,
		},
		Results: make([]JSONResult, 0, len(result.Results)),
	}

	for _, res := range result.Results {
		jr := JSONResult{
			Target: res.Target,
			Name:   res.Assertion.GetName(),
			Path:   res.Assertion.Path,
			Actual: res.ActualValue,
		}

		if res.Error != nil {
			jr.Status = "error"
			jr.Error = res.Error.Error()
		} else if res.Passed {
			jr.Status = "pass"
		} else {
			jr.Status = "fail"
		}

		// Add expected value if it was an equals assertion
		if res.Assertion.Equals != nil {
			jr.Expected = *res.Assertion.Equals
		}

		out.Results = append(out.Results, jr)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}

	if result.Failed > 0 || result.Errors > 0 {
		os.Exit(1)
	}

	return nil
}
