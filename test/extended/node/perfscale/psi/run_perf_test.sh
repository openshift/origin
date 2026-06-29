#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status.

# Change to the script's directory to ensure outputs are created locally.
cd "$(dirname "$0")"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_DIR="./perf_logs_${TIMESTAMP}"
mkdir -p "$LOG_DIR"

# --- Function to generate stress pod definition ---
generate_stress_pod_yaml() {
  local stress_type=$1
  local pod_name="${stress_type}-stress-pod"
  local command
  local args
  local resources="" # Default to empty

  case "$stress_type" in
    "cpu")
      command='["/agnhost"]'
      args='["stress", "--cpus", "2"]'
      resources=$(printf "    resources:\n      requests:\n        cpu: \"1\"\n      limits:\n        cpu: \"2\"")
      ;;
    "memory")
      command='["/agnhost"]'
      args='["stress", "--mem-total", "8589934592", "--mem-alloc-size", "1073741824"]'
      resources=$(printf "    resources:\n      requests:\n        memory: \"8Gi\"\n      limits:\n        memory: \"9Gi\"")
      ;;
    "io")
      # This command runs an infinite loop to generate sustained I/O pressure.
      command='["/bin/sh", "-c"]'
      args='["while true; do dd if=/dev/zero of=testfile bs=1M count=128 &>/dev/null; sync; rm testfile &>/dev/null; done"]'
      resources=$(printf "    resources:\n      requests:\n        cpu: \"250m\"")
      ;;
    *)
      echo "Invalid stress type: ${stress_type}"
      exit 1
      ;;
  esac

  cat > "${pod_name}.yaml" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
spec:
  nodeSelector:
    node-role.kubernetes.io/perf: ""
  restartPolicy: Never
  containers:
  - name: ${stress_type}-stress
    image: registry.k8s.io/e2e-test-images/agnhost:2.56
    command: ${command}
    args: ${args}
${resources}
EOF
}

generate_memory_stress_pod_yaml() {
  local stress_type=$1
  local pod_name="${stress_type}-stress-pod"
  local command
  local args
  local resources="" # Default to empty

  case "$stress_type" in
    "memory")
      memory='8Gi'
      count='8192'
      memory_limit='9Gi'
      ;;
    *)
      echo "Invalid stress type: ${stress_type}"
      exit 1
      ;;
  esac

  cat > "${pod_name}.yaml" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
spec:
  nodeSelector:
    node-role.kubernetes.io/perf: ""
  restartPolicy: Never
  containers:
  - name: ${stress_type}-stress
    image: registry.access.redhat.com/ubi8/ubi-minimal:latest
    command: ["/bin/sh"]
    args:
      - -c
      - |
        dd if=/dev/zero of=/dev/shm/memory bs=1M count=${count}
        echo "Memory allocated, sleeping forever..."
        sleep infinity
    lifecycle:
      preStop:
        exec:
          command:
            - /bin/sh
            - -c
            - |
              echo "Cleaning up memory..."
              rm -f /dev/shm/memory
              sync
              sleep 2
    resources:
      requests:
        memory: "${memory}"
      limits:
        memory: "${memory_limit}"
    volumeMounts:
    - name: tmpfs
      mountPath: /dev/shm
  volumes:
  - name: tmpfs
    emptyDir:
      medium: Memory
      sizeLimit: ${memory}
EOF
}

prepare_test() {
  echo "Prepare test..."
  # --- Apply test  pod ---
  echo "Applying test pod to find the least usage node..."
  cat > "test-pod.yaml" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  nodeSelector:
    node-role.kubernetes.io/worker: ""
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
EOF
  kubectl apply -f test-pod.yaml
  kubectl wait --for=condition=Ready pod/test-pod --timeout=120s
  NODE_NAME=$(kubectl get po test-pod -o jsonpath='{.spec.nodeName}')
  # --- Cleanup ---
  echo "Cleaning up test pod..."
  kubectl delete -f test-pod.yaml --ignore-not-found=true
  echo "Perf test will be run on node: $NODE_NAME"
  rm -rf test-pod.yaml
  # --- Label the perf test worker nodes ---
  echo "Label the perf test node $NODE_NAME with node-role.kubernetes.io/perf..."
  oc label node $NODE_NAME node-role.kubernetes.io/perf=
  # The below restart kublet step seems introduce more unstability to kubelet cpu, so I disable it for now.
  #echo "Restart kublet on the perf test worker nodes to get the memory down to baseline"
  #oc debug node/$NODE_NAME -- chroot /host systemctl restart kubelet
  #echo "Wait 5 minutes to let the"
  #sleep 300
  # --- generate the stress pod yaml ---
  echo "Generating the stress pod yaml..."
  generate_stress_pod_yaml "cpu"
  generate_memory_stress_pod_yaml "memory"
  generate_stress_pod_yaml "io"
}

cleanup_test() {
  oc label node $NODE_NAME node-role.kubernetes.io/perf-
  rm -rf *-stress-pod.yaml
}

run_test() {
  local feature_enabled=$1
  local stress_type=$2
  local log_prefix="baseline_feature_disabled"
  [ "$feature_enabled" = true ] && log_prefix="test_feature_enabled"
  log_prefix="${log_prefix}_${stress_type}_stress"

  local cluster_name="${CLUSTER_NAME_BASE}-$(date +%s)"
  local pod_yaml="${stress_type}-stress-pod.yaml"

  echo "--- Starting Test: Feature enabled = ${feature_enabled}, Stress Type = ${stress_type} ---"

  # --- Metric Collection (Idle) ---
  echo "Collecting idle proxy metrics..."
  kubectl top node "${NODE_NAME}" > "${LOG_DIR}/${log_prefix}_idle_top_node.log"
  kubectl get --raw "/api/v1/nodes/${NODE_NAME}/proxy/stats/summary" > "${LOG_DIR}/${log_prefix}_idle_stats_summary.json"

  # --- Apply Stress Workload ---
  echo "Applying ${stress_type} stress workload..."
  # YAML is already generated and validated by the smoke test
  kubectl apply -f "${pod_yaml}"
  kubectl wait --for=condition=Ready pod/"${stress_type}-stress-pod" --timeout=120s
  echo "${stress_type} stress workload is ready"
  date

  # --- Metric Collection (Under Load) ---
  echo "Collecting proxy metrics under load after 180 seconds..."
  sleep 180
  kubectl top node "${NODE_NAME}" > "${LOG_DIR}/${log_prefix}_load_top_node.log"
  kubectl get --raw "/api/v1/nodes/${NODE_NAME}/proxy/stats/summary" > "${LOG_DIR}/${log_prefix}_load_stats_summary.json"
  kubectl get --raw "/api/v1/nodes/${NODE_NAME}/proxy/metrics/cadvisor" > "${LOG_DIR}/${log_prefix}_load_metrics_cadvisor.log"

  # --- Cleanup ---
  echo "Cleaning up workload..."
  date
  kubectl delete -f "${pod_yaml}" --ignore-not-found=true

  echo "--- Finished Test: ${log_prefix} ---"
}

persist_monitoring() {
  set +e
  oc get cm -n openshift-monitoring cluster-monitoring-config -o yaml  | grep volumeClaimTemplate > /dev/null 2>&1
  if [[ $? -eq 1  ]]; then
      echo "--- Configure persistence for monitoring ---"
      export PROMETHEUS_RETENTION_PERIOD=20d
      export PROMETHEUS_STORAGE_SIZE=50Gi
      export ALERTMANAGER_STORAGE_SIZE=2Gi
      envsubst < cluster-monitoring-config.yaml | oc apply -f -
      echo "--- Sleep 60s to wait for monitoring to take the new config map ---"
      sleep 60
      oc rollout status -n openshift-monitoring deploy/cluster-monitoring-operator
      oc rollout status -n openshift-monitoring sts/prometheus-k8s
      token=$(oc create token -n openshift-monitoring prometheus-k8s --duration=6h)
      URL=https://$(oc get route -n openshift-monitoring prometheus-k8s -o jsonpath="{.spec.host}")
      prom_status="not_started"
      echo "Sleep 30s to wait for prometheus status to become success."
      sleep 30
      retry=20
      while [[ "$prom_status" != "success" && $retry -gt 0 ]]; do
          retry=$(($retry-1))
          echo "--- Prometheus status is not success yet, retrying in 10s, retries left: $retry ---"
          sleep 10
          prom_status=$(curl -s -g -k -X GET -H "Authorization: Bearer $token" -H 'Accept: application/json' -H 'Content-Type: application/json' "$URL/api/v1/query?query=up" | jq -r '.status')
      done
      if [[ "$prom_status" != "success" ]]; then
          prom_status=$(curl -s -g -k -X GET -H "Authorization: Bearer $token" -H 'Accept: application/json' -H 'Content-Type: application/json' "$URL/api/v1/query?query=up" | jq -r '.status')
          echo "--- Prometheus status is '$prom_status'. 'success' is expected ---"
          exit 1
      else
          echo "--- Prometheus is success now. ---"
          echo "--- Sleep 5m to wait for nodes to become stable as persis_monitoring may cause monitoring to be relocated ---"
          sleep 5m
      fi
  else
    echo "--- Monitoring persistence is already configured ---"
  fi
  set -e
}

function install_dittybopper(){
  oc get ns dittybopper
  if [[ $? -eq 0 ]]; then
    echo "--- Dittybopper is already installed ---"
    return
  else
    echo "--- Install dittybopper ---"
    if [[ ! -d performance-dashboards ]]; then
        git clone git@github.com:cloud-bulldozer/performance-dashboards.git --depth 1
    fi
    cd performance-dashboards/dittybopper
    ./deploy.sh -i "$IMPORT_DASHBOARD"

    if [[ $? -eq 0 ]];then
        log "info" "dittybopper installed successfully."
    else
        log "error" "dittybopper install failed."
    fi
  fi
}

# --- Main Execution ---

if [[ "$1" == "--dry-run" ]]; then
  echo "--- YAML Generation Dry Run ---"
  generate_stress_pod_yaml "cpu"
  echo "--- cpu-stress-pod.yaml ---"
  cat cpu-stress-pod.yaml
  echo
  generate_memory_stress_pod_yaml "memory"
  echo "--- memory-stress-pod.yaml ---"
  cat memory-stress-pod.yaml
  echo
  generate_stress_pod_yaml "io"
  echo "--- io-stress-pod.yaml ---"
  cat io-stress-pod.yaml
  exit 0
fi

persist_monitoring
install_dittybopper

STRESS_TYPES=("cpu" "memory" "io")

prepare_test
for stress in "${STRESS_TYPES[@]}"; do
 run_test false "${stress}"
 echo "--- Sleep 180s to let the previous stress to cool down ---"
 sleep 180
done
 
./enable_psi.sh
echo "--- Sleep 10m to let the cluster to become stable after nodes reboot after enabling PSI ---"
sleep 10m

for stress in "${STRESS_TYPES[@]}"; do
 run_test true "${stress}"
 echo "--- Sleep 180s to let the previous stress to cool down ---"
 sleep 180
done
cleanup_test

echo "--- All tests completed. Logs are in ${LOG_DIR} ---"
echo "--- Analyzing results... ---"
python3 analyze_results.py "${LOG_DIR}"