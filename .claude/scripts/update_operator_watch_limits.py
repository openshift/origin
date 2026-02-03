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

    # Track stats with detailed categorization
    stats = {
        'updated': 0,
        'unchanged': 0,
        'critical': [],      # >10x increase
        'warning': [],       # 2x-10x increase
        'decrease': [],      # <50% of old value
        'normal_increase': [], # 1x-2x increase
        'normal_decrease': [], # 50%-100% of old value
        'skipped': []
    }

    # Track all operators we've seen from BigQuery
    seen_operators = set()

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
            stats['skipped'].append(f"{operator} ({platform}/{topology}) - not in JSON")
            continue

        # Track that we've seen this operator
        seen_operators.add((operator, platform, topology))

        # Calculate new limit
        old_limit = limits[topology][platform][operator]
        new_limit = int(round(p99))

        # Skip if unchanged
        if new_limit == old_limit:
            stats['unchanged'] += 1
            continue

        ratio = new_limit / old_limit if old_limit > 0 else 1.0
        pct = ((new_limit - old_limit) / old_limit * 100) if old_limit > 0 else 0

        # Categorize change
        if ratio > 10:
            stats['critical'].append({
                'operator': operator,
                'platform': platform,
                'topology': topology,
                'old': old_limit,
                'new': new_limit,
                'ratio': ratio,
                'pct': pct
            })
        elif ratio >= 2:
            stats['warning'].append({
                'operator': operator,
                'platform': platform,
                'topology': topology,
                'old': old_limit,
                'new': new_limit,
                'ratio': ratio,
                'pct': pct
            })
        elif ratio < 0.5:
            stats['decrease'].append({
                'operator': operator,
                'platform': platform,
                'topology': topology,
                'old': old_limit,
                'new': new_limit,
                'ratio': ratio,
                'pct': pct
            })
        elif new_limit > old_limit:
            stats['normal_increase'].append({
                'operator': operator,
                'platform': platform,
                'topology': topology,
                'old': old_limit,
                'new': new_limit,
                'ratio': ratio,
                'pct': pct
            })
        else:  # new_limit < old_limit but ratio >= 0.5
            stats['normal_decrease'].append({
                'operator': operator,
                'platform': platform,
                'topology': topology,
                'old': old_limit,
                'new': new_limit,
                'ratio': ratio,
                'pct': pct
            })

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

    # Print detailed summary
    print("\n" + "="*80, file=sys.stderr)
    print("OPERATOR WATCH LIMIT CHANGES SUMMARY", file=sys.stderr)
    print("="*80, file=sys.stderr)

    # Critical increases (>10x)
    if stats['critical']:
        print(f"\n🚨 CRITICAL INCREASES (>10x) - {len(stats['critical'])} operators:", file=sys.stderr)
        for item in sorted(stats['critical'], key=lambda x: x['ratio'], reverse=True):
            print(f"  ❌ {item['operator']} ({item['platform']}/{item['topology']}): "
                  f"{item['old']} → {item['new']} ({item['ratio']:.1f}x)", file=sys.stderr)

    # Warning increases (2x-10x)
    if stats['warning']:
        print(f"\n⚠️  WARNING INCREASES (2x-10x) - {len(stats['warning'])} operators:", file=sys.stderr)
        for item in sorted(stats['warning'], key=lambda x: x['ratio'], reverse=True)[:10]:
            print(f"  ⚠️  {item['operator']} ({item['platform']}/{item['topology']}): "
                  f"{item['old']} → {item['new']} ({item['ratio']:.1f}x)", file=sys.stderr)
        if len(stats['warning']) > 10:
            print(f"  ... and {len(stats['warning']) - 10} more", file=sys.stderr)

    # Major decreases (<50%)
    if stats['decrease']:
        print(f"\n📉 MAJOR DECREASES (<50%) - {len(stats['decrease'])} operators:", file=sys.stderr)
        for item in sorted(stats['decrease'], key=lambda x: x['ratio'])[:10]:
            print(f"  📉 {item['operator']} ({item['platform']}/{item['topology']}): "
                  f"{item['old']} → {item['new']} ({item['pct']:.0f}%)", file=sys.stderr)
        if len(stats['decrease']) > 10:
            print(f"  ... and {len(stats['decrease']) - 10} more", file=sys.stderr)

    # Normal increases (1x-2x)
    if stats['normal_increase']:
        print(f"\n✅ NORMAL INCREASES (<2x) - {len(stats['normal_increase'])} operators:", file=sys.stderr)
        for item in stats['normal_increase'][:5]:
            print(f"  ✅ {item['operator']} ({item['platform']}/{item['topology']}): "
                  f"{item['old']} → {item['new']} ({item['pct']:+.0f}%)", file=sys.stderr)
        if len(stats['normal_increase']) > 5:
            print(f"  ... and {len(stats['normal_increase']) - 5} more", file=sys.stderr)

    # Normal decreases (50%-100%)
    if stats['normal_decrease']:
        print(f"\n📊 NORMAL DECREASES (>50%) - {len(stats['normal_decrease'])} operators:", file=sys.stderr)
        for item in stats['normal_decrease'][:5]:
            print(f"  📊 {item['operator']} ({item['platform']}/{item['topology']}): "
                  f"{item['old']} → {item['new']} ({item['pct']:.0f}%)", file=sys.stderr)
        if len(stats['normal_decrease']) > 5:
            print(f"  ... and {len(stats['normal_decrease']) - 5} more", file=sys.stderr)

    # Skipped operators
    if stats['skipped']:
        print(f"\n⏭️  SKIPPED - {len(stats['skipped'])} operators not found in JSON:", file=sys.stderr)
        for item in stats['skipped'][:5]:
            print(f"  ⏭️  {item}", file=sys.stderr)
        if len(stats['skipped']) > 5:
            print(f"  ... and {len(stats['skipped']) - 5} more", file=sys.stderr)

    # Overall summary
    total_operators = len(set(item['operator'] for cat in ['critical', 'warning', 'decrease', 'normal_increase', 'normal_decrease'] for item in stats[cat]))
    print(f"\n{'=' * 80}", file=sys.stderr)
    print(f"TOTAL: {stats['updated']} limits changed across {total_operators} operators", file=sys.stderr)
    print(f"  Critical (>10x): {len(stats['critical'])}", file=sys.stderr)
    print(f"  Warning (2x-10x): {len(stats['warning'])}", file=sys.stderr)
    print(f"  Major decreases (<50%): {len(stats['decrease'])}", file=sys.stderr)
    print(f"  Normal increases (<2x): {len(stats['normal_increase'])}", file=sys.stderr)
    print(f"  Normal decreases (>50%): {len(stats['normal_decrease'])}", file=sys.stderr)
    print(f"  Unchanged: {stats['unchanged']}", file=sys.stderr)
    print(f"  Skipped: {len(stats['skipped'])}", file=sys.stderr)
    print(f"\nLast updated: {limits['_last_updated']}", file=sys.stderr)
    print("=" * 80, file=sys.stderr)

    if stats['critical']:
        print(f"\n⚠️  {len(stats['critical'])} CRITICAL changes detected - review before committing!", file=sys.stderr)
        return 2

    return 0


if __name__ == '__main__':
    sys.exit(main())
