#!/bin/bash
#
# Runs compatibility tests with a previous controller and API server version
RUN_PREVIOUS_CONTROLLER=1 RUN_PREVIOUS_API=1 SKIP_TESTS="\[SkipPrevAPIAndControllers\]" \
	"$(dirname "${BASH_SOURCE}")/compatibility.sh"
