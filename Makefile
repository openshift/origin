# Old-skool build tools.
#
# Targets (see each target for more information):
#   all: Build code.
#   build: Build code.
#   clean: Clean up.

OUT_DIR = _output
OUT_PKG_DIR = Godeps/_workspace/pkg

CONTROLLER_DIR = ovssubnet/controller

export GOFLAGS

# Build code.
#
# Args:
#   WHAT: Directory names to build.  If any of these directories has a 'main'
#     package, the build will produce executable files under $(OUT_DIR)/local/go/bin.
#     If not specified, "everything" will be built.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#
# Example:
#   make
#   make all
#   make all WHAT=cmd/kubelet GOFLAGS=-v
all build:
	hack/build.sh $(WHAT)
.PHONY: all build

install:
	rm -f /usr/bin/openshift-sdn
	rm -f /usr/bin/openshift-sdn-simple-setup-node.sh
	cp -f $(OUT_DIR)/local/go/bin/openshift-sdn /usr/bin/
	cp -f $(OUT_DIR)/local/go/bin/openshift-sdn-simple-setup-node.sh /usr/bin/
	cp -f $(OUT_DIR)/local/go/bin/openshift-ovs-subnet /usr/bin/
	cp -f $(OUT_DIR)/local/go/bin/openshift-sdn-kube-subnet-setup.sh /usr/bin/
	mkdir -p /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/
	cp -f $(OUT_DIR)/local/go/bin/openshift-ovs-subnet /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/
	cp -f $(OUT_DIR)/local/go/bin/openshift-ovs-multitenant /usr/bin/
	cp -f $(OUT_DIR)/local/go/bin/openshift-sdn-multitenant-setup.sh /usr/bin/
	mkdir -p /usr/lib/systemd/system/docker.service.d/
	cp -f rel-eng/docker-sdn-ovs.conf /usr/lib/systemd/system/docker.service.d/

install-dev:
	rm -f /usr/bin/openshift-sdn
	rm -f /usr/bin/openshift-sdn-simple-setup-node.sh
	cp -f $(OUT_DIR)/local/go/bin/openshift-sdn /usr/bin/
	ln -rsf $(CONTROLLER_DIR)/lbr/bin/openshift-sdn-simple-setup-node.sh /usr/bin/
	ln -rsf $(CONTROLLER_DIR)/kube/bin/openshift-ovs-subnet /usr/bin/
	ln -rsf $(CONTROLLER_DIR)/kube/bin/openshift-sdn-kube-subnet-setup.sh /usr/bin/
	mkdir -p /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/
	ln -rsf $(CONTROLLER_DIR)/kube/bin/openshift-ovs-subnet /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/
	ln -rsf $(CONTROLLER_DIR)/multitenant/bin/openshift-ovs-multitenant /usr/bin/
	ln -rsf $(CONTROLLER_DIR)/multitenant/bin/openshift-sdn-multitenant-setup.sh /usr/bin/
	mkdir -p /usr/lib/systemd/system/docker.service.d/
	ln -rsf rel-eng/docker-sdn-ovs.conf /usr/lib/systemd/system/docker.service.d/


# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR) $(OUT_PKG_DIR)
.PHONY: clean

