# Generates 500 projects

set -o errexit
set -o nounset
set -o pipefail

#!/bin/bash
for i in {1..500}
do
  openshift ex new-project projects-${i}
done