package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
)

// registerTools registers all MCP tools with the server
func registerTools(mcpServer *server.MCPServer) {
	// Add list tests tool
	listTestsTool := mcp.NewTool("list_tests",
		mcp.WithDescription("List all available tests in JSON format. Supports optional regex filtering of test names (case-insensitive)."),
		mcp.WithString("filter",
			mcp.Description("Optional regex pattern to filter test names (case-insensitive)"),
		),
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
}

// listTestsHandler handles the list_tests tool
func listTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get the filter parameter (optional)
	filterPattern := request.GetString("filter", "")

	logFields := log.Fields{}
	if filterPattern != "" {
		logFields["filter"] = filterPattern
	}
	log.WithFields(logFields).Info("Executing list_tests tool")

	// Validate regex pattern if provided
	var filterRegex *regexp.Regexp
	if filterPattern != "" {
		var err error
		// Make the regex case-insensitive by default
		caseInsensitivePattern := "(?i)" + filterPattern
		filterRegex, err = regexp.Compile(caseInsensitivePattern)
		if err != nil {
			log.WithError(err).WithField("pattern", filterPattern).Error("Invalid regex pattern")
			return mcp.NewToolResultError(fmt.Sprintf("invalid regex pattern '%s': %v", filterPattern, err)), nil
		}
	}

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

	// Extract JSON from output (skip log lines)
	jsonOutput := extractJSONFromOutput(output)

	// If no filter is specified, return the original JSON output
	if filterPattern == "" {
		return mcp.NewToolResultText(string(jsonOutput)), nil
	}

	// Parse the JSON output and filter the tests
	var tests []map[string]interface{}
	if err := json.Unmarshal(jsonOutput, &tests); err != nil {
		log.WithError(err).Error("Failed to parse test list JSON")
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse test list JSON: %v", err)), nil
	}

	// Filter tests based on the regex pattern
	var filteredTests []map[string]interface{}
	originalCount := len(tests)

	for _, test := range tests {
		if name, ok := test["name"].(string); ok {
			if filterRegex.MatchString(name) {
				filteredTests = append(filteredTests, test)
			}
		}
	}

	filteredCount := len(filteredTests)
	log.WithFields(log.Fields{
		"original_count": originalCount,
		"filtered_count": filteredCount,
		"filter":         filterPattern,
	}).Info("Applied filter to test list")

	// Marshal the filtered results back to JSON
	filteredOutput, err := json.Marshal(filteredTests)
	if err != nil {
		log.WithError(err).Error("Failed to marshal filtered test list")
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal filtered test list: %v", err)), nil
	}

	return mcp.NewToolResultText(string(filteredOutput)), nil
}

// extractJSONFromOutput extracts the JSON content from command output,
// skipping any log lines that appear before the JSON array
func extractJSONFromOutput(output []byte) []byte {
	lines := strings.Split(string(output), "\n")

	// Find the first line that starts with '['
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			// Join all lines from this point onwards
			jsonLines := lines[i:]
			return []byte(strings.Join(jsonLines, "\n"))
		}
	}

	// If no JSON array found, return original output
	return output
}

// runTestHandler handles the run_test tool
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

// clusterInfoHandler handles the cluster_info tool
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
