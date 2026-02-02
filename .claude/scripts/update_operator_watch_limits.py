#!/usr/bin/env python3
"""
Update operator watch request limits from BigQuery data.

This script reads BigQuery JSON output and updates operator_watch_limits.json
by modifying the JSON structure keys.

Usage:
    python3 update_operator_watch_limits.py /tmp/bq_results.json
    cat /tmp/bq_results.json | python3 update_operator_watch_limits.py
"""

import sys
import json
from datetime import datetime
from pathlib import Path


# Platform name mapping: BigQuery lowercase -> configv1.PlatformType format
PLATFORM_MAPPING = {
    "aws": "AWS",
    "azure": "Azure",
    "gcp": "GCP",
    "metal": "BareMetal",
    "vsphere": "vSphere",
    "openstack": "OpenStack",
}

# Topology mapping: BigQuery -> configv1.TopologyMode format
TOPOLOGY_MAPPING = {
    "ha": "HighlyAvailable",
    "single": "SingleReplica",
}


def ensure_operator_suffix(operator_name):
    """Ensure operator name ends with -operator suffix."""
    if not operator_name.endswith('-operator'):
        return f"{operator_name}-operator"
    return operator_name


def main():
    # Read BigQuery data
    if len(sys.argv) > 1:
        print(f"Reading BigQuery data from: {sys.argv[1]}", file=sys.stderr)
        with open(sys.argv[1], 'r') as f:
            bq_data = json.load(f)
    else:
        print("Reading BigQuery data from stdin...", file=sys.stderr)
        bq_data = json.load(sys.stdin)

    # Find limits file
    script_dir = Path(__file__).parent
    repo_root = script_dir.parent.parent
    limits_path = repo_root / 'pkg' / 'monitortests' / 'kubeapiserver' / 'auditloganalyzer' / 'operator_watch_limits.json'

    if not limits_path.exists():
        print(f"❌ Error: {limits_path} not found", file=sys.stderr)
        return 1

    # Load current limits
    print(f"Loading: {limits_path}", file=sys.stderr)
    with open(limits_path, 'r') as f:
        limits = json.load(f)

    # Track stats
    stats = {'updated': 0, 'critical': [], 'warning': [], 'normal': [], 'skipped': []}

    # Update limits from BigQuery data
    for record in bq_data:
        platform_bq = record.get('Platform', '').lower()
        topology_bq = record.get('Topology', '').lower()
        operator_bq = record.get('Operator', '')
        p99 = float(record.get('P99_WatchCount', 0))

        if not all([platform_bq, topology_bq, operator_bq]):
            continue

        # Map to config format
        platform = PLATFORM_MAPPING.get(platform_bq)
        topology = TOPOLOGY_MAPPING.get(topology_bq)
        operator = ensure_operator_suffix(operator_bq)

        if not platform or not topology:
            continue

        # Check if exists in limits
        if topology not in limits or platform not in limits[topology] or operator not in limits[topology][platform]:
            stats['skipped'].append(f"{operator} ({platform}/{topology})")
            continue

        # Calculate new limit
        old_limit = limits[topology][platform][operator]
        new_limit = int(round(p99))
        ratio = new_limit / old_limit if old_limit > 0 else 1.0

        # Categorize change
        if ratio > 10:
            stats['critical'].append(f"{operator} {platform}/{topology}: {old_limit}→{new_limit} ({ratio:.1f}x)")
        elif ratio >= 2:
            stats['warning'].append(f"{operator} {platform}/{topology}: {old_limit}→{new_limit} ({ratio:.1f}x)")
        else:
            pct = ((new_limit - old_limit) / old_limit * 100) if old_limit > 0 else 0
            stats['normal'].append(f"{operator} {platform}/{topology}: {old_limit}→{new_limit} ({pct:+.0f}%)")

        # Update JSON
        limits[topology][platform][operator] = new_limit
        stats['updated'] += 1

    # Update timestamp
    limits['_last_updated'] = datetime.now().strftime('%Y-%m-%d')

    # Write updated JSON
    print(f"Writing: {limits_path}", file=sys.stderr)
    with open(limits_path, 'w') as f:
        json.dump(limits, f, indent=2, sort_keys=False)
        f.write('\n')

    # Print summary
    print("\n" + "="*70, file=sys.stderr)
    if stats['critical']:
        print(f"🚨 CRITICAL ({len(stats['critical'])}):", file=sys.stderr)
        for item in stats['critical']:
            print(f"  ❌ {item}", file=sys.stderr)

    if stats['warning']:
        print(f"⚠️  WARNING ({len(stats['warning'])}):", file=sys.stderr)
        for item in stats['warning'][:5]:
            print(f"  ⚠️  {item}", file=sys.stderr)
        if len(stats['warning']) > 5:
            print(f"  ... {len(stats['warning'])-5} more", file=sys.stderr)

    if stats['normal']:
        print(f"✅ UPDATED ({len(stats['normal'])}):", file=sys.stderr)
        for item in stats['normal'][:10]:
            print(f"  ✅ {item}", file=sys.stderr)
        if len(stats['normal']) > 10:
            print(f"  ... {len(stats['normal'])-10} more", file=sys.stderr)

    print(f"\nTotal: {stats['updated']} operators updated", file=sys.stderr)
    print(f"Last updated: {limits['_last_updated']}", file=sys.stderr)
    print("="*70, file=sys.stderr)

    if stats['critical']:
        print(f"⚠️  {len(stats['critical'])} CRITICAL - review before committing!", file=sys.stderr)
        return 2

    return 0


if __name__ == '__main__':
    sys.exit(main())
