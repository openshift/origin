---
description: Update watch request limits for operators in the apiserver watch count tracking test
argument-hint: <operator-name> <platform> <new-limit> [--topology=HA|single]
---

## Name

update-operator-watch-request-limits

## Synopsis

```
/update-operator-watch-request-limits <operator-name> <platform> <new-limit> [--topology=HA|single]
```

## Description

The `update-operator-watch-request-limits` command updates the upper bounds for watch request counts in the test "[sig-arch][Late] operators should not create watch channels very often". This test monitors the number of watch requests created by operators to detect explosive growth in watch channel usage, which can endanger the kube-apiserver and usually indicates a bug.

The test maintains per-operator, per-platform limits that need periodic adjustments as operators evolve. When the test fails, it typically means watch request counts have gradually increased and need a small bump, provided the increase is not excessive (10x or more) and is explainable.

This command simplifies the maintenance of these limits by:
- Locating the appropriate bounds map (HA or single-node topology)
- Updating the limit for the specified operator and platform
- Validating the change is reasonable
- Creating a commit with the update

## Implementation

The command executes the following workflow:

### 1. Locate the Test File

Finds the test implementation at:
```
pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go
```

### 2. Identify the Bounds Map

Determines which map to update based on topology:
- **HA (default)**: Updates the `upperBounds` map (lines 177-356)
- **single**: Updates the `upperBoundsSingleNode` map (lines 358-387)

The maps are structured as:
```go
upperBounds := map[configv1.PlatformType]platformUpperBound{
    configv1.AWSPlatformType: {
        "marketplace-operator": 52.0,
        "ingress-operator": 556.0,
        ...
    },
    ...
}
```

### 3. Validate Input

- Verifies operator name format (should end in `-operator`)
- Validates platform is one of: AWS, Azure, GCP, BareMetal, vSphere, OpenStack
- Checks that the new limit is reasonable (not more than 10x current value)
- Confirms the platform exists in the selected bounds map
- Warns if increase is >50% and prompts for confirmation

### 4. Parse and Update the Limit

To update the limit:

1. Read the test file
2. Find the appropriate bounds map (upperBounds or upperBoundsSingleNode)
3. Locate the platform section within the map
4. Find the operator entry: `"operator-name": <current-value>,`
5. Replace with: `"operator-name": <new-value>,`
6. Preserve all formatting, indentation, and decimal notation (.0 suffix)

### 5. Validation

- Ensures the file compiles after the change: `go build ./pkg/monitortests/kubeapiserver/auditloganalyzer/...`
- Verifies the update was successful by reading back the value

## Return Value

- **Claude agent text**: Success message with old and new limits, percentage increase
- **Side effects**:
  - Modified file: `pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go`
  - Git changes ready for commit

## Examples

1. **Update marketplace-operator limit on AWS (HA topology)**:

   ```
   /update-operator-watch-request-limits marketplace-operator AWS 60
   ```

   Output:
   ```
   Updating marketplace-operator watch request limit for AWS (HA topology)
   Current limit: 52
   New limit: 60
   Increase: +15.4%

   ✅ Updated successfully in pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go

   The change is ready to commit. Use `git diff` to review.
   ```

2. **Update for single-node topology**:

   ```
   /update-operator-watch-request-limits ingress-operator AWS 700 --topology=single
   ```

   Output:
   ```
   Updating ingress-operator watch request limit for AWS (single-node topology)
   Current limit: 640
   New limit: 700
   Increase: +9.4%

   ✅ Updated successfully
   ```

3. **Update for Azure platform**:

   ```
   /update-operator-watch-request-limits cluster-monitoring-operator Azure 200
   ```

   Output:
   ```
   Updating cluster-monitoring-operator watch request limit for Azure (HA topology)
   Current limit: 191
   New limit: 200
   Increase: +4.7%

   ✅ Updated successfully
   ```

4. **Warning for large increase**:

   ```
   /update-operator-watch-request-limits etcd-operator GCP 500
   ```

   Output:
   ```
   ⚠️  WARNING: Large increase detected
   Current limit: 220
   New limit: 500
   Increase: +127.3%

   This is a significant increase. Have you:
   1. Investigated the root cause?
   2. Confirmed this is not a bug?
   3. Verified recent code changes explain this increase?

   Proceed with update? [y/N]
   ```

## Arguments

- **$1** (required): Operator name
  - Format: `{name}-operator` (e.g., `marketplace-operator`)
  - Must match an existing operator in the bounds map
  - Examples: `marketplace-operator`, `ingress-operator`, `etcd-operator`
  - Common operators:
    - `marketplace-operator`
    - `ingress-operator`
    - `kube-apiserver-operator`
    - `etcd-operator`
    - `cluster-monitoring-operator`
    - `authentication-operator`

- **$2** (required): Platform type
  - One of: `AWS`, `Azure`, `GCP`, `BareMetal`, `vSphere`, `OpenStack`
  - Case-sensitive, must match exactly as it appears in the test
  - Platform must have limits defined in the test
  - Note: Not all platforms have single-node limits defined

- **$3** (required): New limit value
  - Must be a positive integer or decimal with .0 suffix
  - Should not be more than 10x the current value
  - Represents the expected maximum watch request count per hour
  - The actual enforced limit is 2x this value (to account for bucket boundaries)

- **--topology** (optional): Topology mode (default: `HA`)
  - `HA`: Updates limits for high-availability clusters (default, 3+ control plane nodes)
  - `single`: Updates limits for single-node clusters
  - Single-node limits are typically lower than HA limits
  - Currently only AWS has single-node limits defined

## Error Handling

The command handles common error cases:

- **Operator not found**: Lists available operators for the platform and topology
  ```
  Error: Operator "foo-operator" not found in AWS (HA) limits

  Available operators for AWS (HA):
  - authentication-operator (519)
  - aws-ebs-csi-driver-operator (199)
  - marketplace-operator (52)
  ...
  ```

- **Platform not found**: Lists available platforms
  ```
  Error: Platform "Foo" not recognized

  Valid platforms: AWS, Azure, GCP, BareMetal, vSphere, OpenStack
  ```

- **Platform not available for topology**: Suggests alternatives
  ```
  Error: Single-node limits not defined for Azure

  Platforms with single-node limits: AWS

  Use --topology=HA for Azure, or add single-node limits for Azure.
  ```

- **Excessive increase**: Warns if new limit is >10x current value and requires confirmation
  ```
  Error: New limit (5000) is more than 10x the current limit (220)

  This suggests a probable bug. Please investigate before updating.
  If this increase is legitimate, update the file manually.
  ```

- **Invalid topology**: Shows valid topology options
  ```
  Error: Invalid topology "foo"

  Valid topologies: HA, single
  ```

- **File not found**: Provides path to expected test location
  ```
  Error: Test file not found at expected location:
  pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go

  Please ensure you're running this command from the origin repository root.
  ```

- **Compilation error**: Displays Go build errors after update
  ```
  Error: File does not compile after update

  Build error:
  syntax error: unexpected newline

  The update has been reverted. Please check the file manually.
  ```

## Understanding the Test

The test "[sig-arch][Late] operators should not create watch channels very often" serves as a canary for detecting watch request growth that could impact cluster performance:

### Why This Test Exists

- **API Server Protection**: Excessive watch requests can overload the kube-apiserver, causing performance degradation or outages
- **Bug Detection**: Sudden spikes in watch requests often indicate operator bugs like watch restarts in tight loops
- **Performance Monitoring**: Gradual growth helps track operator efficiency over time and identify optimization opportunities
- **Capacity Planning**: Helps understand watch request patterns across different platforms and topologies

### How Limits Are Calculated

The upper bounds are measured from CI runs where tests might run less than 2 hours. To account for bucket boundary effects (watch requests might be split across hour boundaries), the actual limit enforced in the test is `upperBound * 2`.

The test:
1. Parses audit logs for all `verb=watch` events from operator users (usernames ending in `-operator`)
2. Groups watch requests by operator, node, and hour
3. Counts completed watch requests (`stage=ResponseComplete`)
4. Takes the maximum count across all hours and nodes for each operator
5. Compares the maximum against `upperBound * 2` for the operator on the current platform

### When Failures Are Expected

Test failures are normal when:
- New operator functionality legitimately requires more watches (e.g., watching new resource types)
- Kubernetes version upgrades change watch behavior (e.g., new fields trigger re-watches)
- Platform-specific features add new watch requirements (e.g., cloud provider integrations)
- Operator refactoring changes watch patterns (e.g., consolidating controllers)

### When Failures Require Investigation

Failures require investigation when:
- **The increase is >10x**: This almost always indicates a bug (e.g., watch restart loop)
- **The increase is unexplained**: No recent code changes to the operator
- **Multiple operators suddenly increase**: Systemic issue (e.g., apiserver problem, test infrastructure issue)
- **Pattern is erratic**: Watch count varies wildly between runs (flaky behavior)

### How to Investigate Failures

When the test fails:

1. **Check search.ci for recent failures**:
   ```
   https://search.ci.openshift.org/?search=produces+more+watch+requests+than+expected&maxAge=336h&context=0&type=bug%2Bjunit&name=&excludeName=&maxMatches=5&maxBytes=20971520&groupBy=job
   ```

2. **Review the test output** to see actual vs expected counts:
   ```
   Operator "marketplace-operator" produces more watch requests than expected:
   watchrequestcount=120, upperbound=104, ratio=1.15
   ```

3. **Calculate the percentage increase**:
   ```
   (new - old) / old * 100 = (120 - 52) / 52 * 100 = 130.8%
   ```
   Note: The upperbound shown in output is already 2x the value in the map (52 * 2 = 104)

4. **Check recent commits** to the failing operator:
   ```bash
   # For marketplace-operator example:
   cd /path/to/operator/repo
   git log --since="1 month ago" --grep="watch" --oneline
   ```

5. **Decision matrix**:
   - Increase <50%, explainable → Update limit with this command
   - Increase 50-200%, explainable → Update limit and document reason in commit
   - Increase >200%, explainable → Consider if optimization is possible first
   - Increase >1000% or unexplained → File bug, do not update limit

## Notes

- The test file uses Go const-style maps, so formatting consistency is important
- Platform type must match the `configv1.PlatformType` enum exactly (e.g., `AWSPlatformType`)
- Single-node limits are currently only defined for AWS platform
- The actual enforced limit is 2x the value in the map (to account for hour bucket boundaries)
- Changes to this test should include rationale in commit message
- After updating, verify the change compiles: `go build ./pkg/monitortests/kubeapiserver/auditloganalyzer/...`
- Consider checking search.ci to ensure the new limit has headroom (not just barely passing)
- Decimal notation (.0 suffix) should be preserved for consistency with existing values
- If adding limits for a new operator, ensure the operator name exactly matches the username in audit logs
- The test runs in the [Late] phase, so it captures watch behavior across the entire test run

## Commit Message Template

When committing limit updates, use this format:

```
Update watch request limits for <operator-name> on <platform>

Operator: <operator-name>
Platform: <platform>
Topology: <HA|single>
Old limit: <old-value>
New limit: <new-value>
Increase: <percentage>%

Reason: <explanation of why the increase is happening>
- <specific code change or feature>
- <reference to PR or commit if available>

Test: [sig-arch][Late] operators should not create watch channels very often

Search.ci evidence: <link to search.ci query showing failures>
```

Example:
```
Update watch request limits for marketplace-operator on AWS

Operator: marketplace-operator
Platform: AWS
Topology: HA
Old limit: 52
New limit: 60
Increase: 15.4%

Reason: Marketplace operator now watches ClusterServiceVersion resources
in all namespaces to support operator lifecycle management improvements
introduced in https://github.com/operator-framework/operator-lifecycle-manager/pull/1234

Test: [sig-arch][Late] operators should not create watch channels very often

Search.ci evidence: https://search.ci.openshift.org/?search=marketplace-operator+produces+more+watch
```

## See Also

- Test implementation: `pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go`
- Original issue: https://issues.redhat.com/browse/WRKLDS-291
- Search CI: https://search.ci.openshift.org/
- Related test: `[sig-arch][Late] operators should have watch channel requests` (ensures operators are actually making watches)
