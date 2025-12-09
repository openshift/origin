#!/bin/bash
# Systemd Slice CPU Stress Test - Stresses multiple slices on the host
# This uses 'oc debug node' to run systemd-run commands directly on the node

set -e

# Configuration
TARGET_NODE="${1:-}"
STRESS_DURATION="${2:-300}"  # 5 minutes default
SLICE_SPECS="${3:-system.slice:4}"  # Comma-separated list of slice:cores pairs
MONITOR_INTERVAL=5

# Parse slice specifications into arrays
IFS=',' read -ra SLICE_ARRAY <<< "$SLICE_SPECS"
SLICE_NAMES=()
SLICE_CORES=()

for spec in "${SLICE_ARRAY[@]}"; do
    IFS=':' read -r slice cores <<< "$spec"
    slice=$(echo "$slice" | xargs)  # Trim whitespace
    cores=$(echo "$cores" | xargs)

    if [ -z "$slice" ] || [ -z "$cores" ]; then
        echo "Error: Invalid slice specification: '$spec'. Expected format: 'slice:cores'"
        exit 1
    fi

    SLICE_NAMES+=("$slice")
    SLICE_CORES+=("$cores")
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Function to cleanup on exit
cleanup() {
    log_info "Cleaning up stress processes..."

    # Stop all stress test units on the node
    i=0
    while [ $i -lt ${#SLICE_NAMES[@]} ]; do
        slice="${SLICE_NAMES[$i]}"
        cores="${SLICE_CORES[$i]}"
        slice_safe=$(echo "$slice" | sed 's/[.\/]/-/g')

        log_info "Stopping processes for $slice..."
        for j in $(seq 1 $cores); do
            oc debug node/${TARGET_NODE} -- chroot /host systemctl stop stress-test-${slice_safe}-${j} 2>/dev/null || true
        done
        i=$((i+1))
    done

    if [ ! -z "$MONITOR_PID" ]; then
        kill $MONITOR_PID 2>/dev/null || true
    fi
    log_info "Cleanup complete"
}

trap cleanup INT TERM

# Validate node name provided
if [ -z "$TARGET_NODE" ]; then
    log_error "Usage: $0 <node-name> [duration-seconds] [slice-specs]"
    echo ""
    echo "Arguments:"
    echo "  node-name       : Name of the node to stress (required)"
    echo "  duration-seconds: Duration of stress test in seconds (default: 300)"
    echo "  slice-specs     : Comma-separated list of 'slice:cores' pairs (default: system.slice:4)"
    echo ""
    echo "Examples:"
    echo "  $0 worker-1 300 system.slice:4"
    echo "  $0 worker-1 300 'system.slice:4,user.slice:2'"
    echo "  $0 worker-1 600 'system.slice:4,user.slice:2,kubepods.slice:8'"
    echo ""
    echo "Available nodes:"
    kubectl get nodes -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[-1].type,CPU:.status.capacity.cpu
    exit 1
fi

# Validate node exists
if ! kubectl get node "$TARGET_NODE" &>/dev/null; then
    log_error "Node '$TARGET_NODE' not found"
    exit 1
fi

log_info "========================================="
log_info "Systemd Slice CPU Stress Test (Host-level)"
log_info "========================================="
log_info "Target Node: $TARGET_NODE"
log_info "Duration: ${STRESS_DURATION}s"
log_info "Target Slices:"
i=0
while [ $i -lt ${#SLICE_NAMES[@]} ]; do
    slice="${SLICE_NAMES[$i]}"
    cores="${SLICE_CORES[$i]}"
    log_info "  - $slice: $cores cores"
    i=$((i+1))
done
log_info "========================================="

# Get node information
log_info "Gathering node information..."
NODE_INFO=$(kubectl get node "$TARGET_NODE" -o json)
CAPACITY_CPU=$(echo "$NODE_INFO" | jq -r '.status.capacity.cpu')
ALLOCATABLE_CPU=$(echo "$NODE_INFO" | jq -r '.status.allocatable.cpu')

# Convert to millicores for calculation if they contain 'm'
if [[ "$CAPACITY_CPU" == *m ]]; then
    CAPACITY_MILLI=${CAPACITY_CPU%m}
else
    CAPACITY_MILLI=$((CAPACITY_CPU * 1000))
fi

if [[ "$ALLOCATABLE_CPU" == *m ]]; then
    ALLOCATABLE_MILLI=${ALLOCATABLE_CPU%m}
else
    ALLOCATABLE_MILLI=$((ALLOCATABLE_CPU * 1000))
fi

SYSTEM_RESERVED_MILLI=$((CAPACITY_MILLI - ALLOCATABLE_MILLI))
SYSTEM_RESERVED=$(awk "BEGIN {printf \"%.2f\", $SYSTEM_RESERVED_MILLI/1000}")

log_info "Node CPU Info:"
echo "  Total Capacity: ${CAPACITY_CPU} cores"
echo "  Allocatable: ${ALLOCATABLE_CPU} cores"
echo "  System Reserved: ${SYSTEM_RESERVED} cores (${SYSTEM_RESERVED_MILLI}m)"

# Check if node has any taints
TAINTS=$(kubectl get node "$TARGET_NODE" -o jsonpath='{.spec.taints}')
if [ ! -z "$TAINTS" ] && [ "$TAINTS" != "null" ]; then
    log_warn "Node has taints: $TAINTS"
fi

# Function to monitor node and pod status
monitor_status() {
    local output_file="stress_test_log_${TARGET_NODE}_$(date +%Y%m%d_%H%M%S).log"
    log_info "Monitoring output will be saved to: $output_file"

    echo "=== Systemd Slice Stress Test Monitor Log ===" > "$output_file"
    echo "Start Time: $(date)" >> "$output_file"
    echo "Target Node: $TARGET_NODE" >> "$output_file"
    echo "Target Slices:" >> "$output_file"
    i=0
    while [ $i -lt ${#SLICE_NAMES[@]} ]; do
        slice="${SLICE_NAMES[$i]}"
        cores="${SLICE_CORES[$i]}"
        echo "  - $slice: $cores cores" >> "$output_file"
        i=$((i+1))
    done
    echo "" >> "$output_file"
    
    while true; do
        {
            echo "==================== $(date) ===================="
            
#             # Node status
#             echo ""
#             echo "--- Node Status ---"
#             kubectl get node "$TARGET_NODE" -o custom-columns=\
# NAME:.metadata.name,\
# STATUS:.status.conditions[-1].type,\
# READY:.status.conditions[-1].status,\
# CPU:.status.capacity.cpu,\
# MEMORY:.status.capacity.memory
            
            # # Node conditions
            # echo ""
            # echo "--- Node Conditions ---"
            # kubectl get node "$TARGET_NODE" -o json | jq -r '.status.conditions[] | "\(.type): \(.status) - \(.message // "N/A")"'
            
            # Check if metrics-server is available
            if kubectl top node "$TARGET_NODE" &>/dev/null; then
                echo ""
                echo "--- Node Resource Usage ---"
                kubectl top node "$TARGET_NODE"
            fi
            
            # Pods on the node
            echo ""
            echo "--- Pods on Node (Non-Running, excluding Completed) ---"
            kubectl get pods -A --field-selector spec.nodeName="$TARGET_NODE" 2>/dev/null | \
                grep -v -E "(Running|Completed|STATUS|installer-)" || echo "All pods running or none found"
            
            # # Recent events
            # echo ""
            # echo "--- Recent Node Events (last 2 minutes) ---"
            # kubectl get events -A --field-selector involvedObject.name="$TARGET_NODE" \
            #     --sort-by='.lastTimestamp' 2>/dev/null | tail -10 || echo "No recent events"

            echo ""
            echo "--- Stress test processes are running ---"
            oc debug node/${TARGET_NODE} -- bash -c "chroot /host systemctl --type=service --state=running | grep stress-test" || echo "No stress test processes are running"
            
        } | tee -a "$output_file"
        
        sleep $MONITOR_INTERVAL
    done
}

# Start stress processes directly on the node using oc debug
log_info "Starting stress test on node: $TARGET_NODE"
log_warn "Using 'oc debug node' to run systemd-run commands directly on the host"

echo "========================================="
echo "Starting multi-slice stress test"

# Create stress processes in each specified slice using systemd-run
echo ""
echo "Launching stress processes..."
i=0
while [ $i -lt ${#SLICE_NAMES[@]} ]; do
    slice="${SLICE_NAMES[$i]}"
    cores="${SLICE_CORES[$i]}"
    # Sanitize slice name for unit naming (replace dots and slashes)
    slice_safe=$(echo "$slice" | sed 's/[.\/]/-/g')

    echo ""
    echo "Starting $cores processes in $slice..."
    for j in $(seq 1 $cores); do
        oc debug node/${TARGET_NODE} -- chroot /host systemd-run \
          --unit=stress-test-${slice_safe}-${j} \
          --slice=$slice \
          --description="CPU Stress Test for $slice Process $j" \
          bash -c 'while true; do :; done' &
        echo "  Started process $j in $slice"
    done
    i=$((i+1))
done

sleep 5

# Show that processes are running in the specified slices
echo ""
echo "Verifying processes are running:"
oc debug node/${TARGET_NODE} -- bash -c "chroot /host systemctl --type=service --state=running | grep stress-test" || echo "No stress test processes are running"

echo ""
echo "CPU-intensive processes running. Will run for $STRESS_DURATION seconds..."
echo ""

log_info ""
log_info "========================================="
log_info "Stress test running for ${STRESS_DURATION} seconds"
log_info "Monitor the output above for node behavior"
log_info "Stress processes are running in the following slices on the HOST:"
i=0
while [ $i -lt ${#SLICE_NAMES[@]} ]; do
    slice="${SLICE_NAMES[$i]}"
    cores="${SLICE_CORES[$i]}"
    log_info "  - $slice: $cores cores"
    i=$((i+1))
done
log_info "========================================="
log_info ""

# Start monitoring in background
log_info "Starting background monitoring..."
monitor_status &
MONITOR_PID=$!

# Monitor for the duration
START_TIME=$(date +%s)
END_TIME=$((START_TIME + STRESS_DURATION))
while [ $(date +%s) -lt $END_TIME ]; do
    CURRENT_TIME=$(date +%s)
    REMAINING=$((END_TIME - CURRENT_TIME))
    echo "[Time remaining]: ${REMAINING}s"
    echo ""
    sleep 10
done

# Cleanup 
cleanup

# Wait a bit and show final status
sleep 5
echo ""
echo "Slice status AFTER stress:"
oc debug node/${TARGET_NODE} -- bash -c "chroot /host systemctl --type=service --state=running | grep stress-test" || echo "No stress test processes are running"

echo ""
echo "========================================="
echo "Stress test completed!"
echo "========================================="

log_info ""
log_info "========================================="
log_info "Stress test completed!"
log_info "========================================="

# Final status check
log_info "Checking final node status..."
sleep 5

kubectl get node "$TARGET_NODE" -o wide

log_info ""
log_info "Checking for any evicted or failed pods..."
kubectl get pods -A --field-selector spec.nodeName="$TARGET_NODE" | grep -E "(Evicted|Failed|Error)" || log_info "No evicted/failed pods found"

log_info ""
log_info "Recent events on node:"
kubectl get events -A --field-selector involvedObject.name="$TARGET_NODE" --sort-by='.lastTimestamp' | tail -20

log_info ""
log_info "========================================="
log_info "Test Summary"
log_info "========================================="
log_info "Node: $TARGET_NODE"
log_info "Stress Duration: ${STRESS_DURATION}s"
log_info "Slices stressed:"
total_cores=0
i=0
while [ $i -lt ${#SLICE_NAMES[@]} ]; do
    slice="${SLICE_NAMES[$i]}"
    cores="${SLICE_CORES[$i]}"
    log_info "  - $slice: $cores cores"
    total_cores=$((total_cores + cores))
    i=$((i+1))
done
log_info "Total CPU cores stressed: $total_cores"
log_info "Log file: stress_test_log_${TARGET_NODE}_*.log"
log_info ""
log_info "  chroot /host journalctl -u stress-test-* --since '10 minutes ago'"
log_info ""
log_info "Prometheus queries to check (example for first slice):"
if [ ${#SLICE_NAMES[@]} -gt 0 ]; then
    first_slice="${SLICE_NAMES[0]}"
    log_info "  rate(container_cpu_usage_seconds_total{id=\"/$first_slice\", node=\"$TARGET_NODE\"}[1m])* 1000"
 #   log_info "  rate(container_cpu_cfs_throttled_seconds_total{id=\"/$first_slice\"}[1m])"
fi
log_info "========================================="
