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
1. Runs the Python script `.claude/scripts/query_operator_watch_requests.py` to fetch BigQuery data for the specified release
2. Parses the JSON results to extract Average watch counts per operator/platform/topology
3. Updates ALL existing limits in both `upperBounds` and `upperBoundsSingleNode` maps
4. Highlights operators with dramatic increases (>2x or >10x current limit)
5. Adds a "Last updated" comment to the maps with the current date
6. Validates the file compiles after updates

**Important notes:**
- Uses the **Average** value from BigQuery, NOT P99 or Max (this is safe because the test allows 2x the limit)
- Always queries **last 30 days** of data for reliable averages
- Queries the **openshift-ci-data-analysis** project
- Only updates operators that already exist in the maps (doesn't add new ones automatically)
- Preserves formatting and decimal notation (.0 suffix)
- Highlights concerning increases for manual review before committing

## Implementation

### 1. Fetch BigQuery Data

Executes the Python script to query BigQuery:

```bash
python3 .claude/scripts/query_operator_watch_requests.py 4.22
```

The script:
- Invokes `bq query` via command line
- Queries the `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests` table
- Uses the last 30 days of data for the specified release
- Returns JSON to stdout with operator watch counts

**Query executed by the script:**
```sql
SELECT
  Platform,
  Release,
  CASE
    WHEN ControlPlaneTopology = 'SingleReplica' THEN 'single'
    ELSE 'ha'
  END as Topology,
  Operator,
  ROUND(AVG(WatchRequestCount), 1) as Average,
  MAX(WatchRequestCount) as Max_WatchCount,
  APPROX_QUANTILES(WatchRequestCount, 100)[OFFSET(99)] as P99_WatchCount
FROM `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests`
WHERE Release = '4.22'
  AND Timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY)
  AND Platform IN ('aws', 'azure', 'gcp', 'metal', 'vsphere', 'openstack')
GROUP BY Platform, Release, Topology, Operator
HAVING COUNT(*) >= 5
ORDER BY Platform, Operator
```

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

### 2. Parse and Map Data

**Platform name mapping:**
- `aws` → `configv1.AWSPlatformType`
- `azure` → `configv1.AzurePlatformType`
- `gcp` → `configv1.GCPPlatformType`
- `metal` → `configv1.BareMetalPlatformType`
- `vsphere` → `configv1.VSpherePlatformType`
- `openstack` → `configv1.OpenStackPlatformType`

**Topology mapping:**
- `ha` → Updates `upperBounds` map
- `single` → Updates `upperBoundsSingleNode` map

**Operator name mapping:**
The BigQuery data may have operator names without `-operator` suffix. The command will:
- Ensure the operator name ends with `-operator`
- Match it against existing keys in the maps
- Examples:
  - `cluster-capi-operator` → `cluster-capi-operator` (already correct)
  - `capi-operator` → Skip (not a known service account pattern, may be data issue)
  - `cluster-monitoring` → `cluster-monitoring-operator`

### 3. Update Upper Bounds

For each operator in the BigQuery results:

1. Find the corresponding entry in the appropriate map (upperBounds or upperBoundsSingleNode)
2. Extract the Average value and round to nearest integer with .0 suffix
3. Compare old vs new value:
   - **>10x increase**: Flag as CRITICAL - likely a bug
   - **2x-10x increase**: Flag as WARNING - needs investigation
   - **<2x increase**: Normal - update without warning
4. Update the map entry with the new value

### 4. Add Update Timestamp

Updates the comment at the top of each map to include:
```go
// Last updated: 2026-02-02 from BigQuery CI data
```

### 5. Generate Report

Creates a summary report showing:
- Total operators updated
- Operators with critical increases (>10x)
- Operators with warnings (2x-10x)
- Operators with normal updates (<2x)
- Any operators in the maps that had no BigQuery data

### 6. Validation

- Ensures the file compiles: `go build ./pkg/monitortests/kubeapiserver/auditloganalyzer/...`
- Verifies formatting is preserved
- Lists all changes for review

## Return Value

- **Claude agent text**: Detailed report of all updates with warnings
- **Side effects**:
  - Modified file: `pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go`
  - All operator limits updated from BigQuery data
  - Update timestamp added to maps
  - Git changes ready for review and commit

## Example

**Usage:**
```
/update-all-operator-watch-request-limits 4.22
```

**Output:**
```
Running BigQuery query for release 4.22 (last 30 days of data)...
Project: openshift-ci-data-analysis
✅ Query completed, processing 234 results

Updating operator watch request limits from BigQuery data...

CRITICAL - Operators with >10x increase:
❌ cluster-monitoring-operator on AWS (HA): 186 → 2100 (11.3x increase)
   This likely indicates a bug - investigate before committing!

WARNING - Operators with 2x-10x increase:
⚠️  marketplace-operator on GCP (HA): 19 → 45 (2.4x increase)
⚠️  ingress-operator on Azure (HA): 541 → 890 (1.6x increase - just under 2x)

Updated operators (normal increases):
✅ cluster-capi-operator on AWS (HA): 205 → 220 (7% increase)
✅ cluster-capi-operator on Azure (HA): 210 → 215 (2% increase)
✅ authentication-operator on AWS (HA): 519 → 530 (2% increase)
... (120 more updates)

Operators in maps with no BigQuery data:
ℹ️  operator on AWS (HA): 49 (keeping current value)
ℹ️  vsphere-problem-detector-operator on vSphere (HA): 52 (keeping current value)

Summary:
- Total operators updated: 123
- Critical increases (>10x): 1
- Warning increases (2x-10x): 2
- Normal updates: 120
- No data (kept current): 8

Updated maps with timestamp: 2026-02-02

⚠️  REVIEW REQUIRED: 1 critical increase and 2 warnings need investigation before committing.

File updated: pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go
Run `git diff` to review all changes.
```

## Arguments

- **$1** (required): OpenShift release version
  - Format: `X.YY` (e.g., `4.22`, `4.23`, `4.21`)
  - This will be used in the BigQuery query to filter data for that specific release
  - The query will fetch the last 30 days of data for this release

## BigQuery Query Details

The command uses the Python script `.claude/scripts/query_operator_watch_requests.py` to fetch data. The script executes this query:

```sql
SELECT
  Platform,
  Release,
  CASE
    WHEN ControlPlaneTopology = 'SingleReplica' THEN 'single'
    ELSE 'ha'
  END as Topology,
  Operator,
  ROUND(AVG(WatchRequestCount), 1) as Average,
  MAX(WatchRequestCount) as Max_WatchCount,
  APPROX_QUANTILES(WatchRequestCount, 100)[OFFSET(99)] as P99_WatchCount
FROM `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests`
WHERE Release = '<RELEASE>'  -- Substituted from command argument
  AND Timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY)  -- Always 30 days
  AND Platform IN ('aws', 'azure', 'gcp', 'metal', 'vsphere', 'openstack')
GROUP BY Platform, Release, Topology, Operator
HAVING COUNT(*) >= 5  -- Require at least 5 data points for reliable average
ORDER BY Platform, Operator
```

**Query characteristics:**
- **Project**: `openshift-ci-data-analysis`
- **Dataset**: `ci_data_autodl.operator_watch_requests`
- **Time window**: Fixed at 30 days (provides reliable averages)
- **Minimum samples**: 5 data points required per operator/platform/topology
- **Platforms**: aws, azure, gcp, metal (baremetal), vsphere, openstack

## Notes

- **Uses Python script**: Delegates BigQuery query to `.claude/scripts/query_operator_watch_requests.py` for easier debugging and re-running
- **Requires `bq` CLI**: Google Cloud SDK must be installed and authenticated (`gcloud auth login`)
- **BigQuery project**: Always queries `openshift-ci-data-analysis` project
- **Dataset**: Uses `ci_data_autodl.operator_watch_requests` table
- **Fixed time window**: Always uses 30 days of data (not configurable - ensures reliable averages)
- **Release specific**: Only queries data for the specified OpenShift release version
- **Review before commit**: Always review critical and warning increases before committing
- **Operator names**: The BigQuery Operator field should match service account names (with or without `-operator` suffix)
- **Platform names**: Must use lowercase names that match the BigQuery schema (aws, gcp, metal, azure, vsphere, openstack)
- **Uses Average**: Intentionally uses Average instead of P99/Max because test allows 2x headroom
- **Preserves unknowns**: Operators not in BigQuery results keep their current values
- **Formatting**: Maintains .0 decimal notation and indentation
- **Minimum samples**: Requires at least 5 data points per operator/platform/topology for inclusion
- **Re-runnable query**: You can re-run just the BigQuery query with `python3 .claude/scripts/query_operator_watch_requests.py 4.22`

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

- Python script: `.claude/scripts/query_operator_watch_requests.py` - Run just the BigQuery query (used by this command)
- `/update-operator-watch-request-limits` - Update a single operator limit manually
- Test implementation: `pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go`
- BigQuery dataset: `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests`
