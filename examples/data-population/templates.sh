# Templates

# Populates the "openshift" namespace with set of templates

set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating templates"

OPENSHIFTCONFIG=${OPENSHIFT_ADMIN_CONFIG}

ORIGIN_ROOT=$(dirname "${BASH_SOURCE}")/../..
EXAMPLES_ROOT=${ORIGIN_ROOT}/examples

TEMPLATES="$EXAMPLES_ROOT/db-templates
$EXAMPLES_ROOT/sample-app/application-template-*"

for f in $TEMPLATES
do
  openshift cli create -f $f --namespace=openshift  
done

echo "Done"