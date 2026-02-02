---
description: Bulk update all operator watch request limits from BigQuery data
argument-hint: <bigquery-query>
---

## Name

update-all-operator-watch-request-limits

## Synopsis

```
/update-all-operator-watch-request-limits <bigquery-query>
```

## Description

The `update-all-operator-watch-request-limits` command performs a bulk update of ALL operator watch request limits in the test by querying live data from BigQuery. This is used to refresh all limits at once based on recent CI data, rather than updating operators one at a time.

**What this command does:**
1. Runs the provided BigQuery query to get recent watch request data
2. Parses the JSON results to extract Average watch counts per operator/platform/topology
3. Updates ALL existing limits in both `upperBounds` and `upperBoundsSingleNode` maps
4. Highlights operators with dramatic increases (>2x or >10x current limit)
5. Adds a "Last updated" comment to the maps with the current date
6. Validates the file compiles after updates

**Important notes:**
- Uses the **Average** value from BigQuery, NOT P99 or Max (this is safe because the test allows 2x the limit)
- Only updates operators that already exist in the maps (doesn't add new ones automatically)
- Preserves formatting and decimal notation (.0 suffix)
- Highlights concerning increases for manual review before committing

## Implementation

### 1. Run BigQuery Query

Executes the provided BigQuery query using `bq` CLI:
```bash
bq query --format=json --use_legacy_sql=false '<query>'
```

**Expected query results format:**
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
/update-all-operator-watch-request-limits "
SELECT
  Platform,
  Release,
  Topology,
  Operator,
  ROUND(AVG(WatchRequestCount), 1) as Average,
  MAX(WatchRequestCount) as Max_WatchCount,
  APPROX_QUANTILES(WatchRequestCount, 100)[OFFSET(99)] as P99_WatchCount
FROM \`openshift-ci-data-analysis.ci_data.operator_watch_requests\`
WHERE Release = '4.22'
  AND Timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 7 DAY)
GROUP BY Platform, Release, Topology, Operator
ORDER BY Platform, Operator
"
```

**Output:**
```
Running BigQuery query...
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

## BigQuery Query Template

Here's a template query to use with this command:

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
FROM `openshift-ci-data-analysis.ci_data.operator_watch_requests`
WHERE Release = '4.22'
  AND Timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 7 DAY)
  AND Platform IN ('aws', 'azure', 'gcp', 'metal', 'vsphere', 'openstack')
GROUP BY Platform, Release, Topology, Operator
HAVING COUNT(*) >= 5  -- Require at least 5 data points
ORDER BY Platform, Operator
```

**Query parameters to adjust:**
- `Release = '4.22'`: Target OpenShift version
- `INTERVAL 7 DAY`: Time window for data (7 days recommended)
- `COUNT(*) >= 5`: Minimum samples required for reliable average

## Notes

- **Requires `bq` CLI**: Google Cloud SDK must be installed and authenticated
- **BigQuery permissions**: Need read access to the CI data analysis project
- **Review before commit**: Always review critical and warning increases before committing
- **Operator names**: The BigQuery Operator field should match service account names (with or without `-operator` suffix)
- **Platform names**: Must use lowercase names that match the BigQuery schema
- **Uses Average**: Intentionally uses Average instead of P99/Max because test allows 2x headroom
- **Preserves unknowns**: Operators not in BigQuery results keep their current values
- **Formatting**: Maintains .0 decimal notation and indentation

## Error Handling

- **BigQuery query fails**: Shows error message and exits without modifying files
- **Invalid JSON**: Reports parsing errors with line numbers
- **Missing fields**: Warns about records missing Platform/Topology/Operator/Average
- **Unknown platform**: Warns and skips operators on platforms not in the maps
- **Operator not found**: Notes operators in BigQuery data that don't exist in maps (won't add them)
- **Compilation fails**: Reverts changes and reports Go build errors
- **No data**: If query returns empty results, exits without changes

## Warnings

- ⚠️  **Critical increases (>10x)** almost always indicate bugs - investigate before committing
- ⚠️  **Large increases (2x-10x)** need explanation - check recent code changes
- ⚠️  **Review the diff** - ensure formatting is preserved and changes make sense
- ⚠️  **Test compilation** - the command validates but always verify manually
- ⚠️  **Single node data** - upperBoundsSingleNode only has AWS platform, query may return data for other platforms that can't be used

## See Also

- `/update-operator-watch-request-limits` - Update a single operator limit manually
- Test implementation: `pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go`
- BigQuery dataset: `openshift-ci-data-analysis.ci_data.operator_watch_requests`
