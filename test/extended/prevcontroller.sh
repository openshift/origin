#!/bin/bash
#
# Runs compatibility tests with a previous controller version
RUN_PREVIOUS_CONTROLLER=1 SKIP_TESTS="\[SkipPrevControllers\]" \
	"$(dirname "${BASH_SOURCE}")/compatibility.sh"
