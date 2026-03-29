#!/bin/bash
# Systemd Slice CPU Stress Test - Stresses multiple slices on the host
# This uses 'oc debug node' to run systemd-run commands directly on the node

set -e

# Configuration
NON_PROMETHEUS_NODE=$(oc get nodes --no-headers | grep worker | awk '{print $1}' | grep -v -f <(oc get po -n openshift-monitoring -o wide --no-headers | grep prometheus-k8s | awk '{print $7}') | head -n 1)
TARGET_NODE="${NON_PROMETHEUS_NODE}"
STRESS_DURATION="300"  # 5 minutes default
SLICE_SPECS="system.slice:4,kubepods.slice:7"  # Comma-separated list of slice:cores pairs
SYSTEM_SLICE_COMPRESSIBLE="517.5" # Default value for system slice compressible
MONITOR_INTERVAL=5

usage() {
    echo "Usage: $0 [-n <node-name>] [-d <duration-seconds>] [-s <slice-specs>] [-c <compressible-value>]"
    echo "  -n, --node          : Name of the node to stress (default: a non-prometheus worker node)"
    echo "  -d, --duration      : Duration of stress test in seconds (default: 300)"
    echo "  -s, --slices        : Comma-separated list of 'slice:cores' pairs (default: system.slice:4,kubepods.slice:7)"
    echo "  -c, --compressible  : The compressible value for the system slice (default: 500)"
    exit 1
}

if [[ "$#" -eq 0 ]]; then
    usage
fi

while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -n|--node)
        TARGET_NODE="$2"
        shift # past argument
        shift # past value
        ;;
        -d|--duration)
        STRESS_DURATION="$2"
        shift # past argument
        shift # past value
        ;;
        -s|--slices)
        SLICE_SPECS="$2"
        shift # past argument
        shift # past value
        ;;
        -c|--compressible)
        SYSTEM_SLICE_COMPRESSIBLE="$2"
        shift # past argument
        shift # past value
        ;;
        -h|--help)
        usage
        ;;
        *)    # unknown option
        echo "Unknown option: $1"
        usage
        ;;
    esac
done


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
            # sleep 1s to reduce pod creation burst
            sleep 1
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

parse_results() {
    echo "Parsing results directly from JSON output and storing into variables:"

    # MID_TIME
    MID_KUBEPODS_SLICE=$(printf "%.2f" "$(echo "$1" | jq -r '.data.result[] | select(.metric.id == "/kubepods.slice") | .value[1]')")
    MID_SYSTEM_SLICE=$(printf "%.2f" "$(echo "$1" | jq -r '.data.result[] | select(.metric.id == "/system.slice") | .value[1]')")

    # AVG_OVER_TIME
    AVG_KUBEPODS_SLICE=$(printf "%.2f" "$(echo "$2" | jq -r '.data.result[] | select(.metric.id == "/kubepods.slice") | .value[1]')")
    AVG_SYSTEM_SLICE=$(printf "%.2f" "$(echo "$2" | jq -r '.data.result[] | select(.metric.id == "/system.slice") | .value[1]')")

    # MAX_OVER_TIME
    MAX_KUBEPODS_SLICE=$(printf "%.2f" "$(echo "$3" | jq -r '.data.result[] | select(.metric.id == "/kubepods.slice") | .value[1]')")
    MAX_SYSTEM_SLICE=$(printf "%.2f" "$(echo "$3" | jq -r '.data.result[] | select(.metric.id == "/system.slice") | .value[1]')")

    # Optionally print them to confirm (for debugging/user feedback)
    echo "--- Parsed Values ---"
    echo "MID_KUBEPODS_SLICE: $MID_KUBEPODS_SLICE"
    echo "MID_SYSTEM_SLICE: $MID_SYSTEM_SLICE"
    echo "AVG_KUBEPODS_SLICE: $AVG_KUBEPODS_SLICE"
    echo "AVG_SYSTEM_SLICE: $AVG_SYSTEM_SLICE"
    echo "MAX_KUBEPODS_SLICE: $MAX_KUBEPODS_SLICE"
    echo "MAX_SYSTEM_SLICE: $MAX_SYSTEM_SLICE"
}

verify_results() {
    log_info "========================================="                                                                                                                                                                          │
    log_info "Checking for Throttling"                                                                                                                                                                                            │
    log_info "=========================================" 
    oc debug node/${TARGET_NODE} -- bash -c "chroot /host cat /sys/fs/cgroup/system.slice/cpu.stat" | grep -E "nr_throttled|throttled_usec"

    echo "--- Verifying System Slice Compressible ---"
    echo "SYSTEM_SLICE_COMPRESSIBLE threshold: $SYSTEM_SLICE_COMPRESSIBLE"

    # Using awk for floating point comparison
    local mid_check=$(awk -v val="$MID_SYSTEM_SLICE" -v threshold="$SYSTEM_SLICE_COMPRESSIBLE" 'BEGIN { print (val <= threshold) }')
    local avg_check=$(awk -v val="$AVG_SYSTEM_SLICE" -v threshold="$SYSTEM_SLICE_COMPRESSIBLE" 'BEGIN { print (val <= threshold) }')
    local max_check=$(awk -v val="$MAX_SYSTEM_SLICE" -v threshold="$SYSTEM_SLICE_COMPRESSIBLE" 'BEGIN { print (val <= threshold) }')

    log_info "========================================="
    log_info "Test Result - System Slice Compressible"
    log_info "========================================="

    if [ "$mid_check" -eq 1 ]; then
        echo "Test Case PASSED: MID_SYSTEM_SLICE value is within the compressible limit."
        echo "  - MID_SYSTEM_SLICE ($MID_SYSTEM_SLICE) <= $SYSTEM_SLICE_COMPRESSIBLE"
        echo "  - AVG_SYSTEM_SLICE ($AVG_SYSTEM_SLICE)"
        echo "  - MAX_SYSTEM_SLICE ($MAX_SYSTEM_SLICE)"
        exit 0
    else
        echo "Test Case FAILED: MID_SYSTEM_SLICE value exceeded the compressible limit."
        echo "  - MID_SYSTEM_SLICE ($MID_SYSTEM_SLICE) > $SYSTEM_SLICE_COMPRESSIBLE"
        if [ "$avg_check" -ne 1 ]; then echo "  - AVG_SYSTEM_SLICE ($AVG_SYSTEM_SLICE) > $SYSTEM_SLICE_COMPRESSIBLE"; fi
        if [ "$max_check" -ne 1 ]; then echo "  - MAX_SYSTEM_SLICE ($MAX_SYSTEM_SLICE) > $SYSTEM_SLICE_COMPRESSIBLE"; fi
        exit 1
    fi
}

run_queries() {
    query_duration=$((STRESS_DURATION/2))
    token=`oc create token prometheus-k8s -n openshift-monitoring`

    query="sum by (id)(irate(container_cpu_usage_seconds_total{id=~'/system.slice|/kubepods.slice', node='$TARGET_NODE'}[1m])) * 1000"
    query_avg='avg_over_time(sum by (id)(irate(container_cpu_usage_seconds_total{id=~"/system.slice|/kubepods.slice", node="'"$TARGET_NODE"'"}[1m]))['"${query_duration}s"':1m]) * 1000'
    query_max='max_over_time(sum by (id)(irate(container_cpu_usage_seconds_total{id=~"/system.slice|/kubepods.slice", node="'"$TARGET_NODE"'"}[1m]))['"${query_duration}s"':1m]) * 1000'

    echo "=== system.slice & kubepods.slice (millicores) metrics on the middle of the test ===" | tee -a "$output_file"
    MID_TIME_JSON=$(oc -n openshift-monitoring exec -c prometheus prometheus-k8s-0 -- curl -sk -H "Authorization: Bearer $token" 'https://thanos-querier.openshift-monitoring.svc:9091/api/v1/query' --data-urlencode "query=$query" --data-urlencode "time=$MID_TIME" | jq | tee -a "$output_file")
    echo $MID_TIME_JSON

    echo "=== avg_over_time from middle of the test to the end ===" | tee -a "$output_file"
    AVG_OVER_TIME_JSON=$(oc -n openshift-monitoring exec -c prometheus prometheus-k8s-0 -- \
      curl -sk -H "Authorization: Bearer $token" \
      'https://thanos-querier.openshift-monitoring.svc:9091/api/v1/query' \
      --data-urlencode "query=$query_avg" \
      --data-urlencode "time=$END_TIME" | jq | tee -a "$output_file")
    echo $AVG_OVER_TIME_JSON

    echo "=== max_over_time from middle of the test to the end ===" | tee -a "$output_file"
    MAX_OVER_TIME_JSON=$(oc -n openshift-monitoring exec -c prometheus prometheus-k8s-0 -- \
      curl -sk -H "Authorization: Bearer $token" \
      'https://thanos-querier.openshift-monitoring.svc:9091/api/v1/query' \
      --data-urlencode "query=$query_max" \
      --data-urlencode "time=$END_TIME" | jq | tee -a "$output_file")
    echo $MAX_OVER_TIME_JSON
}



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
    log_info "Monitoring output will be saved to: $output_file"

    echo "=== Systemd Slice Stress Test Monitor Log ===" > "$output_file"
    echo "Start Time: $(date)" >> "$output_file"
    echo "Stress Duration: ${STRESS_DURATION}s" >> "$output_file"
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
                grep -v -E "(Running|Completed|STATUS|installer-)" || echo "All pods running or no pod found"
            
            # # Recent events
            # echo ""
            # echo "--- Recent Node Events (last 2 minutes) ---"
            # kubectl get events -A --field-selector involvedObject.name="$TARGET_NODE" \
            #     --sort-by='.lastTimestamp' 2>/dev/null | tail -10 || echo "No recent events"

            echo ""
            echo "--- Stress test processes are running ---"
            oc debug node/${TARGET_NODE} -- bash -c "chroot /host cat systemctl --type=service --state=running | grep stress-test" || echo "No stress test processes are running"
            
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
        # sleep 1s to reduce pod creation burst
        sleep 1
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

# Define the output file name
output_file="stress_test_log_${TARGET_NODE}_$(date +%Y%m%d_%H%M%S).log"

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

# Wait a 5s and show final status
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
log_info "Log file: $output_file"
log_info ""
log_info "chroot /host journalctl -u stress-test-* --since '10 minutes ago'"
log_info ""
log_info "========================================="

echo "Start timestamp: $START_TIME. Start time: "$(date -r $START_TIME)
echo "End timestamp: $END_TIME. End time: "$(date -r $END_TIME)
MID_TIME=$((START_TIME + STRESS_DURATION/2))
echo "Mid timestamp: $MID_TIME. Mid time: "$(date -r $MID_TIME)

run_queries
parse_results "$MID_TIME_JSON" "$AVG_OVER_TIME_JSON" "$MAX_OVER_TIME_JSON"
verify_results
