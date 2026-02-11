package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/config"
	"github.com/ndtobs/netsert/pkg/generate"
	"github.com/ndtobs/netsert/pkg/gnmiclient"
	"github.com/ndtobs/netsert/pkg/inventory"
	"github.com/ndtobs/netsert/pkg/runner"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	Summary JSONSummary  `json:"summary"`
	Results []JSONResult `json:"results"`
}

type JSONSummary struct {
	File     string `json:"file"`
	Total    int    `json:"total"`
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	Errors   int    `json:"errors"`
	Duration string `json:"duration"`
	Success  bool   `json:"success"`
}

type JSONResult struct {
	Target   string `json:"target"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Status   string `json:"status"` // "pass", "fail", "error"
	Actual   string `json:"actual,omitempty"`
	Expected string `json:"expected,omitempty"`
	Error    string `json:"error,omitempty"`
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
	rootCmd.AddCommand(generateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	var (
		workers       int
		parallel      int
		failFast      bool
		inventoryFile string
		group         string
	)

	cmd := &cobra.Command{
		Use:   "run <assertions.yaml>",
		Short: "Run assertions against targets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssertions(args[0], workers, parallel, failFast, inventoryFile, group)
		},
	}

	cmd.Flags().IntVarP(&workers, "workers", "w", runner.DefaultWorkers, "number of concurrent targets")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", runner.DefaultParallel, "number of parallel assertions per target")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop on first failure")
	cmd.Flags().StringVarP(&inventoryFile, "inventory", "i", "", "inventory file (YAML or INI format)")
	cmd.Flags().StringVarP(&group, "group", "g", "", "run only against hosts in this group")

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

func runAssertions(path string, workers, parallel int, failFast bool, inventoryFile, group string) error {
	af, err := assertion.LoadFile(path)
	if err != nil {
		return fmt.Errorf("load assertions: %w", err)
	}

	// Check if assertion file contains @group references
	hasGroupRefs := false
	for _, target := range af.Targets {
		if strings.HasPrefix(target.GetHost(), "@") {
			hasGroupRefs = true
			break
		}
	}

	// Load inventory
	var inv *inventory.Inventory
	if inventoryFile != "" {
		// Explicit inventory file provided
		inv, err = inventory.Load(inventoryFile)
		if err != nil {
			return fmt.Errorf("load inventory: %w", err)
		}
	} else if hasGroupRefs || group != "" {
		// Auto-discover inventory if @group refs found or -g flag used
		var invPath string
		inv, invPath, err = inventory.AutoDiscover()
		if err != nil {
			return fmt.Errorf("auto-discover inventory: %w", err)
		}
		if inv == nil {
			if hasGroupRefs {
				return fmt.Errorf("assertion file contains @group references but no inventory found - create inventory.yaml or pass -i")
			}
			return fmt.Errorf("--group/-g requires an inventory file - create inventory.yaml or pass -i")
		}
		if output != "json" {
			fmt.Printf("Using inventory: %s\n", invPath)
		}
	}

	// Expand group references if inventory is available
	if inv != nil {
		af = expandInventoryGroups(af, inv, group)

		// Check if filtering resulted in no targets
		if len(af.Targets) == 0 {
			if group != "" {
				return fmt.Errorf("no targets match group %q - check that assertion file uses @group syntax or hosts are in the group", group)
			}
			return fmt.Errorf("no targets found after expanding inventory groups")
		}
	}

	// Load config (credentials, defaults)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply inventory defaults to config if available
	if inv != nil && cfg != nil {
		if cfg.Defaults.Username == "" && inv.Defaults.Username != "" {
			cfg.Defaults.Username = inv.Defaults.Username
		}
		if cfg.Defaults.Password == "" && inv.Defaults.Password != "" {
			cfg.Defaults.Password = inv.Defaults.Password
		}
		if !cfg.Defaults.Insecure && inv.Defaults.Insecure {
			cfg.Defaults.Insecure = inv.Defaults.Insecure
		}
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
	r.Workers = workers
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

// expandInventoryGroups expands group references in assertion file targets
func expandInventoryGroups(af *assertion.AssertionFile, inv *inventory.Inventory, filterGroup string) *assertion.AssertionFile {
	var newTargets []assertion.Target

	for _, target := range af.Targets {
		// Check if this target references a group (starts with @)
		if strings.HasPrefix(target.GetHost(), "@") {
			groupName := strings.TrimPrefix(target.GetHost(), "@")
			hosts, ok := inv.GetGroup(groupName)
			if !ok {
				// Group not found, keep as-is (will fail later with connection error)
				newTargets = append(newTargets, target)
				continue
			}

			// Create a target for each host in the group
			for _, host := range hosts {
				newTarget := target
				newTarget.Host = host
				newTarget.Address = "" // Clear deprecated field
				newTargets = append(newTargets, newTarget)
			}
		} else {
			newTargets = append(newTargets, target)
		}
	}

	// Filter by group if specified
	if filterGroup != "" {
		hosts, ok := inv.GetGroup(filterGroup)
		if ok {
			hostSet := make(map[string]bool)
			for _, h := range hosts {
				hostSet[h] = true
			}

			var filtered []assertion.Target
			for _, t := range newTargets {
				if hostSet[t.GetHost()] {
					filtered = append(filtered, t)
				}
			}
			newTargets = filtered
		}
	}

	return &assertion.AssertionFile{Targets: newTargets}
}

func generateCmd() *cobra.Command {
	var (
		username      string
		password      string
		insecure      bool
		generators    []string
		outFile       string
		inventoryFile string
	)

	cmd := &cobra.Command{
		Use:   "generate <target>",
		Short: "Generate assertions from current device state",
		Long: `Query a device and generate assertion YAML from its current state.

Target can be a single host or @group to generate for all hosts in a group.

Available generators:
  bgp         - BGP neighbor session states
  interfaces  - Interface oper-status
  lldp        - LLDP neighbor relationships
  ospf        - OSPF neighbor states
  system      - Hostname and software version

Examples:
  netsert generate spine1:6030 --gen bgp
  netsert generate spine1:6030 --gen bgp --gen interfaces
  netsert generate spine1:6030 -f assertions.yaml
  netsert generate spine1:6030  # All generators
  netsert generate @spines      # All hosts in spines group
  netsert generate @all -f baseline.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(args[0], generators, username, password, insecure, outFile, inventoryFile)
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "username (or use config file)")
	cmd.Flags().StringVarP(&password, "password", "P", "", "password (or use config file)")
	cmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "skip TLS verification")
	cmd.Flags().StringArrayVar(&generators, "gen", nil, "generators to run (bgp, interfaces). Default: all")
	cmd.Flags().StringVarP(&outFile, "file", "f", "", "output file (default: stdout)")
	cmd.Flags().StringVarP(&inventoryFile, "inventory", "i", "", "inventory file (for @group targets)")

	return cmd
}

func runGenerate(target string, generators []string, username, password string, insecure bool, outFile, inventoryFile string) error {
	// Expand @group targets
	var targets []string
	if strings.HasPrefix(target, "@") {
		groupName := strings.TrimPrefix(target, "@")

		// Load inventory
		var inv *inventory.Inventory
		var err error
		if inventoryFile != "" {
			inv, err = inventory.Load(inventoryFile)
		} else {
			inv, _, err = inventory.AutoDiscover()
		}
		if err != nil {
			return fmt.Errorf("load inventory: %w", err)
		}
		if inv == nil {
			return fmt.Errorf("target %s requires inventory - create inventory.yaml or pass -i", target)
		}

		hosts, ok := inv.GetGroup(groupName)
		if !ok {
			return fmt.Errorf("group %q not found in inventory", groupName)
		}
		if len(hosts) == 0 {
			return fmt.Errorf("group %q is empty", groupName)
		}
		targets = hosts
	} else {
		targets = []string{target}
	}

	// Load config for credentials
	cfg, _ := config.Load()

	// Default to all generators
	if len(generators) == 0 {
		generators = generate.List()
	}

	// Generate for all targets
	var allTargets []assertion.Target
	var totalAssertions int

	for _, t := range targets {
		// Get credentials for this target
		u, p, ins := username, password, insecure
		if cfg != nil {
			cfgUser, cfgPass, cfgInsecure := cfg.GetCredentials(t)
			if u == "" {
				u = cfgUser
			}
			if p == "" {
				p = cfgPass
			}
			if !ins {
				ins = cfgInsecure
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		client, err := gnmiclient.NewClient(gnmiclient.Config{
			Address:  t,
			Username: u,
			Password: p,
			Insecure: ins,
			Timeout:  timeout,
		})
		if err != nil {
			cancel()
			return fmt.Errorf("connect to %s: %w", t, err)
		}

		af, err := generate.GenerateFile(ctx, client, generators, generate.Options{
			Target:   t,
			Username: u,
			Password: p,
		})
		client.Close()
		cancel()

		if err != nil {
			return fmt.Errorf("generate from %s: %w", t, err)
		}

		if len(af.Targets) > 0 {
			allTargets = append(allTargets, af.Targets[0])
			totalAssertions += len(af.Targets[0].Assertions)
		}

		if output != "json" && len(targets) > 1 {
			fmt.Fprintf(os.Stderr, "Generated from %s (%d assertions)\n", t, len(af.Targets[0].Assertions))
		}
	}

	// Combine into single file
	combined := &assertion.AssertionFile{Targets: allTargets}

	// Convert to YAML
	yamlData, err := yaml.Marshal(combined)
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}

	// Add header comment
	header := fmt.Sprintf("# Generated by netsert from %s\n# Review and edit as needed\n\n", target)
	result := header + string(yamlData)

	// Write to file or stdout
	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(result), 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated %d assertions (%d targets) to %s\n", totalAssertions, len(allTargets), outFile)
	} else {
		fmt.Print(result)
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
