# Using openshift-tests' MCP Server

The openshift-tests MCP ([Model Context Protocol](https://modelcontextprotocol.io/)) server provides AI assistants with direct access to
openshift-tests. It exposes tools for listing, running, and creating new conformance tests.

## What are the prerequisites?

* An MCP client, such as Cursor
* An OpenShift cluster to test (perhaps from Cluster Bot)
* A locally built copy of `openshift-tests`

## What can it do?

The MCP server acts as a bridge between AI assistants (like Claude in Cursor) and the openshift-tests binary. It provides
structured tools and prompts.

### Creating a New Test

openshift-tests MCP includes a prompt that can help Cursor write a new test according to our best practices.  Prompts
are invoked in AI assistants typically with a `/` command. 

To create a new integration test, invoke `/new-test` and fill in the form. In cursor, you have to press `<enter>`
twice after completing the specification for your new test.  An example spec would look something like this:

```
Create a test that does the following:
 * Apply a manifest that runs a busybox pod with the command echo success && sleep 5.
 * Wait for the pod to complete (kubectl wait --for=condition=Succeeded).
 * Retrieve the pod logs.
 * Assert that the logs contain the string success.
 * Clean up the pod afterward.
```

The prompt guides Cursor on how to create and validating the test, including running your new test 10x to
verify its stability.

### Working with existing tests

The MCP server can list tests, for example, you can ask: "What tests are available for machine-config-operator?". 
Or, "Can you run the test named `<X>` 10 times to verify its reliability?"

### Gathering cluster information

`openshift-tests` is aware of a wealth of information about the cluster under test.  You can ask "Please tell me
about the cluster being tested" or more specific questions like, "What network stack is the cluster using?"

## How do I run the MCP server?

openshift-tests MCP supports both stdio and HTTP access. When developing with Cursor, you should
use stdio as the LLM will need to rebuild the openshift-tests binary to run any new tests you create.

### Configuring Cursor

In Cursor, go to `Settings -> Cursor Settings -> Tools & Integrations` and point it to your
locally built copy of openshift-tests.

```json
{
  "mcpServers": {
        "openshift-tests local": {
          "command": "/home/stbenjam/go/src/github.com/openshift/origin/openshift-tests",
          "args": ["mcp", "--mode", "stdio"],
          "env": {
            "TEST_PROVIDER": "{\"type\":\"aws\"}",
            "KUBECONFIG": "/home/stbenjam/Downloads/cluster-bot-2025-08-07-120418.kubeconfig",
            "EXTENSIONS_PAYLOAD_OVERRIDE": "registry.ci.openshift.org/ocp/release:4.20.0-0.nightly-2025-07-31-06312",
            "EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS": "tests"
          }
        }
  }
}
```

### Command Line Usage

```bash
# Start in stdio mode
./openshift-tests mcp --mode stdio

# Start HTTP server on default port
./openshift-tests mcp --mode http

# Start HTTP server on custom port
./openshift-tests mcp --mode http --listen-address ":9090"
```

### Environment Configuration

The MCP server needs the same environment variables as openshift-tests:

- `KUBECONFIG`: Path to cluster kubeconfig file
- `TEST_PROVIDER`: JSON describing the cloud provider (e.g., `{"type":"aws"}`)
- `EXTENSIONS_PAYLOAD_OVERRIDE`: Use a specific payload, required for some cluster-bot MCE clusters whose payload is reaped after install.
- `EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS`: Only opt-in to specific extension binaries, an optimization for running locally.