#!/usr/bin/env python3
"""
Summarize changes to operator watch limits from git diff.

This script parses the git diff of operator_watch_limits.json and provides
a human-readable summary of which operators changed and by how much.

Usage:
    python3 summarize_operator_watch_changes.py

Output:
    Summary of operator limit changes with categorization
"""

import sys
import subprocess
import json
import re


def parse_git_diff():
    """Parse git diff output for operator_watch_limits.json."""
    try:
        result = subprocess.run(
            ['git', 'diff', 'pkg/monitortests/kubeapiserver/auditloganalyzer/operator_watch_limits.json'],
            capture_output=True,
            text=True,
            check=True
        )
        return result.stdout
    except subprocess.CalledProcessError as e:
        print(f"❌ Error running git diff: {e}", file=sys.stderr)
        return None


def extract_changes(diff_output):
    """Extract operator limit changes from git diff output.

    The diff shows entire platform sections being removed and re-added as blocks.
    We track the current topology/platform context from context lines to properly
    label each change.
    """
    changes = []
    current_topology = None
    current_platform = None

    # Track removed values with context
    pending_removals = {}  # key: (op, platform, topology) -> old_value

    lines = diff_output.split('\n')

    for line in lines:
        # Track topology context (lines without +/- prefix)
        if '"HighlyAvailable"' in line and not line.startswith('+') and not line.startswith('-'):
            current_topology = 'HighlyAvailable'
        elif '"SingleReplica"' in line and not line.startswith('+') and not line.startswith('-'):
            current_topology = 'SingleReplica'

        # Track platform context (lines without +/- prefix)
        for platform in ['AWS', 'Azure', 'GCP', 'BareMetal', 'vSphere', 'OpenStack']:
            if f'"{platform}"' in line and not line.startswith('+') and not line.startswith('-'):
                current_platform = platform
                break

        # Match removed operators
        removed = re.match(r'-\s+"([a-z-]+)":\s+(\d+)', line)
        if removed and current_topology and current_platform:
            op_name = removed.group(1)
            old_value = int(removed.group(2))
            key = (op_name, current_platform, current_topology)
            pending_removals[key] = old_value

        # Match added operators and pair with removals
        added = re.match(r'\+\s+"([a-z-]+)":\s+(\d+)', line)
        if added and current_topology and current_platform:
            op_name = added.group(1)
            new_value = int(added.group(2))
            key = (op_name, current_platform, current_topology)

            if key in pending_removals:
                old_value = pending_removals[key]
                if old_value != new_value:
                    changes.append({
                        'operator': op_name,
                        'platform': current_platform,
                        'topology': current_topology,
                        'old': old_value,
                        'new': new_value
                    })
                del pending_removals[key]
            else:
                # New operator added
                changes.append({
                    'operator': op_name,
                    'platform': current_platform,
                    'topology': current_topology,
                    'old': 0,
                    'new': new_value
                })

    return changes


def categorize_change(old_value, new_value):
    """Categorize the change based on ratio and return category and metrics."""
    if old_value == 0:
        return 'new', float('inf'), 0

    ratio = new_value / old_value
    pct_change = ((new_value - old_value) / old_value) * 100

    if ratio > 10:
        return 'critical', ratio, pct_change
    elif ratio >= 2:
        return 'warning', ratio, pct_change
    elif ratio < 0.5:
        return 'decrease', ratio, pct_change
    else:
        return 'normal', ratio, pct_change


def main():
    print("Analyzing operator watch limit changes...\n", file=sys.stderr)

    diff_output = parse_git_diff()
    if not diff_output:
        print("❌ No git diff output found", file=sys.stderr)
        return 1

    if not diff_output.strip():
        print("✅ No changes detected in operator_watch_limits.json", file=sys.stderr)
        return 0

    changes = extract_changes(diff_output)

    if not changes:
        print("⚠️  Git diff detected but no operator limit changes parsed", file=sys.stderr)
        print("This might indicate a formatting-only change", file=sys.stderr)
        return 0

    # Categorize changes
    categorized = {
        'critical': [],
        'warning': [],
        'decrease': [],
        'normal': [],
        'new': []
    }

    for change in changes:
        category, ratio, pct_change = categorize_change(change['old'], change['new'])
        change['ratio'] = ratio
        change['pct_change'] = pct_change
        categorized[category].append(change)

    # Print summary
    print("=" * 80, file=sys.stderr)
    print("OPERATOR WATCH LIMIT CHANGES SUMMARY", file=sys.stderr)
    print("=" * 80, file=sys.stderr)

    if categorized['critical']:
        print(f"\n🚨 CRITICAL INCREASES (>10x) - {len(categorized['critical'])} entries:", file=sys.stderr)
        for c in categorized['critical']:
            print(f"  ❌ {c['operator']} ({c['platform']}/{c['topology']}): {c['old']} → {c['new']} ({c['ratio']:.1f}x)", file=sys.stderr)

    if categorized['warning']:
        print(f"\n⚠️  WARNING INCREASES (2x-10x) - {len(categorized['warning'])} entries:", file=sys.stderr)
        for c in sorted(categorized['warning'], key=lambda x: x['ratio'], reverse=True)[:10]:
            print(f"  ⚠️  {c['operator']} ({c['platform']}/{c['topology']}): {c['old']} → {c['new']} ({c['ratio']:.1f}x)", file=sys.stderr)
        if len(categorized['warning']) > 10:
            print(f"  ... and {len(categorized['warning']) - 10} more", file=sys.stderr)

    if categorized['decrease']:
        print(f"\n📉 MAJOR DECREASES (<50%) - {len(categorized['decrease'])} entries:", file=sys.stderr)
        for c in sorted(categorized['decrease'], key=lambda x: x['ratio'])[:10]:
            print(f"  📉 {c['operator']} ({c['platform']}/{c['topology']}): {c['old']} → {c['new']} ({c['pct_change']:.0f}%)", file=sys.stderr)
        if len(categorized['decrease']) > 10:
            print(f"  ... and {len(categorized['decrease']) - 10} more", file=sys.stderr)

    if categorized['normal']:
        print(f"\n✅ NORMAL UPDATES (<2x) - {len(categorized['normal'])} entries:", file=sys.stderr)
        # Show a sample
        for c in categorized['normal'][:5]:
            print(f"  ✅ {c['operator']} ({c['platform']}/{c['topology']}): {c['old']} → {c['new']} ({c['pct_change']:+.0f}%)", file=sys.stderr)
        if len(categorized['normal']) > 5:
            print(f"  ... and {len(categorized['normal']) - 5} more", file=sys.stderr)

    if categorized['new']:
        print(f"\n🆕 NEW OPERATORS - {len(categorized['new'])} entries:", file=sys.stderr)
        for c in categorized['new']:
            print(f"  🆕 {c['operator']} ({c['platform']}/{c['topology']}): {c['new']}", file=sys.stderr)

    # Overall summary
    total_changes = len(changes)
    unique_operators = len(set(c['operator'] for c in changes))
    print(f"\n{'=' * 80}", file=sys.stderr)
    print(f"TOTAL: {total_changes} entries changed across {unique_operators} operators", file=sys.stderr)
    print(f"  Critical (>10x): {len(categorized['critical'])}", file=sys.stderr)
    print(f"  Warning (2x-10x): {len(categorized['warning'])}", file=sys.stderr)
    print(f"  Major decreases (<50%): {len(categorized['decrease'])}", file=sys.stderr)
    print(f"  Normal updates (<2x): {len(categorized['normal'])}", file=sys.stderr)
    print(f"  New: {len(categorized['new'])}", file=sys.stderr)
    print("=" * 80, file=sys.stderr)

    # Return non-zero if critical changes need review
    if categorized['critical']:
        print(f"\n⚠️  {len(categorized['critical'])} CRITICAL changes detected - review before committing!", file=sys.stderr)
        return 2

    return 0


if __name__ == '__main__':
    sys.exit(main())
