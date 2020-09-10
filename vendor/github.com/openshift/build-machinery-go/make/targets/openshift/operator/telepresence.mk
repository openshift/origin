self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))
scripts_dir :=$(shell realpath $(self_dir)../../../../scripts)

telepresence:
	$(info Running operator locally against a remote cluster using telepresence (https://telepresence.io))
	$(info )
	$(info To override the operator log level, set TP_VERBOSITY=<log level>)
	$(info To debug the operator, set TP_DEBUG=y (requires the delve debugger))
	$(info See the run-telepresence.sh script for more usage and configuration details)
	$(info )
	bash $(scripts_dir)/run-telepresence.sh
.PHONY: telepresence
