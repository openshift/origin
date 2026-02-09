#!/usr/bin/env python3
"""
Query BigQuery for operator watch request data from OpenShift CI.

This script queries the last 30 days of operator watch request data
for a specified OpenShift release version.

Usage:
    python3 query_operator_watch_requests.py 4.22
    python3 query_operator_watch_requests.py 4.23

Output:
    JSON array to stdout with operator watch request statistics
    Status messages to stderr
"""

import sys
import subprocess
import json


def main():
    if len(sys.argv) != 2:
        print("Usage: query_operator_watch_requests.py <release>", file=sys.stderr)
        print("Example: query_operator_watch_requests.py 4.22", file=sys.stderr)
        sys.exit(1)

    release = sys.argv[1]

    # Validate release format (e.g., 4.22, 4.23)
    if not release.replace('.', '').isdigit():
        print(f"Error: Invalid release format '{release}'. Expected format like '4.22'", file=sys.stderr)
        sys.exit(1)

    # Construct the BigQuery query
    query = f"""
SELECT
  Platform,
  Release,
  Topology,
  Operator,
  ROUND(AVG(WatchRequestCount), 0) AS Average,
  MAX(WatchRequestCount) AS Max_WatchCount,
  -- Takes all values in the group, sorts them, and picks the value at 99%
  APPROX_QUANTILES(WatchRequestCount, 100)[OFFSET(99)] AS P99_WatchCount
FROM
  `openshift-ci-data-analysis.ci_data_autodl.operator_watch_requests` AS Watches
  INNER JOIN `openshift-gce-devel.ci_analysis_us.jobs` AS JobRuns ON JobRuns.prowjob_build_id = Watches.JobRunName
  INNER JOIN `openshift-ci-data-analysis.ci_data.JobsWithVariants` AS Jobs ON Jobs.JobName = JobRuns.prowjob_job_name
WHERE
  PartitionTime > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY)
  AND Platform IN ('aws', 'gcp', 'azure', 'metal', 'openstack', 'vsphere')
  AND Release = '{release}'
  AND Topology IN ('ha', 'single')
GROUP BY
  1, 2, 3, 4
"""

    print(f"Querying BigQuery for release {release} (last 30 days)...", file=sys.stderr)
    print(f"Project: openshift-ci-data-analysis", file=sys.stderr)
    print(f"Dataset: ci_data_autodl.operator_watch_requests", file=sys.stderr)

    # Run bq query command
    cmd = [
        'bq', 'query',
        '--project_id=openshift-ci-data-analysis',
        '--format=json',
        '--use_legacy_sql=false',
        '--max_rows=1000',  # Increase from default 100 to handle all operator/platform/topology combinations
        query
    ]

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=True
        )

        # Parse and validate the JSON
        data = json.loads(result.stdout)
        print(f"✅ Query completed, retrieved {len(data)} results", file=sys.stderr)

        # Output the JSON to stdout (this is what the calling command will use)
        print(json.dumps(data, indent=2))

        return 0

    except subprocess.CalledProcessError as e:
        print(f"❌ Error running bq query: {e}", file=sys.stderr)
        if e.stderr:
            print(f"BigQuery error details:", file=sys.stderr)
            print(e.stderr, file=sys.stderr)
        return 1

    except json.JSONDecodeError as e:
        print(f"❌ Error parsing BigQuery JSON output: {e}", file=sys.stderr)
        print(f"Raw output: {result.stdout}", file=sys.stderr)
        return 1

    except FileNotFoundError:
        print("❌ Error: 'bq' command not found", file=sys.stderr)
        print("", file=sys.stderr)
        print("Please install Google Cloud SDK:", file=sys.stderr)
        print("  https://cloud.google.com/sdk/docs/install", file=sys.stderr)
        print("", file=sys.stderr)
        print("And authenticate with:", file=sys.stderr)
        print("  gcloud auth login", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
