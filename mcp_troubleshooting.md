# MCP Server Troubleshooting Guide

## "Channel closed" Error Diagnosis

When you see a "Channel closed" error, it typically means the MCP server connection was interrupted. Here's how to diagnose and fix it:

### 1. Check Server Status

First, verify the server is running:

```bash
# Check if the server process is running
ps aux | grep "openshift-tests mcp"

# Check if the port is in use (for HTTP mode)
lsof -i :8080  # Replace 8080 with your port
```

### 2. Test Server Health

Use the new health check tool to verify server functionality:

```bash
# For HTTP mode - use curl to test the health endpoint
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "health_check",
      "arguments": {}
    }
  }'
```

### 3. Start Server with Enhanced Logging

Run the server with verbose logging to see what's happening:

```bash
# HTTP mode with detailed logging
openshift-tests mcp --mode http --listen-address :8080 -v 5

# Stdio mode with detailed logging  
openshift-tests mcp --mode stdio -v 5
```

### 4. Common Issues and Solutions

#### Port Already in Use
```
ERROR[0000] HTTP server failed error="listen tcp :8080: bind: address already in use"
```
**Solution**: Use a different port or kill the process using the port:
```bash
lsof -ti:8080 | xargs kill -9
# or use a different port
openshift-tests mcp --mode http --listen-address :9999
```

#### Permission Denied
```
ERROR[0000] HTTP server failed error="listen tcp :80: bind: permission denied"
```
**Solution**: Use a port > 1024 or run with elevated privileges:
```bash
# Use unprivileged port
openshift-tests mcp --mode http --listen-address :8080
```

#### Context Cancelled
```
WARN[0000] hello_world tool called with cancelled context
```
**Solution**: The client cancelled the request. Check client timeout settings.

### 5. Connection Testing

Test basic connectivity:

```bash
# Test if server accepts connections (HTTP mode)
telnet localhost 8080

# Test with a simple tool call
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0", 
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "hello_world",
      "arguments": {"name": "test"}
    }
  }'
```

### 6. Log Analysis

Look for these patterns in the logs:

- **Server startup**: `INFO[0000] Starting HTTP MCP server on :8080`
- **Tool registration**: `INFO[0000] All MCP tools registered successfully`
- **Connection issues**: `ERROR[0000] HTTP server failed`
- **Panic recovery**: `ERROR[0000] HTTP server panicked`

### 7. Recovery Steps

If the server becomes unresponsive:

1. **Graceful restart**: Send SIGINT (Ctrl+C) and restart
2. **Force kill**: `pkill -f "openshift-tests mcp"`
3. **Check resources**: `top` or `htop` to see if server is consuming excessive resources
4. **Clear port**: `lsof -ti:PORT | xargs kill -9`

### 8. Client-Side Debugging

If using a client library, enable debug logging and check:
- Connection timeouts
- Request/response format
- Network connectivity
- Authentication if required

### 9. Environment Issues

Check these environment factors:
- **KUBECONFIG**: Required for test-related tools
- **Network**: Firewall or proxy blocking connections
- **Resources**: Sufficient memory and CPU
- **Permissions**: File system and network permissions
