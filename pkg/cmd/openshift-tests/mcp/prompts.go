package mcp

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerPrompts registers all MCP prompts with the server
func registerPrompts(mcpServer *server.MCPServer) {
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
}

// newTestPromptHandler handles the new-test prompt
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

## Origin Guidelines
If you are working in the "origin" code repository, then:
- All tests should go into the test/extended directory
- If you create a new package, you should make sure its imported into test/extended/include.go
- After writing your test, you **MUST** re-build the openshift-tests binary (make openshift-tests) and then validate it
for reliability by using your MCP tools.

## Extension Guidelines
Some tests exist outside of the origin repo, but are still orchestrated by it (these are called extensions). If you
are adding a test to the extension, make sure to rebuild the extension binary using the relevant make target.

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
