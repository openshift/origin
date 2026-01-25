#!/bin/bash
#
# enable-psi.sh - Enable PSI (Pressure Stall Information) on OpenShift Worker Nodes
#
# This script:
# 1. Checks if PSI is already enabled
# 2. Creates and applies MachineConfig to enable PSI
# 3. Waits for worker nodes to be updated
# 4. Verifies PSI is enabled on all workers
#
# Requirements:
# - oc CLI installed and logged in
# - Cluster admin permissions
#
# Usage: ./enable-psi.sh

set -e  # Exit on error
set -o pipefail  # Catch errors in pipelines

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
MACHINECONFIG_NAME="99-worker-enable-psi"
MACHINECONFIG_FILE="99-worker-enable-psi.yaml"
CHECK_INTERVAL=30  # seconds between checks
MAX_WAIT_TIME=3600  # 1 hour timeout

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if oc is installed
    if ! command -v oc &> /dev/null; then
        log_error "oc CLI is not installed. Please install it first."
        exit 1
    fi
    
    # Check if logged in
    if ! oc whoami &> /dev/null; then
        log_error "Not logged in to OpenShift. Please run 'oc login' first."
        exit 1
    fi
    
    # Check cluster admin permissions
    if ! oc auth can-i create machineconfigs &> /dev/null; then
        log_error "Insufficient permissions. Cluster admin access required."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

check_psi_status() {
    local node=$1
    # Check if PSI files exist
    psi_check=$(
    oc debug node/"$node" -- chroot /host bash -c '
        [ -f /proc/pressure/cpu ] && [ -f /proc/pressure/memory ] && [ -f /proc/pressure/io ] \
        && echo enabled || echo disabled
    ' 2>/dev/null | grep -E "enabled|disabled"
    )
    
    echo "$psi_check"
}

check_all_workers_psi() {
    log_info "Checking PSI status on all worker nodes..."
    
    local workers=$(oc get nodes -l node-role.kubernetes.io/worker -o jsonpath='{.items[*].metadata.name}')
    local all_enabled=true
    
    for worker in $workers; do
        local status=$(check_psi_status "$worker")
        if [ "$status" == "enabled" ]; then
            log_success "PSI is enabled on $worker"
        else
            log_warning "PSI is NOT enabled on $worker"
            all_enabled=false
        fi
    done
    
    if [ "$all_enabled" = true ]; then
        return 0
    else
        return 1
    fi
}

create_machineconfig_yaml() {
    log_info "Creating MachineConfig YAML: $MACHINECONFIG_FILE"
    
    cat > "$MACHINECONFIG_FILE" << 'EOF'
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  labels:
    machineconfiguration.openshift.io/role: "worker"
  name: 99-worker-enable-psi
spec:
  kernelArguments:
    - psi=1
EOF
    
    log_success "MachineConfig YAML created"
}

apply_machineconfig() {
    log_info "Applying MachineConfig to cluster..."
    
    if oc get machineconfig "$MACHINECONFIG_NAME" &> /dev/null; then
        log_error "MachineConfig $MACHINECONFIG_NAME already exists!"
        log_error "Please delete it first with: oc delete machineconfig $MACHINECONFIG_NAME"
        exit 1
    else
        oc apply -f "$MACHINECONFIG_FILE"
        log_success "MachineConfig applied"
    fi
    rm -rf $MACHINECONFIG_FILE
}

wait_for_mcp_update() {
    log_info "Waiting for MachineConfigPool 'worker' to update..."
    log_info "This may take 30-60 minutes. Workers will reboot one by one."
    
    local elapsed=0
    local mcp_name="worker"
    
    while [ $elapsed -lt $MAX_WAIT_TIME ]; do
        # Get MCP status (with default values if fields don't exist)
        local updated=$(oc get mcp "$mcp_name" -o jsonpath='{.status.updatedMachineCount}' 2>/dev/null || echo "0")
        local total=$(oc get mcp "$mcp_name" -o jsonpath='{.status.machineCount}' 2>/dev/null || echo "0")
        local ready=$(oc get mcp "$mcp_name" -o jsonpath='{.status.readyMachineCount}' 2>/dev/null || echo "0")
        local degraded=$(oc get mcp "$mcp_name" -o jsonpath='{.status.degradedMachineCount}' 2>/dev/null || echo "0")
        
        # Set to 0 if empty
        updated=${updated:-0}
        total=${total:-0}
        ready=${ready:-0}
        degraded=${degraded:-0}
        
        log_info "MCP Status: Updated=$updated/$total, Ready=$ready/$total, Degraded=$degraded"
        
        # Check if all nodes are updated and ready
        if [ "$updated" -eq "$total" ] && [ "$ready" -eq "$total" ]; then
            log_success "All worker nodes have been updated!"
            return 0
        fi
        
        # Check for degraded nodes
        if [ "$degraded" -gt 0 ]; then
            log_warning "Some nodes are degraded. Checking details..."
            oc get mcp "$mcp_name" -o yaml | grep -A 5 "conditions:"
        fi
        
        log_info "Waiting $CHECK_INTERVAL seconds before next check... (elapsed: ${elapsed}s)"
        sleep $CHECK_INTERVAL
        elapsed=$((elapsed + CHECK_INTERVAL))
    done
    
    log_error "Timeout waiting for worker nodes to update after ${MAX_WAIT_TIME}s"
    return 1
}

show_node_status() {
    log_info "Current worker node status:"
    oc get nodes -l node-role.kubernetes.io/worker -o wide
}

main() {
    echo "=========================================="
    echo "  OpenShift PSI Enablement Script"
    echo "=========================================="
    echo
    
    # Step 0: Check prerequisites
    check_prerequisites
    echo
    
    # Step 1: Check if PSI is already enabled
    log_info "Step 1: Checking current PSI status on worker nodes..."
    if check_all_workers_psi; then
        log_success "PSI is already enabled on all worker nodes!"
        exit 0
    else
        log_info "PSI is not fully enabled. Proceeding with enablement..."
    fi
    echo
    
    # Step 2: Create MachineConfig YAML
    log_info "Step 2: Creating MachineConfig YAML..."
    create_machineconfig_yaml
    echo
    
    # Step 3: Apply MachineConfig
    log_info "Step 3: Applying MachineConfig to cluster..."
    apply_machineconfig
    echo
    
    # Show current node status
    show_node_status
    echo
    echo "--- Wait 120 seconds for mcp to start updating ---"
    sleep 120

    # Step 4: Wait for workers to update
    log_info "Step 4: Waiting for all worker nodes to be updated..."
    if wait_for_mcp_update; then
        log_success "Worker node update completed successfully!"
    else
        log_error "Failed to update all worker nodes within timeout"
        log_info "You can check the status manually with: oc get mcp worker -w"
        exit 1
    fi
    echo
    echo "--- Wait 60 seconds for PSI to be ready on nodes---"
    sleep 60
    
    # Step 5: Verify PSI is enabled
    log_info "Step 5: Verifying PSI is enabled on all worker nodes..."
    sleep 10  # Wait a bit for nodes to stabilize
    
    if check_all_workers_psi; then
        log_success "✅ PSI has been successfully enabled on all worker nodes!"
    else
        log_error "❌ PSI verification failed on some nodes"
        log_info "Please check individual node status manually"
        exit 1
    fi
    echo
    
    # Final status
    echo "=========================================="
    log_success "PSI Enablement Complete!"
    echo "=========================================="
    log_info "You can verify PSI on any worker node with:"
    echo "  oc debug node/<node-name> -- chroot /host cat /proc/pressure/cpu"
    echo
    log_info "MachineConfig created: $MACHINECONFIG_FILE"
    log_info "To remove this configuration later, run:"
    echo "  oc delete machineconfig $MACHINECONFIG_NAME"
    echo
}

# Run main function
main "$@"