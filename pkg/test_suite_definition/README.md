This package contains the definition of all test suites.

We describe test suites sufficiently for our suite runners and e2e monitor to understand
what setup, monitoring, and invariant tests should be enforced.
We have those react to this information instead of the other way around because we can know the
producer (suite creator), but the suite creator cannot realistically know all the test writers.

Dimensions include
1. is it an update?
2. is it a stable system (no platform disruption like a disaster recovery scenario)?
