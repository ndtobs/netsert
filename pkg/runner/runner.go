package runner

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/config"
	"github.com/ndtobs/netsert/pkg/gnmiclient"
)

// Runner executes assertions against targets
type Runner struct {
	Output   io.Writer
	Timeout  time.Duration
	Parallel int
	Verbose  bool
	Config   *config.Config
}

// RunResult contains the results of a run
type RunResult struct {
	TotalAssertions int
	Passed          int
	Failed          int
	Errors          int
	Results         []*assertion.Result
	Duration        time.Duration
}

// NewRunner creates a new runner with defaults
func NewRunner(output io.Writer) *Runner {
	return &Runner{
		Output:   output,
		Timeout:  30 * time.Second,
		Parallel: 1,
	}
}

// Run executes all assertions in the file
func (r *Runner) Run(ctx context.Context, af *assertion.AssertionFile) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{}

	for _, target := range af.Targets {
		// Apply config credentials if not specified in assertion file
		target = r.applyConfig(target)

		targetResults, err := r.runTarget(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", target.Address, err)
		}
		result.Results = append(result.Results, targetResults...)
	}

	// Tally results
	for _, res := range result.Results {
		result.TotalAssertions++
		if res.Error != nil {
			result.Errors++
		} else if res.Passed {
			result.Passed++
		} else {
			result.Failed++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// applyConfig merges config settings into target (assertion file takes precedence)
func (r *Runner) applyConfig(target assertion.Target) assertion.Target {
	if r.Config == nil {
		return target
	}

	username, password, insecure := r.Config.GetCredentials(target.Address)

	// Only apply if not already set in assertion file
	if target.Username == "" {
		target.Username = username
	}
	if target.Password == "" {
		target.Password = password
	}
	if !target.Insecure {
		target.Insecure = insecure
	}

	return target
}

func (r *Runner) runTarget(ctx context.Context, target assertion.Target) ([]*assertion.Result, error) {
	// Connect to target
	client, err := gnmiclient.NewClient(gnmiclient.Config{
		Address:  target.Address,
		Username: target.Username,
		Password: target.Password,
		Insecure: target.Insecure,
		Timeout:  r.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	var results []*assertion.Result
	var mu sync.Mutex

	// Run assertions
	sem := make(chan struct{}, max(r.Parallel, 1))
	var wg sync.WaitGroup

	for _, a := range target.Assertions {
		wg.Add(1)
		a := a // capture

		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := r.runAssertion(ctx, client, target, a)
			res.Target = target.Address

			mu.Lock()
			results = append(results, res)
			mu.Unlock()

			r.printResult(res)
		}()
	}

	wg.Wait()
	return results, nil
}

func (r *Runner) runAssertion(ctx context.Context, client *gnmiclient.Client, target assertion.Target, a assertion.Assertion) *assertion.Result {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	value, exists, err := client.Get(ctx, a.Path, target.Username, target.Password)
	if err != nil {
		return &assertion.Result{
			Assertion: a,
			Error:     err,
		}
	}

	return a.Validate(value, exists)
}

func (r *Runner) printResult(res *assertion.Result) {
	if r.Output == nil {
		return
	}

	icon := "✓"
	status := "PASS"
	if res.Error != nil {
		icon = "✗"
		status = "ERROR"
	} else if !res.Passed {
		icon = "✗"
		status = "FAIL"
	}

	name := res.Assertion.GetName()
	if len(name) > 60 {
		name = name[:57] + "..."
	}

	fmt.Fprintf(r.Output, "%s [%s] %s @ %s\n", icon, status, name, res.Target)

	if r.Verbose && (res.Error != nil || !res.Passed) {
		if res.Error != nil {
			fmt.Fprintf(r.Output, "    error: %v\n", res.Error)
		}
		if res.ActualValue != "" {
			fmt.Fprintf(r.Output, "    actual: %s\n", res.ActualValue)
		}
		if res.Assertion.Equals != nil {
			fmt.Fprintf(r.Output, "    expected: %s\n", *res.Assertion.Equals)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
