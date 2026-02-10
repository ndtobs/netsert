# netsert Code Walkthrough

A complete guide to how netsert works, from CLI to gNMI.

---

## Architecture Overview

```
User runs: ./netsert run assertions.yaml
                    │
                    ▼
            ┌──────────────┐
            │   main.go    │  CLI parsing (cobra)
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │   loader.go  │  Parse YAML → AssertionFile struct
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │  runner.go   │  For each target, for each assertion...
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │  client.go   │  gNMI Get request → extract value
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │  types.go    │  Validate(actualValue) → pass/fail
            └──────────────┘
```

**File locations:**
```
netsert/
├── cmd/netsert/main.go        # CLI entry point
├── pkg/
│   ├── assertion/
│   │   ├── types.go           # Data structures + validation logic
│   │   └── loader.go          # YAML parsing
│   ├── gnmiclient/
│   │   └── client.go          # gNMI client wrapper
│   └── runner/
│       └── runner.go          # Test execution orchestration
```

---

## 1. Entry Point: cmd/netsert/main.go

This is where execution starts.

### What it does:
- Uses [Cobra](https://github.com/spf13/cobra) library for CLI parsing
- Defines two commands: `run` and `validate`
- Handles command-line flags like `--verbose` and `--timeout`
- Sets up signal handling for Ctrl+C

### Key code explained:

```go
func main() {
    rootCmd := &cobra.Command{
        Use:   "netsert",
        Short: "Declarative network state assertions using gNMI",
    }

    // Global flags available to all commands
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
    rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "timeout per assertion")

    // Add subcommands
    rootCmd.AddCommand(runCmd())
    rootCmd.AddCommand(validateCmd())

    rootCmd.Execute()
}
```

### The run command flow:

```go
func runAssertions(path string, parallel int, failFast bool) error {
    // 1. Load the YAML file
    af, err := assertion.LoadFile(path)

    // 2. Set up context with cancellation (for Ctrl+C handling)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // 3. Create a runner with our settings
    r := runner.NewRunner(os.Stdout)
    r.Timeout = timeout
    r.Parallel = parallel
    r.Verbose = verbose

    // 4. Execute all assertions
    result, err := r.Run(ctx, af)

    // 5. Print summary and exit with code 1 if failures
    if result.Failed > 0 || result.Errors > 0 {
        os.Exit(1)
    }
}
```

---

## 2. Data Structures: pkg/assertion/types.go

This file defines what assertions look like and how to validate them.

### The YAML structure maps to Go structs:

```yaml
# What you write in YAML:
targets:
  - address: spine1:6030
    username: admin
    password: admin
    insecure: true
    assertions:
      - name: Ethernet1 is UP
        path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: "UP"
```

```go
// How it's represented in Go:

type AssertionFile struct {
    Targets []Target `yaml:"targets"`
}

type Target struct {
    Address    string      `yaml:"address"`
    Username   string      `yaml:"username,omitempty"`
    Password   string      `yaml:"password,omitempty"`
    Insecure   bool        `yaml:"insecure,omitempty"`
    Assertions []Assertion `yaml:"assertions"`
}

type Assertion struct {
    Name        string  `yaml:"name,omitempty"`
    Description string  `yaml:"description,omitempty"`
    Path        string  `yaml:"path"`

    // Assertion types - only ONE should be set
    Equals   *string `yaml:"equals,omitempty"`
    Contains *string `yaml:"contains,omitempty"`
    Matches  *string `yaml:"matches,omitempty"`
    Exists   *bool   `yaml:"exists,omitempty"`
    Absent   *bool   `yaml:"absent,omitempty"`
    GT       *string `yaml:"gt,omitempty"`
    LT       *string `yaml:"lt,omitempty"`
    GTE      *string `yaml:"gte,omitempty"`
    LTE      *string `yaml:"lte,omitempty"`
}
```

### Why pointers (`*string`) instead of just `string`?

This is a common Go pattern for optional fields:
- `nil` means "not set" (user didn't specify this assertion type)
- Non-nil means "set" (even if the value is empty string)

```go
// If we used plain string:
Equals string  // "" could mean "not set" OR "check for empty string" - ambiguous!

// With pointer:
Equals *string // nil = not set, &"" = check for empty string - clear!
```

### The Validate() method - core logic:

```go
func (a *Assertion) Validate(value string, exists bool) *Result {
    result := &Result{
        Assertion:   *a,
        ActualValue: value,
        Passed:      false,  // assume fail until proven otherwise
    }

    // Check "exists" assertion first
    if a.Exists != nil && *a.Exists {
        result.Passed = exists
        return result
    }

    // Check "absent" assertion
    if a.Absent != nil && *a.Absent {
        result.Passed = !exists
        return result
    }

    // For all other assertions, the path must exist
    if !exists {
        result.Error = fmt.Errorf("path does not exist")
        return result
    }

    // Check "equals"
    if a.Equals != nil {
        result.Passed = value == *a.Equals
        return result
    }

    // Check "contains"
    if a.Contains != nil {
        result.Passed = strings.Contains(value, *a.Contains)
        return result
    }

    // Check "matches" (regex)
    if a.Matches != nil {
        re, err := regexp.Compile(*a.Matches)
        if err != nil {
            result.Error = fmt.Errorf("invalid regex: %w", err)
            return result
        }
        result.Passed = re.MatchString(value)
        return result
    }

    // Numeric comparisons (gt, lt, gte, lte)
    if a.GT != nil || a.LT != nil || a.GTE != nil || a.LTE != nil {
        actualNum, err := strconv.ParseFloat(value, 64)
        if err != nil {
            result.Error = fmt.Errorf("value is not numeric: %w", err)
            return result
        }

        if a.GT != nil {
            threshold, _ := strconv.ParseFloat(*a.GT, 64)
            result.Passed = actualNum > threshold
        }
        // ... similar for LT, GTE, LTE
    }

    return result
}
```

---

## 3. YAML Loader: pkg/assertion/loader.go

Simple file that reads and parses YAML.

```go
func LoadFile(path string) (*AssertionFile, error) {
    // Read the file
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading file: %w", err)
    }

    return Parse(data)
}

func Parse(data []byte) (*AssertionFile, error) {
    var af AssertionFile

    // yaml.Unmarshal does the magic - converts YAML bytes to Go structs
    // The `yaml:"fieldname"` tags on the structs tell it how to map fields
    if err := yaml.Unmarshal(data, &af); err != nil {
        return nil, fmt.Errorf("parsing YAML: %w", err)
    }

    // Basic validation
    for i, target := range af.Targets {
        if target.Address == "" {
            return nil, fmt.Errorf("target %d: address is required", i)
        }
        for j, assertion := range target.Assertions {
            if assertion.Path == "" {
                return nil, fmt.Errorf("target %d, assertion %d: path is required", i, j)
            }
        }
    }

    return &af, nil
}
```

---

## 4. gNMI Client: pkg/gnmiclient/client.go

This is where we talk to network devices using gNMI (gRPC Network Management Interface).

### What is gNMI?

gNMI is a protocol for:
- **Getting** device state (what we use)
- **Setting** device config
- **Subscribing** to streaming telemetry

It runs over gRPC (Google's RPC framework) and uses structured paths based on YANG models.

### Creating a connection:

```go
func NewClient(cfg Config) (*Client, error) {
    var opts []grpc.DialOption

    // Set up TLS (or skip it for insecure connections)
    if cfg.Insecure {
        opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    } else {
        tlsConfig := &tls.Config{
            InsecureSkipVerify: true,  // TODO: proper cert validation
        }
        opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
    }

    // Open gRPC connection
    conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
    if err != nil {
        return nil, fmt.Errorf("dial: %w", err)
    }

    // Create gNMI client from the connection
    // gnmi.NewGNMIClient is generated from the gNMI protobuf definition
    return &Client{
        conn:   conn,
        client: gnmi.NewGNMIClient(conn),
    }, nil
}
```

### The Get() method - fetching a value:

```go
func (c *Client) Get(ctx context.Context, path string, username, password string) (string, bool, error) {
    // Returns: (value, exists, error)

    // 1. Convert string path to gNMI Path struct
    gnmiPath, err := parsePath(path)

    // 2. Build the request
    req := &gnmi.GetRequest{
        Path:     []*gnmi.Path{gnmiPath},
        Encoding: gnmi.Encoding_JSON_IETF,  // We want JSON responses
    }

    // 3. Add credentials to gRPC metadata (how gNMI auth works)
    if username != "" {
        ctx = metadata.AppendToOutgoingContext(ctx, "username", username, "password", password)
    }

    // 4. Make the RPC call
    resp, err := c.client.Get(ctx, req)
    if err != nil {
        // Check if it's just "path not found" (not a real error)
        if strings.Contains(err.Error(), "NotFound") {
            return "", false, nil  // exists = false
        }
        return "", false, err
    }

    // 5. Extract the value from the response
    // Response structure: Notification[] -> Update[] -> Val
    if len(resp.Notification) == 0 || len(resp.Notification[0].Update) == 0 {
        return "", false, nil  // no data = doesn't exist
    }

    update := resp.Notification[0].Update[0]
    value := extractValue(update.Val)

    return value, true, nil
}
```

### Path parsing - the tricky part:

gNMI paths look like XPath:
```
/interfaces/interface[name=Ethernet1]/state/oper-status
```

This needs to become a structured Path object:
```go
Path{
    Elem: []*PathElem{
        {Name: "interfaces"},
        {Name: "interface", Key: map[string]string{"name": "Ethernet1"}},
        {Name: "state"},
        {Name: "oper-status"},
    }
}
```

The tricky part is splitting on `/` but NOT when inside brackets `[]`:

```go
func splitPath(path string) []string {
    var segments []string
    var current strings.Builder
    depth := 0  // track bracket depth

    for _, r := range path {
        switch r {
        case '[':
            depth++
            current.WriteRune(r)
        case ']':
            depth--
            current.WriteRune(r)
        case '/':
            if depth == 0 {
                // Not inside brackets - this is a real separator
                if current.Len() > 0 {
                    segments = append(segments, current.String())
                    current.Reset()
                }
            } else {
                // Inside brackets - treat as regular character
                current.WriteRune(r)
            }
        default:
            current.WriteRune(r)
        }
    }

    // Don't forget the last segment
    if current.Len() > 0 {
        segments = append(segments, current.String())
    }

    return segments
}
```

### Extracting values from gNMI responses:

gNMI returns a `TypedValue` which can be many types:

```go
func extractValue(val *gnmi.TypedValue) string {
    if val == nil {
        return ""
    }

    // Type switch - check what kind of value it is
    switch v := val.Value.(type) {
    case *gnmi.TypedValue_StringVal:
        return v.StringVal
    case *gnmi.TypedValue_IntVal:
        return fmt.Sprintf("%d", v.IntVal)
    case *gnmi.TypedValue_UintVal:
        return fmt.Sprintf("%d", v.UintVal)
    case *gnmi.TypedValue_BoolVal:
        return fmt.Sprintf("%t", v.BoolVal)
    case *gnmi.TypedValue_JsonVal:
        return string(v.JsonVal)
    case *gnmi.TypedValue_JsonIetfVal:
        return string(v.JsonIetfVal)
    default:
        return fmt.Sprintf("%v", val.Value)
    }
}
```

---

## 5. Runner: pkg/runner/runner.go

Orchestrates everything - connects to devices and runs assertions.

### The Runner struct:

```go
type Runner struct {
    Output    io.Writer     // Where to print results (usually os.Stdout)
    Timeout   time.Duration // Per-assertion timeout
    Parallel  int           // Max concurrent assertions per target
    Verbose   bool          // Print extra details on failure
}
```

### Main Run() method:

```go
func (r *Runner) Run(ctx context.Context, af *assertion.AssertionFile) (*RunResult, error) {
    start := time.Now()
    result := &RunResult{}

    // Process each target (device) sequentially
    for _, target := range af.Targets {
        targetResults, err := r.runTarget(ctx, target)
        if err != nil {
            return nil, fmt.Errorf("target %s: %w", target.Address, err)
        }
        result.Results = append(result.Results, targetResults...)
    }

    // Count up pass/fail/error
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
```

### Running assertions for a single target:

```go
func (r *Runner) runTarget(ctx context.Context, target assertion.Target) ([]*assertion.Result, error) {
    // 1. Connect to the device
    client, err := gnmiclient.NewClient(gnmiclient.Config{
        Address:  target.Address,
        Username: target.Username,
        Password: target.Password,
        Insecure: target.Insecure,
    })
    if err != nil {
        return nil, fmt.Errorf("connect: %w", err)
    }
    defer client.Close()  // Always close when done

    var results []*assertion.Result
    var mu sync.Mutex  // Protects results slice from concurrent writes

    // 2. Set up concurrency control (semaphore pattern)
    sem := make(chan struct{}, max(r.Parallel, 1))
    var wg sync.WaitGroup

    // 3. Run each assertion in a goroutine
    for _, a := range target.Assertions {
        wg.Add(1)
        a := a  // IMPORTANT: capture loop variable (Go gotcha)

        go func() {
            defer wg.Done()

            // Acquire semaphore slot (blocks if at max concurrency)
            sem <- struct{}{}
            defer func() { <-sem }()  // Release slot when done

            // Run the assertion
            res := r.runAssertion(ctx, client, target, a)
            res.Target = target.Address

            // Safely append to results
            mu.Lock()
            results = append(results, res)
            mu.Unlock()

            // Print result immediately
            r.printResult(res)
        }()
    }

    // 4. Wait for all assertions to complete
    wg.Wait()
    return results, nil
}
```

### The semaphore pattern explained:

```go
// Create a buffered channel with capacity = max concurrent operations
sem := make(chan struct{}, 3)  // Allow 3 concurrent operations

// In each goroutine:
sem <- struct{}{}        // Try to send - blocks if channel full (3 items)
defer func() { <-sem }() // Receive when done - frees a slot
// ... do work ...
```

This limits how many goroutines can run simultaneously.

### Running a single assertion:

```go
func (r *Runner) runAssertion(ctx context.Context, client *gnmiclient.Client, 
                               target assertion.Target, a assertion.Assertion) *assertion.Result {
    // Set timeout for this assertion
    ctx, cancel := context.WithTimeout(ctx, r.Timeout)
    defer cancel()

    // Get the value from the device
    value, exists, err := client.Get(ctx, a.Path, target.Username, target.Password)
    if err != nil {
        return &assertion.Result{
            Assertion: a,
            Error:     err,
        }
    }

    // Validate the value against the assertion
    return a.Validate(value, exists)
}
```

---

## Complete Data Flow Example

Let's trace a single assertion through the entire system:

### Input (assertions.yaml):
```yaml
targets:
  - address: clab-netsert-spine1:6030
    username: admin
    password: admin
    insecure: true
    assertions:
      - name: Ethernet1 is UP
        path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: "UP"
```

### Step 1: main.go
```
./netsert run assertions.yaml
    │
    ▼
runAssertions("assertions.yaml", ...)
    │
    ▼
assertion.LoadFile("assertions.yaml")
```

### Step 2: loader.go
```
LoadFile() reads file bytes
    │
    ▼
yaml.Unmarshal() converts to:
AssertionFile{
    Targets: []Target{
        {
            Address: "clab-netsert-spine1:6030",
            Username: "admin",
            Password: "admin",
            Insecure: true,
            Assertions: []Assertion{
                {
                    Name: "Ethernet1 is UP",
                    Path: "/interfaces/interface[name=Ethernet1]/state/oper-status",
                    Equals: ptr("UP"),  // pointer to string "UP"
                },
            },
        },
    },
}
```

### Step 3: runner.go Run()
```
for each target in af.Targets:
    │
    ▼
runTarget(target)
    │
    ▼
gnmiclient.NewClient() - opens gRPC connection to spine1:6030
    │
    ▼
for each assertion in target.Assertions:
    │
    ▼ (in goroutine)
runAssertion()
```

### Step 4: client.go Get()
```
Get("/interfaces/interface[name=Ethernet1]/state/oper-status")
    │
    ▼
parsePath() converts string to:
Path{
    Elem: [
        {Name: "interfaces"},
        {Name: "interface", Key: {"name": "Ethernet1"}},
        {Name: "state"},
        {Name: "oper-status"},
    ]
}
    │
    ▼
Build GetRequest with path
    │
    ▼
c.client.Get(ctx, req)  -- gRPC call to device
    │
    ▼
Device returns:
GetResponse{
    Notification: [{
        Update: [{
            Path: ...,
            Val: TypedValue{StringVal: "UP"}
        }]
    }]
}
    │
    ▼
extractValue() returns "UP"
    │
    ▼
return ("UP", true, nil)
       value  exists  error
```

### Step 5: types.go Validate()
```
Validate("UP", true)
    │
    ▼
a.Equals != nil, so check:
    "UP" == "UP"  →  true
    │
    ▼
return Result{
    Passed: true,
    ActualValue: "UP",
}
```

### Step 6: Back to runner.go
```
printResult() outputs:
    ✓ [PASS] Ethernet1 is UP @ clab-netsert-spine1:6030
```

### Step 7: Back to main.go
```
Print summary:
    Completed in 90ms
      Total:  1
      Passed: 1
      Failed: 0

Exit code: 0 (success)
```

---

## Key Go Patterns Used

### 1. Error wrapping
```go
if err != nil {
    return nil, fmt.Errorf("reading file: %w", err)
}
```
The `%w` verb wraps the original error, preserving the chain.

### 2. Defer for cleanup
```go
client, err := gnmiclient.NewClient(...)
if err != nil {
    return nil, err
}
defer client.Close()  // Always runs when function returns
```

### 3. Context for cancellation/timeout
```go
ctx, cancel := context.WithTimeout(ctx, r.Timeout)
defer cancel()
value, _, err := client.Get(ctx, ...)  // Will abort if ctx times out
```

### 4. Goroutines + WaitGroup for concurrency
```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func() {
        defer wg.Done()
        // do work
    }()
}
wg.Wait()  // Block until all goroutines done
```

### 5. Mutex for thread-safe writes
```go
var mu sync.Mutex
mu.Lock()
results = append(results, res)
mu.Unlock()
```

### 6. Capturing loop variables
```go
for _, a := range assertions {
    a := a  // Create new variable that goroutine can safely capture
    go func() {
        // use a here
    }()
}
```
Without this, all goroutines would share the same `a` and get wrong values.

---

## Dependencies

From `go.mod`:

- **github.com/spf13/cobra** - CLI framework
- **gopkg.in/yaml.v3** - YAML parsing
- **github.com/openconfig/gnmi** - gNMI protobuf definitions
- **google.golang.org/grpc** - gRPC client

---

## What's Next?

Potential features to add:

1. **Watch mode** - Use gNMI Subscribe instead of Get for continuous validation
2. **JSON output** - `--output json` for CI integration
3. **Config file** - Default settings in `~/.netsert.yaml`
4. **Parallel targets** - Run against multiple devices simultaneously
5. **Init command** - Generate assertion file from device state
6. **More assertions** - `one-of`, `all-of`, `range`, etc.
