#!/bin/bash
# OpenShift Node Log Collector Service
# This service starts early in the boot process and provides HTTP access to systemd logs
# even when kubelet is down.

PORT=9333
LOG_LINES=50

# Function to get the last N lines of a systemd service
get_service_logs() {
    local service=$1
    local lines=${2:-$LOG_LINES}

    # Use journalctl to get logs, with fallback to "not found" if service doesn't exist
    journalctl -u "${service}.service" --no-pager -n "$lines" 2>/dev/null || \
        echo "Service ${service} not found or no logs available"
}

# Simple HTTP server using netcat
handle_request() {
    local request
    read -r request

    # Parse the request path
    local path=$(echo "$request" | awk '{print $2}')

    case "$path" in
        /logs/kubelet)
            echo -ne "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"
            get_service_logs "kubelet"
            ;;
        /logs/crio)
            echo -ne "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"
            get_service_logs "crio"
            ;;
        /logs/both)
            echo -ne "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"
            echo "=== KUBELET LOGS ==="
            get_service_logs "kubelet"
            echo ""
            echo "=== CRIO LOGS ==="
            get_service_logs "crio"
            ;;
        /health)
            echo -ne "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"
            echo "OK"
            ;;
        *)
            echo -ne "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\n\r\n"
            echo "Available endpoints: /logs/kubelet, /logs/crio, /logs/both, /health"
            ;;
    esac
}

# Main server loop
echo "Starting OpenShift Node Log Collector Service on port $PORT"
while true; do
    handle_request | nc -l -p $PORT -q 1
done
