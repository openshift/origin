package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
)

type MCPOptions struct {
	ListenAddress string
	Mode          string
}

func (o *MCPOptions) Run() error {
	log.WithFields(log.Fields{
		"mode":           o.Mode,
		"listen_address": o.ListenAddress,
	}).Info("Initializing MCP server")

	// Create hooks for monitoring MCP protocol events
	hooks := &server.Hooks{}

	// Monitor session registration/unregistration
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		log.WithField("session_id", session.SessionID()).Info("MCP client session registered")
	})

	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		log.WithField("session_id", session.SessionID()).Info("MCP client session unregistered")
	})

	// Create the MCP server with common configuration
	mcpServer := server.NewMCPServer(
		"openshift-tests MCP Server",
		"0.0.1",
		server.WithToolCapabilities(false),
		server.WithLogging(),    // Enable MCP protocol logging
		server.WithRecovery(),   // Enable panic recovery
		server.WithHooks(hooks), // Add our monitoring hooks
		server.WithPromptCapabilities(false),
	)
	log.Debug("Created MCP server instance with logging, recovery, and monitoring hooks enabled")

	// Add list tests tool
	listTestsTool := mcp.NewTool("list_tests",
		mcp.WithDescription("List all available tests in JSON format"),
	)
	mcpServer.AddTool(listTestsTool, listTestsHandler)
	log.Debug("Registered list_tests tool")

	// Add run test tool
	runTestTool := mcp.NewTool("run_test",
		mcp.WithDescription("Run a specific test by name, optionally multiple times"),
		mcp.WithString("test_name",
			mcp.Required(),
			mcp.Description("Name of the test to run"),
		),
		mcp.WithNumber("count",
			mcp.Description("Number of times to run the test (default: 1)"),
		),
	)
	mcpServer.AddTool(runTestTool, runTestHandler)
	log.Debug("Registered run_test tool")

	// Add cluster info tool
	clusterInfoTool := mcp.NewTool("cluster_info",
		mcp.WithDescription(`Get information about the cluster state and configuration.

Returns detailed information about the OpenShift cluster including platform type, topology,
network configuration, node counts, feature gates, and API groups.

Example JSON output:
{
  "cluster_state": {
    "api_url": "https://api.example.com:6443",
    "platform_status": {
      "type": "AWS",
      "aws": {
        "region": "us-east-1"
      }
    },
    "control_plane_topology": "HighlyAvailable",
    "masters": {
      "items": [...]
    },
    "non_masters": {
      "items": [...]
    },
    "network_spec": {
      "clusterNetwork": [...]
    },
    "version": {
      "status": {
        "desired": {
          "version": "4.15.0"
        }
      }
    }
  },
  "cluster_configuration": {
    "type": "aws",
    "region": "us-east-1",
    "multiMaster": true,
    "multiZone": true,
    "zones": ["us-east-1a", "us-east-1b", "us-east-1c"],
    "numNodes": 3,
    "singleReplicaTopology": false
  }
}`),
	)
	mcpServer.AddTool(clusterInfoTool, clusterInfoHandler)
	log.Debug("Registered cluster_info tool")

	log.Info("All MCP tools registered successfully")

	// Add prompt for test writing guidance
	newTestPrompt := mcp.NewPrompt("new-test",
		mcp.WithPromptDescription("Write and validate a new OpenShift test based on specifications"),
		mcp.WithArgument("specification",
			mcp.ArgumentDescription("Detailed specification of what the test should validate"),
			mcp.RequiredArgument(),
		),
	)
	mcpServer.AddPrompt(newTestPrompt, newTestPromptHandler)
	log.Debug("Registered new-test prompt")

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the server based on the selected mode
	switch o.Mode {
	case "stdio":
		log.Info("Starting stdio MCP server (press Ctrl+C to stop)")
		log.Debug("Stdio server will read from stdin and write to stdout")

		// Run stdio server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("Stdio server panicked")
					errChan <- fmt.Errorf("stdio server panicked: %v", r)
				}
			}()

			log.Debug("Starting stdio server listener")
			err := server.ServeStdio(mcpServer)
			if err != nil {
				log.WithError(err).Error("Stdio server terminated with error")
			} else {
				log.Info("Stdio server terminated normally")
			}
			errChan <- err
		}()

		// Wait for either completion or signal
		select {
		case err := <-errChan:
			if err != nil {
				log.WithError(err).Error("Stdio server failed")
			}
			return err
		case sig := <-sigChan:
			log.WithField("signal", sig).Info("Received signal, shutting down stdio server...")
			return nil
		}

	case "http":
		httpServer := server.NewStreamableHTTPServer(mcpServer)
		log.WithField("address", o.ListenAddress).Info("Starting HTTP MCP server (press Ctrl+C to stop)")
		log.WithField("endpoint", fmt.Sprintf("http://localhost%s/mcp", o.ListenAddress)).Info("MCP server will be available at endpoint")
		log.Info("Server will log all MCP protocol events and connection state changes")

		// Start HTTP server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("HTTP server panicked")
					errChan <- fmt.Errorf("HTTP server panicked: %v", r)
				}
			}()

			log.Debug("Starting HTTP server listener")
			err := httpServer.Start(o.ListenAddress)
			if err != nil {
				log.WithError(err).Error("HTTP server terminated with error")
			} else {
				log.Info("HTTP server terminated normally")
			}
			errChan <- err
		}()

		// Wait for either completion or signal
		select {
		case err := <-errChan:
			if err != nil {
				log.WithError(err).Error("HTTP server failed")
				// Check for common error conditions
				if strings.Contains(err.Error(), "address already in use") {
					log.WithField("address", o.ListenAddress).Error("Port is already in use - another service may be running on this port")
				} else if strings.Contains(err.Error(), "permission denied") {
					log.WithField("address", o.ListenAddress).Error("Permission denied - you may need elevated privileges to bind to this port")
				}
			}
			return err
		case sig := <-sigChan:
			log.WithField("signal", sig).Info("Received signal, shutting down HTTP server...")
			// Gracefully shutdown the HTTP server
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			log.Debug("Initiating graceful shutdown")
			if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
				log.WithError(shutdownErr).Error("Error during graceful shutdown")
			} else {
				log.Info("HTTP server shutdown completed successfully")
			}
			return nil
		}

	default:
		return fmt.Errorf("unsupported mode: %s", o.Mode)
	}
}

func NewMCPCommand() *cobra.Command {
	f := NewMCPFlags()

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run an openshift-tests MCP server",
		Long: templates.LongDesc(`The openshift-tests MCP server allows you to interact with an instance of openshift-tests using your favorite LLM.

		The server can run in two modes:
		- stdio: Communicates via standard input/output (suitable for direct integration with LLM tools)
		- http: Runs as an HTTP server on a specified address and port (suitable for network-based access)`),

		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(command *cobra.Command, args []string) error {
			// Only validate flags, don't require cluster access at startup
			// Cluster access will be checked when tools are actually called
			_, err := f.ToOptions()
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := f.ToOptions()
			if err != nil {
				return errors.WithMessage(err, "error converting to options")
			}

			if err := o.Run(); err != nil {
				return errors.WithMessage(err, "error running MCP server")
			}
			return nil
		},
	}
	f.BindFlags(mcpCmd.Flags())
	return mcpCmd
}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("hello_world tool panicked")
		}
	}()

	// Check if context is already cancelled
	if ctx.Err() != nil {
		log.WithError(ctx.Err()).Warn("hello_world tool called with cancelled context")
		return mcp.NewToolResultError("request cancelled"), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		log.WithError(err).Error("Failed to get name parameter for hello_world tool")
		return mcp.NewToolResultError(err.Error()), nil
	}

	log.WithField("name", name).Info("Executing hello_world tool")
	result := fmt.Sprintf("Hello, %s!", name)
	log.WithField("result", result).Debug("hello_world tool completed successfully")

	return mcp.NewToolResultText(result), nil
}

func listTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Info("Executing list_tests tool")

	// Use exec to call the existing list tests command
	cmdArgs := []string{"list", "tests", "--output", "json"}
	log.WithField("command", fmt.Sprintf("%s %s", os.Args[0], strings.Join(cmdArgs, " "))).Debug("Running list tests command")

	cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)

	// Copy the current environment to preserve auth info
	cmd.Env = os.Environ()

	startTime := time.Now()
	output, err := cmd.Output()
	duration := time.Since(startTime)

	if err != nil {
		log.WithFields(log.Fields{
			"duration": duration,
			"error":    err,
		}).Error("list tests command failed")

		if exitErr, ok := err.(*exec.ExitError); ok {
			log.WithField("stderr", string(exitErr.Stderr)).Error("list tests command stderr")
			return mcp.NewToolResultError(fmt.Sprintf("list tests command failed: %s\nStderr: %s", err, string(exitErr.Stderr))), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to execute list tests command: %v", err)), nil
	}

	log.WithFields(log.Fields{
		"duration":    duration,
		"output_size": len(output),
	}).Info("list tests command completed successfully")

	return mcp.NewToolResultText(string(output)), nil
}

func runTestHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	testName, err := request.RequireString("test_name")
	if err != nil {
		log.WithError(err).Error("Failed to get test_name parameter for run_test tool")
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get the count parameter (optional, defaults to 1)
	count := 1
	countValue := request.GetFloat("count", 1.0)
	if countValue < 1 {
		log.WithField("count", countValue).Error("Invalid count parameter: must be at least 1")
		return mcp.NewToolResultError("count must be at least 1"), nil
	}
	if countValue > 100 {
		log.WithField("count", countValue).Error("Invalid count parameter: cannot exceed 100")
		return mcp.NewToolResultError("count cannot exceed 100 to prevent excessive resource usage"), nil
	}
	count = int(countValue)

	log.WithFields(log.Fields{
		"test_name": testName,
		"count":     count,
	}).Info("Starting run_test tool execution")

	var allResults []map[string]interface{}
	var successCount, failureCount int
	overallStartTime := time.Now()

	// Run the test the specified number of times
	for i := 1; i <= count; i++ {
		runLogger := log.WithFields(log.Fields{
			"test_name":  testName,
			"run_number": i,
			"total_runs": count,
		})

		runLogger.Info("Starting test run")

		// Use exec to call the existing run-test command
		cmdArgs := []string{"run-test", testName}
		runLogger.WithField("command", fmt.Sprintf("%s %s", os.Args[0], strings.Join(cmdArgs, " "))).Debug("Executing test command")

		cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)

		// Copy the current environment to preserve auth info
		cmd.Env = os.Environ()

		runStartTime := time.Now()
		output, err := cmd.Output()
		runDuration := time.Since(runStartTime)

		runResult := map[string]interface{}{
			"run_number": i,
			"test_name":  testName,
			"duration":   runDuration.String(),
		}

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// For test failures, we still want to return the output
				runResult["result"] = "failed"
				runResult["output"] = string(output)
				runResult["error"] = string(exitErr.Stderr)
				failureCount++

				runLogger.WithFields(log.Fields{
					"duration": runDuration,
					"stderr":   string(exitErr.Stderr),
				}).Warn("Test run failed")
			} else {
				runResult["result"] = "error"
				runResult["error"] = fmt.Sprintf("failed to execute run-test command: %v", err)
				failureCount++

				runLogger.WithFields(log.Fields{
					"duration": runDuration,
					"error":    err,
				}).Error("Test run encountered execution error")
			}
		} else {
			// For successful tests
			runResult["result"] = "passed"
			runResult["output"] = string(output)
			successCount++

			runLogger.WithField("duration", runDuration).Info("Test run completed successfully")
		}

		allResults = append(allResults, runResult)

		// If running multiple times, add a small delay between runs
		if count > 1 && i < count {
			runLogger.Debug("Waiting 1 second before next test run")
			select {
			case <-ctx.Done():
				log.WithField("completed_runs", i).Warn("Test execution cancelled by context")
				return mcp.NewToolResultError("test execution cancelled"), nil
			case <-time.After(1 * time.Second):
				// Continue to next iteration
			}
		}
	}

	overallDuration := time.Since(overallStartTime)

	// Determine overall result
	overallResult := func() string {
		if failureCount == 0 {
			return "all_passed"
		} else if successCount == 0 {
			return "all_failed"
		} else {
			return "mixed"
		}
	}()

	// Log comprehensive summary
	log.WithFields(log.Fields{
		"test_name":      testName,
		"total_runs":     count,
		"success_count":  successCount,
		"failure_count":  failureCount,
		"overall_result": overallResult,
		"total_duration": overallDuration,
	}).Info("run_test tool execution completed")

	// Create summary result
	summaryData := map[string]interface{}{
		"test_name":      testName,
		"total_runs":     count,
		"success_count":  successCount,
		"failure_count":  failureCount,
		"overall_result": overallResult,
		"total_duration": overallDuration.String(),
		"runs":           allResults,
	}

	jsonData, err := json.MarshalIndent(summaryData, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal test result to JSON")
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal test result to JSON: %v", err)), nil
	}

	responseSize := len(jsonData)
	log.WithFields(log.Fields{
		"response_size":    responseSize,
		"response_size_mb": float64(responseSize) / (1024 * 1024),
	}).Debug("Returning test execution results")

	// Warn if response is very large (might cause connection issues)
	if responseSize > 1024*1024 { // 1MB
		log.WithField("response_size_mb", float64(responseSize)/(1024*1024)).Warn("Large response size - this might cause connection issues with some MCP clients")
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func clusterInfoHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("cluster_info tool panicked")
		}
	}()

	log.Info("Executing cluster_info tool")

	// Check if context is already cancelled
	if ctx.Err() != nil {
		log.WithError(ctx.Err()).Warn("cluster_info tool called with cancelled context")
		return mcp.NewToolResultError("request cancelled"), nil
	}

	startTime := time.Now()

	// Load cluster configuration
	clientConfig, err := e2e.LoadConfig(true)
	if err != nil {
		log.WithError(err).Error("Failed to load cluster configuration")
		return mcp.NewToolResultError(fmt.Sprintf("failed to load cluster configuration: %v", err)), nil
	}

	// Discover cluster state
	clusterState, err := clusterdiscovery.DiscoverClusterState(clientConfig)
	if err != nil {
		log.WithError(err).Error("Failed to discover cluster state")
		return mcp.NewToolResultError(fmt.Sprintf("failed to discover cluster state: %v", err)), nil
	}

	// Generate cluster configuration from state
	clusterConfig, err := clusterdiscovery.LoadConfig(clusterState)
	if err != nil {
		log.WithError(err).Error("Failed to load cluster configuration from state")
		return mcp.NewToolResultError(fmt.Sprintf("failed to load cluster configuration from state: %v", err)), nil
	}

	// Create response data structure
	responseData := map[string]interface{}{
		"cluster_state":         clusterState,
		"cluster_configuration": clusterConfig,
		"timestamp":             startTime.Format(time.RFC3339),
	}

	jsonData, err := json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal cluster info to JSON")
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal cluster info to JSON: %v", err)), nil
	}

	duration := time.Since(startTime)
	responseSize := len(jsonData)
	log.WithFields(log.Fields{
		"duration":         duration,
		"response_size":    responseSize,
		"response_size_mb": float64(responseSize) / (1024 * 1024),
	}).Info("cluster_info tool completed successfully")

	// Warn if response is very large (might cause connection issues)
	if responseSize > 1024*1024 { // 1MB
		log.WithField("response_size_mb", float64(responseSize)/(1024*1024)).Warn("Large response size - this might cause connection issues with some MCP clients")
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func newTestPromptHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("new-test prompt panicked")
		}
	}()

	log.Info("Executing new-test prompt")

	// Check if context is already cancelled
	if ctx.Err() != nil {
		log.WithError(ctx.Err()).Warn("new-test prompt called with cancelled context")
		return nil, fmt.Errorf("request cancelled")
	}

	// Get the test specification from arguments
	specification := request.Params.Arguments["specification"]
	if specification == "" {
		return nil, fmt.Errorf("specification argument is required")
	}

	promptContent := fmt.Sprintf(`# Write and Validate OpenShift Test

You are tasked with writing a new test for the OpenShift test suite based on the following specification:

**Test Specification:**
%s

## Your Task
1. **Write the test** following the guidelines below
2. **Validate the test** by running it multiple times to ensure reliability

## Test Framework Guidelines
- OpenShift-tests uses **Ginkgo** as its testing framework
- Tests are organized in a BDD (Behavior-Driven Development) style with Describe/Context/It blocks
- All tests should follow Ginkgo patterns and conventions
- All tests should go into the test/extended directory
- If you create a new package, you should make sure its imported into test/extended/include.go

## Critical Test Naming Requirements
- **CRITICAL**: Test names must be stable and deterministic
- **NEVER** include dynamic information in test titles such as:
  - Pod names (e.g., "test-pod-abc123")
  - Timestamps
  - Random UUIDs or generated identifiers
  - Node names
  - Namespace names with random suffixes
- **ALWAYS** use descriptive, static names that clearly indicate what the test validates
- Good example: "should create a pod with custom security context"
- Bad example: "should create pod test-pod-xyz123 with custom security context"

## Test Validation Requirements
After writing your test, you **MUST** re-build the openshift-tests binary (make openshift-tests) and then validate it
for reliability by using your MCP tools.

1. **Initial Test**: Use the "run_test" tool to run your test once to verify basic functionality
2. **Reliability Testing**: Run the test **minimum 5 times, recommended 10 times** to check for flakiness
3. **NEVER** invoke ginkgo directly - always use the provided "run_test" tool

### Validation Commands:
- Single run: run_test(test_name="your test name here")
- Reliability check: run_test(test_name="your test name", count=10)

## Test Structure Guidelines
- Tests should be focused and test one specific behavior
- Use proper setup and cleanup in BeforeEach/AfterEach blocks
- Include appropriate timeouts for operations
- Add meaningful assertions with clear failure messages
- Follow existing patterns in the codebase for consistency

## Success Criteria
Your test is ready when:
- ✅ It has a stable, descriptive name
- ✅ It passes a single run
- ✅ It passes at least 5 consecutive runs (preferably 10)
- ✅ It follows Ginkgo best practices
- ✅ It properly validates the specified behavior

Now write the test and validate it according to these requirements.`, specification)

	return mcp.NewGetPromptResult(
		"Write and Validate OpenShift Test",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(promptContent),
			),
		},
	), nil
}
