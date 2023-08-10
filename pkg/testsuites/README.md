This package contains the definition of all test suites.

We describe test suites sufficiently for our suite runners and e2e monitor to understand
what setup, e2e monitoring, and invariant tests should be run.
We have the consumers (suite runners, e2e monitor, invariants) react to this information instead
of the other way around because we can know the producer (suite creator), but the suite creator
cannot realistically know all the test writers.

Dimensions include
1. is it an update?
2. is it a stable system (no platform disruption like a disaster recovery scenario)?
