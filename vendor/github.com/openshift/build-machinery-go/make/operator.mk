include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	default.mk \
	targets/openshift/operator/*.mk \
)
