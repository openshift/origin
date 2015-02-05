#!/bin/sh
# This script will update the application-template-stibuild.json with the correct
# registry service ip+port.  You must have started openshift and deployed
# the docker-registry service before running this.
# The new template file produced is template.json.
REGISTRY_IP=$(osc get services docker-registry -o template --template="{{ .portalIP}}:{{ .port }}")
sed s/172\.30\.17\.3:5001/$REGISTRY_IP/g application-template-stibuild.json > template.json
