#!/bin/bash

# Populate

# Populates the system with everything

echo "Populating all content"

source $(dirname "${BASH_SOURCE}")/users.sh
source $(dirname "${BASH_SOURCE}")/templates.sh
source $(dirname "${BASH_SOURCE}")/projects.sh
source $(dirname "${BASH_SOURCE}")/limits.sh
source $(dirname "${BASH_SOURCE}")/quotas.sh
source $(dirname "${BASH_SOURCE}")/apps.sh

echo "Done"
