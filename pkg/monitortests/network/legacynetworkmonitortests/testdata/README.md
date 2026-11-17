## Sandbox rollout fixtures

`sandbox-rollouts.json` retains six intervals from each of the five
OCPBUGS-141934 runs of
`periodic-ci-openshift-release-main-nightly-5.1-e2e-aws-ovn-upgrade-fips`:

- network `Progressing=True`;
- same-node OVN pod graceful deletion;
- affected pod's UID-bearing scheduled lifecycle;
- two missing-CNI sandbox failures;
- subsequent default-network `AddedInterface`.

The interval fields and timestamps are retained from the public artifacts.
The raw events are under
`https://storage.googleapis.com/test-platform-results-public/logs/<job>/<build-id>/artifacts/e2e-aws-ovn-upgrade-fips/openshift-e2e-test/artifacts/junit/`.

| Build ID | Events file |
| --- | --- |
| 2120188789715374080 | e2e-events_20261110-170733.json |
| 2120448016950235136 | e2e-events_20261111-103706.json |
| 2120448019441651712 | e2e-events_20261111-102829.json |
| 2120566602322677760 | e2e-events_20261111-180915.json |
| 2120566602289123328 | e2e-events_20261111-181301.json |

In these runs the operator *does* progress, but clears the condition before the
last node restores CNI. Pod network recovery occurs 22–28 seconds after the
same-node OVN deletion. Removing recovery from each fixture must retain a hard
failure; the synthetic table tests cover unrelated and ambiguous evidence.

Serialized intervals retain only whole-second timestamps. Live pod/operator
watch observations also retain fractions of a second, while Kubernetes Event
timestamp annotations have whole-second precision. Replay tests therefore add
representative sub-second offsets to the watch-derived intervals as well as
testing the saved timestamps. A separate regression test covers scheduling and
the first sandbox failure observed within the same second, and rejects a
scheduled lifecycle that starts in the following second.
