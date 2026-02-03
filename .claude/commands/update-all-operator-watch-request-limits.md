---
description: Bulk update all operator watch request limits from BigQuery data
argument-hint: <release>
---

## Name

update-all-operator-watch-request-limits

## Synopsis

```
/update-all-operator-watch-request-limits <release>
```

## Description

The `update-all-operator-watch-request-limits` command performs a bulk update of ALL operator watch request limits in the test by querying live data from BigQuery. This is used to refresh all limits at once based on recent CI data (last 30 days), rather than updating operators one at a time.

**What this command does:**
1. Runs `.claude/scripts/query_operator_watch_requests.py` to fetch BigQuery data for the specified release
2. Pipes the JSON results to `.claude/scripts/update_operator_watch_limits.py` to update the JSON file
3. Updates ALL existing limits in the JSON file for both topologies (HighlyAvailable and SingleReplica)
4. Shows a categorized summary of all changes as they're applied:
   - Critical increases (>10x) - likely bugs
   - Warning increases (2x-10x) - need investigation
   - Major decreases (<50%) - significant reductions
   - Normal increases/decreases - expected changes

**Important notes:**
- Uses the P99 over the past 30 days. Note the test allows 2x this limit before it will complain allowing for natural growth.
- Only updates operators that already exist in the JSON (doesn't add new ones automatically)
- Updates `operator_watch_limits.json` directly 
- Preserves JSON formatting and structure
- Highlights concerning increases for manual review before committing

## Implementation

### 1. Fetch BigQuery Data and Update Limits

Executes the query script and pipes output to the update script:

```bash
# Query BigQuery and pipe directly to update script
python3 .claude/scripts/query_operator_watch_requests.py "$RELEASE" | \
  python3 .claude/scripts/update_operator_watch_limits.py
```

The query script:
- Invokes `bq query` via command line with `--max_rows=1000` to handle all results (up from default 100)
- Queries the `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests` table
- Uses the last 30 days of data for the specified release
- Outputs JSON to stdout (piped to update script)

The update script:
- Reads JSON from stdin
- Updates operator_watch_limits.json with new values
- Tracks all changes as it processes them
- Categorizes changes by severity (critical, warning, decreases, normal)
- Outputs a comprehensive summary at the end

**Expected JSON results:**
```json
[{
  "Platform": "aws",
  "Release": "4.22",
  "Topology": "ha",
  "Operator": "cluster-capi-operator",
  "Average": "117.0",
  "Max_WatchCount": "216",
  "P99_WatchCount": "199"
}, ...]
```

### 2. Summarize Changes

After updating the JSON file, summarize the changes using git diff:

```bash
# Show summary of what changed
python3 .claude/scripts/summarize_operator_watch_changes.py
```

The summarize script:
- Parses `git diff` of operator_watch_limits.json
- Categorizes changes: critical (>10x), warning (2x-10x), decreases (<50%), normal (<2x)
- Provides a human-readable summary of all changes

The update script accepts JSON from stdin and:
- Updates operator_watch_limits.json directly
- Preserves JSON structure and formatting
- Updates the `_last_updated` timestamp

### 3. Parse and Map Data

**Platform name mapping:**
- `aws` → `AWS` (in JSON)
- `azure` → `Azure` (in JSON)
- `gcp` → `GCP` (in JSON)
- `metal` → `BareMetal` (in JSON)
- `vsphere` → `vSphere` (in JSON)
- `openstack` → `OpenStack` (in JSON)

**Topology mapping:**
- `ha` → `HighlyAvailable` (in JSON)
- `single` → `SingleReplica` (in JSON)

**Operator name mapping:**
The BigQuery data may have operator names without `-operator` suffix. The command will:
- Ensure the operator name ends with `-operator`
- Match it against existing keys in the maps
- Examples:
  - `cluster-capi-operator` → `cluster-capi-operator` (already correct)
  - `capi-operator` → Skip (not a known service account pattern, may be data issue)
  - `cluster-monitoring` → `cluster-monitoring-operator`

### 4. Update Limits in JSON

For each operator in the BigQuery results:

1. Find the corresponding entry in the JSON file under the appropriate topology and platform
2. Extract the P99 value and round to nearest integer
3. Compare with old value and categorize the change
4. Update the JSON entry with the new value
5. Track the change for the summary report

The update script categorizes all changes as it processes them:
- **Critical increases (>10x)** - likely bugs, need investigation
- **Warning increases (2x-10x)** - need explanation
- **Major decreases (<50%)** - significant reductions in watch usage
- **Normal increases (<2x)** - expected growth
- **Normal decreases (>50%)** - minor reductions
- **Unchanged** - P99 matches current limit
- **Skipped** - operators in BigQuery but not in JSON

After processing all changes:
- Updates the `_last_updated` timestamp to today's date
- Outputs a comprehensive summary showing all categorized changes

### 5. Validation

After updating, validate the changes:
- Review the comprehensive summary output for concerning changes
- Pay special attention to critical (>10x) and warning (2x-10x) increases
- If needed, run `git diff pkg/monitortests/kubeapiserver/auditloganalyzer/operator_watch_limits.json` to see raw changes
- Ensure the code still compiles: `go build ./pkg/monitortests/kubeapiserver/auditloganalyzer/...`
- Review and investigate critical/warning increases before committing

## Return Value

- **Claude agent text**: Detailed report of all updates with warnings and summary of changes
- **Side effects**:
  - Modified file: `pkg/monitortests/kubeapiserver/auditloganalyzer/operator_watch_limits.json`
  - All operator limits updated from BigQuery data
  - Update timestamp added to JSON (`_last_updated`)
  - Git changes ready for review and commit

## Example

**Usage:**
```
/update-all-operator-watch-request-limits 4.22
```

**Output:**
```
Querying BigQuery for release 4.22 (last 30 days)...
Project: openshift-ci-data-analysis
Dataset: ci_data_autodl.operator_watch_requests
✅ Query completed, retrieved 250 results

Reading BigQuery data from stdin...
Loading: pkg/monitortests/kubeapiserver/auditloganalyzer/operator_watch_limits.json
Writing: pkg/monitortests/kubeapiserver/auditloganalyzer/operator_watch_limits.json

================================================================================
OPERATOR WATCH LIMIT CHANGES SUMMARY
================================================================================

🚨 CRITICAL INCREASES (>10x) - 1 operators:
  ❌ cluster-monitoring-operator (AWS/HighlyAvailable): 186 → 2100 (11.3x)

⚠️  WARNING INCREASES (2x-10x) - 2 operators:
  ⚠️  marketplace-operator (GCP/HighlyAvailable): 45 → 95 (2.1x)
  ⚠️  ingress-operator (Azure/HighlyAvailable): 541 → 890 (1.6x)

📉 MAJOR DECREASES (<50%) - 1 operators:
  📉 dns-operator (vSphere/HighlyAvailable): 120 → 55 (-54%)

✅ NORMAL INCREASES (<2x) - 115 operators:
  ✅ cluster-capi-operator (AWS/HighlyAvailable): 205 → 220 (+7%)
  ✅ authentication-operator (AWS/HighlyAvailable): 519 → 530 (+2%)
  ✅ etcd-operator (AWS/HighlyAvailable): 245 → 252 (+3%)
  ✅ console-operator (Azure/HighlyAvailable): 212 → 218 (+3%)
  ✅ kube-apiserver-operator (GCP/HighlyAvailable): 260 → 275 (+6%)
  ... and 110 more

📊 NORMAL DECREASES (>50%) - 3 operators:
  📊 prometheus-operator (OpenStack/HighlyAvailable): 125 → 118 (-6%)
  📊 service-ca-operator (BareMetal/HighlyAvailable): 135 → 130 (-4%)
  📊 cluster-samples-operator (Azure/SingleReplica): 40 → 38 (-5%)
  ... and 0 more

⏭️  SKIPPED - 2 operators not found in JSON:
  ⏭️  new-test-operator (AWS/HighlyAvailable) - not in JSON
  ⏭️  experimental-operator (GCP/HighlyAvailable) - not in JSON

================================================================================
TOTAL: 122 limits changed across 120 operators
  Critical (>10x): 1
  Warning (2x-10x): 2
  Major decreases (<50%): 1
  Normal increases (<2x): 115
  Normal decreases (>50%): 3
  Unchanged: 78
  Skipped: 2

Last updated: 2026-02-03
================================================================================

⚠️  1 CRITICAL changes detected - review before committing!
```

## Arguments

- **$1** (required): OpenShift release version
  - Format: `X.YY` (e.g., `4.22`, `4.23`, `4.21`)
  - This will be used in the BigQuery query to filter data for that specific release
  - The query will fetch the last 30 days of data for this release

## BigQuery Query Details

The command uses the Python script `.claude/scripts/query_operator_watch_requests.py` to fetch data. 

## Notes

- **Uses Python scripts**: Two scripts work together:
  - `query_operator_watch_requests.py` - Queries BigQuery
  - `update_operator_watch_limits.py` - Updates JSON file and provides comprehensive summary
- **Requires `bq` CLI**: Google Cloud SDK must be installed and authenticated (`gcloud auth login`)
- **BigQuery project**: Always queries `openshift-ci-data-analysis` project
- **Dataset**: Uses `ci_data_autodl.operator_watch_requests` table
- **Max rows**: Query limited to 1000 results (up from default 100) to handle all operator/platform/topology combinations
- **Fixed time window**: Always uses 30 days of data (not configurable - ensures reliable averages)
- **Release specific**: Only queries data for the specified OpenShift release version
- **Review before commit**: Always review critical and warning increases before committing
- **Operator names**: The BigQuery Operator field should match service account names (with or without `-operator` suffix)
- **Platform names**: Must use lowercase names that match the BigQuery schema (aws, gcp, metal, azure, vsphere, openstack)
- **Uses Average**: Intentionally uses Average instead of P99/Max because test allows 2x headroom
- **Preserves unknowns**: Operators not in BigQuery results keep their current values
- **JSON structure**: Updates `operator_watch_limits.json` directly, organized by topology → platform → operator
- **Formatting**: Maintains JSON indentation and structure
- **Re-runnable query**: You can re-run just the BigQuery query with `python3 .claude/scripts/query_operator_watch_requests.py 4.22`
- **Comprehensive summary**: The update script tracks and categorizes all changes as it processes them, providing a detailed summary at the end

## Error Handling

- **Missing release parameter**: Prompts user to provide release version (e.g., `4.22`)
- **Invalid release format**: Validates release matches pattern like `4.22` or `4.23`
- **BigQuery query fails**: Shows error message and exits without modifying files
- **bq CLI not available**: Checks for `bq` command and provides installation instructions
- **Authentication required**: Prompts to run `gcloud auth login` if not authenticated
- **Invalid JSON**: Reports parsing errors with line numbers
- **Missing fields**: Warns about records missing Platform/Topology/Operator/Average
- **Unknown platform**: Warns and skips operators on platforms not in the maps
- **Operator not found**: Notes operators in BigQuery data that don't exist in maps (won't add them)
- **Compilation fails**: Reverts changes and reports Go build errors
- **No data**: If query returns empty results, exits without changes and suggests checking the release version

## Warnings

- ⚠️  **Critical increases (>10x)** almost always indicate bugs - investigate before committing
- ⚠️  **Large increases (2x-10x)** need explanation - check recent code changes
- ⚠️  **Review the diff** - ensure formatting is preserved and changes make sense
- ⚠️  **Test compilation** - the command validates but always verify manually
- ⚠️  **Single node data** - upperBoundsSingleNode only has AWS platform, query may return data for other platforms that can't be used

## See Also

- Python scripts:
  - `.claude/scripts/query_operator_watch_requests.py` - Fetch BigQuery data for a release
  - `.claude/scripts/update_operator_watch_limits.py` - Update limits in JSON file and provide summary
- `/update-operator-watch-request-limits` - Update a single operator limit manually
- Limits file: `pkg/monitortests/kubeapiserver/auditloganalyzer/operator_watch_limits.json`
- Test implementation: `pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go`
- BigQuery dataset: `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests`
