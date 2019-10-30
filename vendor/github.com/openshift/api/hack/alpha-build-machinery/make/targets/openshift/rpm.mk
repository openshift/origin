RPM_OUTPUT_DIR ?=_output
RPM_TOPDIR ?=$(abspath ./)
RPM_BUILDDIR ?=$(RPM_TOPDIR)
RPM_BUILDROOT ?=$(RPM_TOPDIR)
RPM_SOURCEDIR ?=$(RPM_TOPDIR)
RPM_SPECDIR ?=$(RPM_TOPDIR)
RPM_RPMDIR ?=$(RPM_TOPDIR)/$(RPM_OUTPUT_DIR)/rpms
RPM_SRCRPMDIR ?=$(RPM_TOPDIR)/$(RPM_OUTPUT_DIR)/srpms

RPM_SPECFILES ?=$(wildcard *.spec)
RPM_BUILDFLAGS ?=-ba
RPM_EXTRAFLAGS ?=

rpm-build:
	$(strip \
	rpmbuild $(RPM_BUILDFLAGS) \
		--define "_topdir $(RPM_TOPDIR)" \
		--define "_builddir $(RPM_BUILDDIR)" \
		--define "_buildrootdir $(RPM_BUILDROOT)" \
		--define "_rpmdir $(RPM_RPMDIR)" \
		--define "_srcrpmdir $(RPM_SRCRPMDIR)" \
		--define "_specdir $(RPM_SPECDIR)" \
		--define "_sourcedir $(RPM_SOURCEDIR)" \
		--define "go_package $(GO_PACKAGE)" \
		$(RPM_EXTRAFLAGS) \
		$(RPM_SPECFILES) \
	)

clean-rpms:
	$(RM) -r '$(RPM_RPMDIR)' '$(RPM_SRCRPMDIR)'
	if [ -d '$(RPM_OUTPUT_DIR)' ]; then rmdir --ignore-fail-on-non-empty '$(RPM_OUTPUT_DIR)'; fi
.PHONY: clean-rpms

clean: clean-rpms

# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
)
