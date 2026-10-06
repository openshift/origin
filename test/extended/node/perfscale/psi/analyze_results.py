import json
import os
import re
import sys
from collections import defaultdict

if len(sys.argv) < 2:
    print("Usage: python3 analyze_results.py <log_directory>")
    sys.exit(1)

LOG_DIR = sys.argv[1]

def parse_top_node_log(file_path):
    """Parses the output of 'kubectl top node'."""
    with open(file_path, 'r') as f:
        lines = f.readlines()
        if len(lines) < 2:
            return None, None
        
        # Skip header
        parts = lines[1].split()
        
        cpu_raw = parts[1]
        mem_raw = parts[3]

        cpu_cores = int(cpu_raw.replace('m', ''))
        
        mem_kib = 0
        if 'Ki' in mem_raw:
            mem_kib = int(mem_raw.replace('Ki', ''))
        elif 'Mi' in mem_raw:
            mem_mib_raw = int(mem_raw.replace('Mi', ''))
            mem_kib = mem_mib_raw * 1024
        
        mem_mib = mem_kib / 1024

        return cpu_cores, mem_mib

def parse_stats_summary(file_path):
    """Parses the kubelet stats/summary JSON."""
    with open(file_path, 'r') as f:
        data = json.load(f)
        
        kubelet_stats = next((sc for sc in data.get("node", {}).get("systemContainers", []) if sc["name"] == "kubelet"), None)
        if not kubelet_stats:
            return None, None

        cpu_nanocores = kubelet_stats.get("cpu", {}).get("usageNanoCores", 0)
        mem_workingset_bytes = kubelet_stats.get("memory", {}).get("workingSetBytes", 0)

        return cpu_nanocores, mem_workingset_bytes / (1024**2) # Convert to MiB

# Helper to calculate delta
def get_delta(base, test, unit, is_cpu=False):
    if base is None or test is None:
        return "N/A"
    delta = test - base
    percent = (delta / base * 100) if base != 0 else 0
    sign = "+" if delta >= 0 else ""
    
    # Convert nanocores to millicores for CPU
    if is_cpu:
        base /= 1_000_000
        test /= 1_000_000
        delta /= 1_000_000

    return f"{base:.2f} -> {test:.2f} ({sign}{delta:.2f} / {sign}{percent:.2f}%)"
        
def main():
    results = defaultdict(lambda: defaultdict(dict))
    
    for filename in os.listdir(LOG_DIR):
        if not filename.endswith(".log") and not filename.endswith(".json"):
            continue

        parts = filename.split('_')
        
        feature_status = "enabled" if "enabled" in filename else "disabled"
        stress_type = "unknown"
        if "cpu" in filename:
            stress_type = "cpu"
        elif "memory" in filename:
            stress_type = "memory"
        elif "io" in filename:
            stress_type = "io"

        condition = "unknown"
        if "idle" in filename:
            condition = "idle"
        elif "load" in filename:
            condition = "load"

        file_path = os.path.join(LOG_DIR, filename)

        if "top_node" in filename:
            cpu, mem = parse_top_node_log(file_path)
            results[stress_type][feature_status][f"{condition}_node_cpu_m"] = cpu
            results[stress_type][feature_status][f"{condition}_node_mem_mib"] = mem
        elif "stats_summary" in filename:
            cpu, mem = parse_stats_summary(file_path)
            results[stress_type][feature_status][f"{condition}_kubelet_cpu_nanocores"] = cpu
            results[stress_type][feature_status][f"{condition}_kubelet_mem_mib"] = mem

    print("--- Performance Analysis Summary by kubectl top node and /proxy/stats/summary---")

    for stress_type, data in sorted(results.items()):
        print(f"\n### Stress Type: {stress_type.upper()}\n")

        print("| Metric                 | Condition | Result (Baseline -> Test) |")
        print("|------------------------|-----------|---------------------------|")
        # Node Metrics
        idle_node_cpu = get_delta(data["disabled"].get("idle_node_cpu_m"), data["enabled"].get("idle_node_cpu_m"), "m")
        load_node_cpu = get_delta(data["disabled"].get("load_node_cpu_m"), data["enabled"].get("load_node_cpu_m"), "m")
        idle_node_mem = get_delta(data["disabled"].get("idle_node_mem_mib"), data["enabled"].get("idle_node_mem_mib"), "MiB")
        load_node_mem = get_delta(data["disabled"].get("load_node_mem_mib"), data["enabled"].get("load_node_mem_mib"), "MiB")
        
        print(f"| **Node CPU (m)**       | Idle      | {idle_node_cpu} |")
        print(f"| **Node CPU (m)**       | Load      | {load_node_cpu} |")
        print(f"| **Node Memory (MiB)**  | Idle      | {idle_node_mem} |")
        print(f"| **Node Memory (MiB)**  | Load      | {load_node_mem} |")

        # Kubelet Metrics
        idle_kubelet_cpu = get_delta(data["disabled"].get("idle_kubelet_cpu_nanocores"), data["enabled"].get("idle_kubelet_cpu_nanocores"), "m", is_cpu=True)
        load_kubelet_cpu = get_delta(data["disabled"].get("load_kubelet_cpu_nanocores"), data["enabled"].get("load_kubelet_cpu_nanocores"), "m", is_cpu=True)
        idle_kubelet_mem = get_delta(data["disabled"].get("idle_kubelet_mem_mib"), data["enabled"].get("idle_kubelet_mem_mib"), "MiB")
        load_kubelet_mem = get_delta(data["disabled"].get("load_kubelet_mem_mib"), data["enabled"].get("load_kubelet_mem_mib"), "MiB")

        print(f"| **Kubelet CPU (m)**    | Idle      | {idle_kubelet_cpu} |")
        print(f"| **Kubelet CPU (m)**    | Load      | {load_kubelet_cpu} |")
        print(f"| **Kubelet Memory (MiB)**| Idle      | {idle_kubelet_mem} |")
        print(f"| **Kubelet Memory (MiB)**| Load      | {load_kubelet_mem} |")

if __name__ == "__main__":
    main()
