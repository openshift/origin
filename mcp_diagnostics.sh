#!/bin/bash

# MCP Server Diagnostics Script
# This script helps diagnose "Channel closed" and other MCP server issues

set -e

PORT=${1:-8080}
HOST=${2:-localhost}

echo "🔍 MCP Server Diagnostics"
echo "========================="
echo "Testing server at: $HOST:$PORT"
echo ""

# Function to test if server is reachable
test_connectivity() {
    echo "📡 Testing basic connectivity..."
    if timeout 5 bash -c "</dev/tcp/$HOST/$PORT" 2>/dev/null; then
        echo "✅ Server is accepting connections on $HOST:$PORT"
        return 0
    else
        echo "❌ Cannot connect to server on $HOST:$PORT"
        return 1
    fi
}

# Function to test health check
test_health_check() {
    echo ""
    echo "🏥 Testing health check tool..."
    
    local response
    response=$(curl -s -w "\n%{http_code}" -X POST "http://$HOST:$PORT/mcp" \
        -H "Content-Type: application/json" \
        -d '{
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tools/call",
            "params": {
                "name": "health_check",
                "arguments": {}
            }
        }' 2>/dev/null || echo "CURL_FAILED")
    
    if [[ "$response" == "CURL_FAILED" ]]; then
        echo "❌ Health check failed - curl command failed"
        return 1
    fi
    
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)
    
    if [[ "$http_code" == "200" ]]; then
        echo "✅ Health check successful (HTTP $http_code)"
        echo "📋 Health check response:"
        echo "$body" | jq . 2>/dev/null || echo "$body"
        return 0
    else
        echo "❌ Health check failed (HTTP $http_code)"
        echo "Response: $body"
        return 1
    fi
}

# Function to test hello world tool
test_hello_world() {
    echo ""
    echo "👋 Testing hello_world tool..."
    
    local response
    response=$(curl -s -w "\n%{http_code}" -X POST "http://$HOST:$PORT/mcp" \
        -H "Content-Type: application/json" \
        -d '{
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "hello_world",
                "arguments": {"name": "diagnostics"}
            }
        }' 2>/dev/null || echo "CURL_FAILED")
    
    if [[ "$response" == "CURL_FAILED" ]]; then
        echo "❌ Hello world test failed - curl command failed"
        return 1
    fi
    
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)
    
    if [[ "$http_code" == "200" ]] && echo "$body" | grep -q "Hello, diagnostics"; then
        echo "✅ Hello world tool working correctly"
        return 0
    else
        echo "❌ Hello world tool failed (HTTP $http_code)"
        echo "Response: $body"
        return 1
    fi
}

# Function to check server process
check_server_process() {
    echo ""
    echo "🔍 Checking server process..."
    
    local pids
    pids=$(pgrep -f "openshift-tests mcp" 2>/dev/null || true)
    
    if [[ -n "$pids" ]]; then
        echo "✅ MCP server process found:"
        ps -p $pids -o pid,ppid,cmd,etime,pcpu,pmem 2>/dev/null || true
    else
        echo "❌ No MCP server process found"
        echo "💡 Start the server with: openshift-tests mcp --mode http --listen-address :$PORT"
    fi
}

# Function to check port usage
check_port_usage() {
    echo ""
    echo "🔌 Checking port usage..."
    
    local port_info
    port_info=$(lsof -i :$PORT 2>/dev/null || true)
    
    if [[ -n "$port_info" ]]; then
        echo "✅ Port $PORT is in use:"
        echo "$port_info"
    else
        echo "❌ Port $PORT is not in use"
        echo "💡 Make sure the server is running and listening on port $PORT"
    fi
}

# Function to test tools list
test_tools_list() {
    echo ""
    echo "🛠️  Testing tools list..."
    
    local response
    response=$(curl -s -w "\n%{http_code}" -X POST "http://$HOST:$PORT/mcp" \
        -H "Content-Type: application/json" \
        -d '{
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/list",
            "params": {}
        }' 2>/dev/null || echo "CURL_FAILED")
    
    if [[ "$response" == "CURL_FAILED" ]]; then
        echo "❌ Tools list failed - curl command failed"
        return 1
    fi
    
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)
    
    if [[ "$http_code" == "200" ]]; then
        echo "✅ Tools list retrieved successfully"
        echo "📋 Available tools:"
        echo "$body" | jq '.result.tools[].name' 2>/dev/null || echo "$body"
        return 0
    else
        echo "❌ Tools list failed (HTTP $http_code)"
        echo "Response: $body"
        return 1
    fi
}

# Main diagnostic flow
main() {
    echo "Starting diagnostics for MCP server..."
    echo ""
    
    # Check if required tools are available
    if ! command -v curl &> /dev/null; then
        echo "❌ curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo "⚠️  jq not found - JSON output will not be formatted"
    fi
    
    # Run diagnostics
    check_server_process
    check_port_usage
    
    if test_connectivity; then
        test_health_check
        test_hello_world
        test_tools_list
    else
        echo ""
        echo "🚨 Server connectivity failed. Possible causes:"
        echo "   1. Server is not running"
        echo "   2. Server is running on a different port"
        echo "   3. Firewall is blocking the connection"
        echo "   4. Server crashed or is unresponsive"
        echo ""
        echo "💡 Try starting the server with:"
        echo "   openshift-tests mcp --mode http --listen-address :$PORT -v 5"
    fi
    
    echo ""
    echo "🏁 Diagnostics complete!"
}

# Show usage if help requested
if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    echo "Usage: $0 [PORT] [HOST]"
    echo ""
    echo "Diagnose MCP server connectivity and functionality"
    echo ""
    echo "Arguments:"
    echo "  PORT    Server port (default: 8080)"
    echo "  HOST    Server host (default: localhost)"
    echo ""
    echo "Examples:"
    echo "  $0                    # Test localhost:8080"
    echo "  $0 9999               # Test localhost:9999"
    echo "  $0 8080 example.com   # Test example.com:8080"
    exit 0
fi

main
