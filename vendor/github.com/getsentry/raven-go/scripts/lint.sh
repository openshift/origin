#!/bin/bash
set -e

go get golang.org/x/lint/golint

# Http => HTTP change would require a major version bump as all those functions/types are exported
if [[ -z "$(golint | grep -vE \"Http.*should be.*HTTP\")" ]]; then
  exit 0
else
  echo "Failed golint command"
  exit 1
fi
