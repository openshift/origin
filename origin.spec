#debuginfo not supported with Go
%global debug_package %{nil}
# modifying the Go binaries breaks the DWARF debugging
%global __os_install_post %{_rpmconfigdir}/brp-compress

%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
# The following should only be used for cleanup of sdn-ovs upgrades
%global kube_plugin_path /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet

# docker_version is the version of docker requires by packages
%global docker_version 1.12
# tuned_version is the version of tuned requires by packages
%global tuned_version  2.3
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.6.1
# this is the version we obsolete up to. The packaging changed for Origin
# 1.0.6 and OSE 3.1 such that 'openshift' package names were no longer used.
%global package_refector_version 3.0.2.900
%global golang_version 1.8.1
# %commit and %os_git_vars are intended to be set by tito custom builders provided
# in the .tito/lib directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit 86b5e46426ba828f49195af21c56f7c6674b48f7
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# os_git_vars needed to run hack scripts during rpm builds
%{!?os_git_vars:
%global os_git_vars OS_GIT_VERSION='' OS_GIT_COMMIT='' OS_GIT_MAJOR='' OS_GIT_MINOR='' OS_GIT_TREE_STATE='' OS_GIT_CATALOG_VERSION=''
}

%if 0%{?fedora} || 0%{?epel}
%global need_redistributable_set 0
%else
# Due to library availability, redistributable builds only work on x86_64
%ifarch x86_64
%global need_redistributable_set 1
%else
%global need_redistributable_set 0
%endif
%endif
%{!?make_redistributable: %global make_redistributable %{need_redistributable_set}}

%if "%{dist}" == ".el7aos"
%global package_name atomic-openshift
%global product_name Atomic OpenShift
%else
%global package_name origin
%global product_name Origin
%endif

Name:           %{package_name}
# Version is not kept up to date and is intended to be set by tito custom
# builders provided in the .tito/lib directory of this project
Version:        0.0.1
Release:        0%{?dist}
Summary:        Open Source Container Management by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}

# If go_arches not defined fall through to implicit golang archs
%if 0%{?go_arches:1}
ExclusiveArch:  %{go_arches}
%else
ExclusiveArch:  x86_64 aarch64 ppc64le s390x
%endif

Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz
BuildRequires:  systemd
BuildRequires:  bsdtar
BuildRequires:  golang >= %{golang_version}
BuildRequires:  krb5-devel
BuildRequires:  rsync
Requires:       %{name}-clients = %{version}-%{release}
Requires:       iptables
Obsoletes:      openshift < %{package_refector_version}

#
# The following Bundled Provides entries are populated automatically by the
# OpenShift Origin tito custom builder found here:
#   https://github.com/openshift/origin/blob/master/.tito/lib/origin/builder/
#
# These are defined as per:
# https://fedoraproject.org/wiki/Packaging:Guidelines#Bundling_and_Duplication_of_system_libraries
#
### AUTO-BUNDLED-GEN-ENTRY-POINT

%description
OpenShift is a distribution of Kubernetes optimized for enterprise application
development and deployment. OpenShift adds developer and operational centric
tools on top of Kubernetes to enable rapid application development, easy
deployment and scaling, and long-term lifecycle maintenance for small and large
teams and applications. It provides a secure and multi-tenant configuration for
Kubernetes allowing you to safely host many different applications and workloads
on a unified cluster.

%package master
Summary:        %{product_name} Master
Requires:       %{name} = %{version}-%{release}
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd
Obsoletes:      openshift-master < %{package_refector_version}

%description master
%{summary}

%package tests
Summary: %{product_name} Test Suite

%description tests
%{summary}

%package node
Summary:        %{product_name} Node
Requires:       %{name} = %{version}-%{release}
Requires:       docker >= %{docker_version}
Requires:       tuned-profiles-%{name}-node = %{version}-%{release}
Requires:       util-linux
Requires:       socat
Requires:       nfs-utils
Requires:       ethtool
Requires:       device-mapper-persistent-data >= 0.6.2
Requires:       conntrack-tools
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd
Obsoletes:      openshift-node < %{package_refector_version}

%description node
%{summary}

%package -n tuned-profiles-%{name}-node
Summary:        Tuned profiles for %{product_name} Node hosts
Requires:       tuned >= %{tuned_version}
Obsoletes:      tuned-profiles-openshift-node < %{package_refector_version}

%description -n tuned-profiles-%{name}-node
%{summary}

%package clients
Summary:        %{product_name} Client binaries for Linux
Obsoletes:      openshift-clients < %{package_refector_version}
Requires:       git
Requires:       bash-completion

%description clients
%{summary}

%if 0%{?make_redistributable}
%package clients-redistributable
Summary:        %{product_name} Client binaries for Linux, Mac OSX, and Windows
Obsoletes:      openshift-clients-redistributable < %{package_refector_version}
BuildRequires:  goversioninfo

%description clients-redistributable
%{summary}
%endif

%package dockerregistry
Summary:        Docker Registry v2 for %{product_name}

%description dockerregistry
%{summary}

%package pod
Summary:        %{product_name} Pod

%description pod
%{summary}

%package sdn-ovs
Summary:          %{product_name} SDN Plugin for Open vSwitch
Requires:         openvswitch >= %{openvswitch_version}
Requires:         %{name}-node = %{version}-%{release}
Requires:         bridge-utils
Requires:         bind-utils
Requires:         ethtool
Requires:         procps-ng
Requires:         iproute
Obsoletes:        openshift-sdn-ovs < %{package_refector_version}

%description sdn-ovs
%{summary}

%package federation-services
Summary:        %{produce_name} Federation Services

%description federation-services

%package service-catalog
Summary:        %{product_name} Service Catalog

%description service-catalog
%{summary}

%package template-service-broker
Summary: Template Service Broker
%description template-service-broker
%{summary}

%package cluster-capacity
Summary:        %{product_name} Cluster Capacity Analysis Tool

%description cluster-capacity
%{summary}

%package excluder
Summary:   Exclude openshift packages from updates
BuildArch: noarch

%description excluder
Many times admins do not want openshift updated when doing
normal system updates.

%{name}-excluder exclude - No openshift packages can be updated
%{name}-excluder unexclude - Openshift packages can be updated

%package docker-excluder
Summary:   Exclude docker packages from updates
BuildArch: noarch

%description docker-excluder
Certain versions of OpenShift will not work with newer versions
of docker.  Exclude those versions of docker.

%{name}-docker-excluder exclude - No major docker updates
%{name}-docker-excluder unexclude - docker packages can be updated

%prep
%setup -q

%build
%if 0%{make_redistributable}
# Create Binaries for all supported arches
%{os_git_vars} hack/build-cross.sh
%{os_git_vars} hack/build-go.sh vendor/github.com/onsi/ginkgo/ginkgo
%{os_git_vars} unset GOPATH; cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog/hack/build-cross.sh
%{os_git_vars} unset GOPATH; cmd/cluster-capacity/go/src/github.com/kubernetes-incubator/cluster-capacity/hack/build-cross.sh
%else
# Create Binaries only for building arch
%ifarch x86_64
  BUILD_PLATFORM="linux/amd64"
%endif
%ifarch %{ix86}
  BUILD_PLATFORM="linux/386"
%endif
%ifarch ppc64le
  BUILD_PLATFORM="linux/ppc64le"
%endif
%ifarch %{arm} aarch64
  BUILD_PLATFORM="linux/arm64"
%endif
%ifarch s390x
  BUILD_PLATFORM="linux/s390x"
%endif
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} hack/build-cross.sh
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} hack/build-go.sh vendor/github.com/onsi/ginkgo/ginkgo
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} unset GOPATH; cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog/hack/build-cross.sh
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} unset GOPATH; cmd/cluster-capacity/go/src/github.com/kubernetes-incubator/cluster-capacity/hack/build-cross.sh
%endif

# Generate man pages
%{os_git_vars} hack/generate-docs.sh

%install

PLATFORM="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
install -d %{buildroot}%{_bindir}

# Install linux components
for bin in oc openshift dockerregistry kubefed template-service-broker
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 _output/local/bin/${PLATFORM}/${bin} %{buildroot}%{_bindir}/${bin}
done

# Install tests
install -d %{buildroot}%{_libexecdir}/%{name}
install -p -m 755 _output/local/bin/${PLATFORM}/extended.test %{buildroot}%{_libexecdir}/%{name}/
install -p -m 755 _output/local/bin/${PLATFORM}/ginkgo %{buildroot}%{_libexecdir}/%{name}/

%if 0%{?make_redistributable}
# Install client executable for windows and mac
install -d %{buildroot}%{_datadir}/%{name}/{linux,macosx,windows}
install -p -m 755 _output/local/bin/linux/amd64/oc %{buildroot}%{_datadir}/%{name}/linux/oc
install -p -m 755 _output/local/bin/darwin/amd64/oc %{buildroot}/%{_datadir}/%{name}/macosx/oc
install -p -m 755 _output/local/bin/windows/amd64/oc.exe %{buildroot}/%{_datadir}/%{name}/windows/oc.exe
%endif

# Install federation services
install -p -m 755 _output/local/bin/${PLATFORM}/hyperkube %{buildroot}%{_bindir}/

# Install cluster capacity
install -p -m 755 cmd/cluster-capacity/go/src/github.com/kubernetes-incubator/cluster-capacity/_output/local/bin/${PLATFORM}/hypercc %{buildroot}%{_bindir}/
ln -s hypercc %{buildroot}%{_bindir}/cluster-capacity

# Install service-catalog
install -p -m 755 cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog/_output/local/bin/${PLATFORM}/apiserver %{buildroot}%{_bindir}/
install -p -m 755 cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog/_output/local/bin/${PLATFORM}/controller-manager %{buildroot}%{_bindir}/

# Install pod
install -p -m 755 _output/local/bin/${PLATFORM}/pod %{buildroot}%{_bindir}/

install -d -m 0755 %{buildroot}%{_unitdir}

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig

for cmd in \
    kube-apiserver \
    kube-controller-manager \
    kube-proxy \
    kube-scheduler \
    kubelet \
    kubernetes \
    oadm \
    openshift-deploy \
    openshift-docker-build \
    openshift-sti-build \
    openshift-git-clone \
    openshift-manage-dockerfile \
    openshift-extract-image-content \
    openshift-f5-router \
    openshift-recycle \
    openshift-router \
    origin
do
    ln -s openshift %{buildroot}%{_bindir}/$cmd
done

ln -s oc %{buildroot}%{_bindir}/kubectl

install -d -m 0755 %{buildroot}%{_sysconfdir}/origin/{master,node}

# different service for origin vs aos
install -m 0644 contrib/systemd/%{name}-master.service %{buildroot}%{_unitdir}/%{name}-master.service
install -m 0644 contrib/systemd/%{name}-node.service %{buildroot}%{_unitdir}/%{name}-node.service
# same sysconfig files for origin vs aos
install -m 0644 contrib/systemd/origin-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/%{name}-master
install -m 0644 contrib/systemd/origin-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/%{name}-node
install -d -m 0755 %{buildroot}%{_prefix}/lib/tuned/%{name}-node-{guest,host}
install -m 0644 contrib/tuned/origin-node-guest/tuned.conf %{buildroot}%{_prefix}/lib/tuned/%{name}-node-guest/tuned.conf
install -m 0644 contrib/tuned/origin-node-host/tuned.conf %{buildroot}%{_prefix}/lib/tuned/%{name}-node-host/tuned.conf

# Install man1 man pages
install -d -m 0755 %{buildroot}%{_mandir}/man1
install -m 0644 docs/man/man1/* %{buildroot}%{_mandir}/man1/

# Patch and install the manpage for tuned profiles on aos
install -d -m 0755 %{buildroot}%{_mandir}/man7
%if "%{dist}" == ".el7aos"
%{__sed} -e 's|origin-node|atomic-openshift-node|g' \
 -e 's|ORIGIN_NODE|ATOMIC_OPENSHIFT_NODE|' \
 contrib/tuned/man/tuned-profiles-origin-node.7 > %{buildroot}%{_mandir}/man7/tuned-profiles-%{name}-node.7
%else
install -m 0644 contrib/tuned/man/tuned-profiles-origin-node.7 %{buildroot}%{_mandir}/man7/tuned-profiles-%{name}-node.7
%endif

mkdir -p %{buildroot}%{_sharedstatedir}/origin

# Install sdn scripts
install -d -m 0755 %{buildroot}%{_sysconfdir}/cni/net.d
install -d -m 0755 %{buildroot}/opt/cni/bin
install -p -m 0755 _output/local/bin/${PLATFORM}/sdn-cni-plugin %{buildroot}/opt/cni/bin/openshift-sdn
install -p -m 0755 _output/local/bin/${PLATFORM}/host-local %{buildroot}/opt/cni/bin
install -p -m 0755 _output/local/bin/${PLATFORM}/loopback %{buildroot}/opt/cni/bin

install -d -m 0755 %{buildroot}%{_unitdir}/%{name}-node.service.d
install -p -m 0644 contrib/systemd/openshift-sdn-ovs.conf %{buildroot}%{_unitdir}/%{name}-node.service.d/openshift-sdn-ovs.conf

# Install bash completions
install -d -m 755 %{buildroot}%{_sysconfdir}/bash_completion.d/
for bin in oadm oc openshift
do
  echo "+++ INSTALLING BASH COMPLETIONS FOR ${bin} "
  %{buildroot}%{_bindir}/${bin} completion bash > %{buildroot}%{_sysconfdir}/bash_completion.d/${bin}
  chmod 644 %{buildroot}%{_sysconfdir}/bash_completion.d/${bin}
done

# Install origin-accounting
install -d -m 755 %{buildroot}%{_sysconfdir}/systemd/system.conf.d/
install -p -m 644 contrib/systemd/origin-accounting.conf %{buildroot}%{_sysconfdir}/systemd/system.conf.d/

# Excluder variables
mkdir -p $RPM_BUILD_ROOT/usr/sbin
%if 0%{?fedora}
  OS_CONF_FILE="/etc/dnf.conf"
%else
  OS_CONF_FILE="/etc/yum.conf"
%endif

# Install openshift-excluder script
sed "s|@@CONF_FILE-VARIABLE@@|${OS_CONF_FILE}|" contrib/excluder/excluder-template > $RPM_BUILD_ROOT/usr/sbin/%{name}-excluder
sed -i "s|@@PACKAGE_LIST-VARIABLE@@|%{name} %{name}-clients %{name}-clients-redistributable %{name}-dockerregistry %{name}-master %{name}-node %{name}-pod %{name}-recycle %{name}-sdn-ovs %{name}-tests tuned-profiles-%{name}-node|" $RPM_BUILD_ROOT/usr/sbin/%{name}-excluder
chmod 0744 $RPM_BUILD_ROOT/usr/sbin/%{name}-excluder

# Install docker-excluder script
sed "s|@@CONF_FILE-VARIABLE@@|${OS_CONF_FILE}|" contrib/excluder/excluder-template > $RPM_BUILD_ROOT/usr/sbin/%{name}-docker-excluder
sed -i "s|@@PACKAGE_LIST-VARIABLE@@|docker*1.13* docker*1.14* docker*1.15* docker*1.16* docker*1.17* docker*1.18* docker*1.19* docker*1.20*|" $RPM_BUILD_ROOT/usr/sbin/%{name}-docker-excluder
chmod 0744 $RPM_BUILD_ROOT/usr/sbin/%{name}-docker-excluder

# Install migration scripts
install -d %{buildroot}%{_datadir}/%{name}/migration
install -p -m 755 contrib/migration/* %{buildroot}%{_datadir}/%{name}/migration/

%files
%doc README.md
%license LICENSE
%{_bindir}/openshift
%{_bindir}/kube-apiserver
%{_bindir}/kube-controller-manager
%{_bindir}/kube-proxy
%{_bindir}/kube-scheduler
%{_bindir}/kubelet
%{_bindir}/kubernetes
%{_bindir}/oadm
%{_bindir}/openshift-deploy
%{_bindir}/openshift-f5-router
%{_bindir}/openshift-recycle
%{_bindir}/openshift-router
%{_bindir}/openshift-docker-build
%{_bindir}/openshift-sti-build
%{_bindir}/openshift-git-clone
%{_bindir}/openshift-extract-image-content
%{_bindir}/openshift-manage-dockerfile
%{_bindir}/origin
%{_sharedstatedir}/origin
%{_sysconfdir}/bash_completion.d/oadm
%{_sysconfdir}/bash_completion.d/openshift
%defattr(-,root,root,0700)
%dir %config(noreplace) %{_sysconfdir}/origin
%ghost %dir %config(noreplace) %{_sysconfdir}/origin
%ghost %config(noreplace) %{_sysconfdir}/origin/.config_managed
%{_mandir}/man1/oadm*
%{_mandir}/man1/openshift*

%pre
# If /etc/openshift exists and /etc/origin doesn't, symlink it to /etc/origin
if [ -d "%{_sysconfdir}/openshift" ]; then
  if ! [ -d "%{_sysconfdir}/origin"  ]; then
    ln -s %{_sysconfdir}/openshift %{_sysconfdir}/origin
  fi
fi
if [ -d "%{_sharedstatedir}/openshift" ]; then
  if ! [ -d "%{_sharedstatedir}/origin"  ]; then
    ln -s %{_sharedstatedir}/openshift %{_sharedstatedir}/origin
  fi
fi

%files tests
%{_libexecdir}/%{name}
%{_libexecdir}/%{name}/extended.test

%files master
%{_unitdir}/%{name}-master.service
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}-master
%dir %{_datadir}/%{name}/migration/
%{_datadir}/%{name}/migration/*
%defattr(-,root,root,0700)
%config(noreplace) %{_sysconfdir}/origin/master
%ghost %config(noreplace) %{_sysconfdir}/origin/admin.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/admin.key
%ghost %config(noreplace) %{_sysconfdir}/origin/admin.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/ca.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/ca.key
%ghost %config(noreplace) %{_sysconfdir}/origin/ca.serial.txt
%ghost %config(noreplace) %{_sysconfdir}/origin/etcd.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/etcd.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master-config.yaml
%ghost %config(noreplace) %{_sysconfdir}/origin/master.etcd-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master.etcd-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master.kubelet-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master.kubelet-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-master.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-master.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-master.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-registry.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-registry.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-registry.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-router.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-router.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-router.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/policy.json
%ghost %config(noreplace) %{_sysconfdir}/origin/serviceaccounts.private.key
%ghost %config(noreplace) %{_sysconfdir}/origin/serviceaccounts.public.key
%ghost %config(noreplace) %{_sysconfdir}/origin/.config_managed

%post master
%systemd_post %{name}-master.service
# Create master config and certs if both do not exist
if [[ ! -e %{_sysconfdir}/origin/master/master-config.yaml &&
     ! -e %{_sysconfdir}/origin/master/ca.crt ]]; then
  %{_bindir}/openshift start master --write-config=%{_sysconfdir}/origin/master
  # Create node configs if they do not already exist
  if ! find %{_sysconfdir}/origin/ -type f -name "node-config.yaml" | grep -E "node-config.yaml"; then
    %{_bindir}/oc adm create-node-config --node-dir=%{_sysconfdir}/origin/node/ --node=localhost --hostnames=localhost,127.0.0.1 --node-client-certificate-authority=%{_sysconfdir}/origin/master/ca.crt --signer-cert=%{_sysconfdir}/origin/master/ca.crt --signer-key=%{_sysconfdir}/origin/master/ca.key --signer-serial=%{_sysconfdir}/origin/master/ca.serial.txt --certificate-authority=%{_sysconfdir}/origin/master/ca.crt
  fi
  # Generate a marker file that indicates config and certs were RPM generated
  echo "# Config generated by RPM at "`date -u` > %{_sysconfdir}/origin/.config_managed
fi


%preun master
%systemd_preun %{name}-master.service

%postun master
%systemd_postun

%files node
%{_unitdir}/%{name}-node.service
%{_sysconfdir}/systemd/system.conf.d/origin-accounting.conf
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}-node
%defattr(-,root,root,0700)
%config(noreplace) %{_sysconfdir}/origin/node
%ghost %config(noreplace) %{_sysconfdir}/origin/node/node-config.yaml
%ghost %config(noreplace) %{_sysconfdir}/origin/.config_managed

%post node
%systemd_post %{name}-node.service
# If accounting is not currently enabled systemd reexec
if [[ `systemctl show docker %{name}-node | grep -q -e CPUAccounting=no -e MemoryAccounting=no; echo $?` == 0 ]]; then
  systemctl daemon-reexec
fi

%preun node
%systemd_preun %{name}-node.service

%postun node
%systemd_postun

%files sdn-ovs
%dir %{_unitdir}/%{name}-node.service.d/
%dir %{_sysconfdir}/cni/net.d
%dir /opt/cni/bin
%{_unitdir}/%{name}-node.service.d/openshift-sdn-ovs.conf
/opt/cni/bin/*

%posttrans sdn-ovs
# This path was installed by older packages but the directory wasn't owned by
# RPM so we need to clean it up otherwise kubelet throws an error trying to
# load the directory as a plugin
if [ -d %{kube_plugin_path} ]; then
  rmdir %{kube_plugin_path}
fi

%files service-catalog
%{_bindir}/apiserver
%{_bindir}/controller-manager

%files -n tuned-profiles-%{name}-node
%license LICENSE
%{_prefix}/lib/tuned/%{name}-node-host
%{_prefix}/lib/tuned/%{name}-node-guest
%{_mandir}/man7/tuned-profiles-%{name}-node.7*

%post -n tuned-profiles-%{name}-node
recommended=`/usr/sbin/tuned-adm recommend`
if [[ "${recommended}" =~ guest ]] ; then
  /usr/sbin/tuned-adm profile %{name}-node-guest > /dev/null 2>&1
else
  /usr/sbin/tuned-adm profile %{name}-node-host > /dev/null 2>&1
fi

%preun -n tuned-profiles-%{name}-node
# reset the tuned profile to the recommended profile
# $1 = 0 when we're being removed > 0 during upgrades
if [ "$1" = 0 ]; then
  recommended=`/usr/sbin/tuned-adm recommend`
  /usr/sbin/tuned-adm profile $recommended > /dev/null 2>&1
fi

%files clients
%license LICENSE
%{_bindir}/oc
%{_bindir}/kubectl
%{_bindir}/kubefed
%{_sysconfdir}/bash_completion.d/oc
%{_mandir}/man1/oc*

%if 0%{?make_redistributable}
%files clients-redistributable
%dir %{_datadir}/%{name}/linux/
%dir %{_datadir}/%{name}/macosx/
%dir %{_datadir}/%{name}/windows/
%{_datadir}/%{name}/linux/oc
%{_datadir}/%{name}/macosx/oc
%{_datadir}/%{name}/windows/oc.exe
%endif

%files dockerregistry
%{_bindir}/dockerregistry

%files pod
%{_bindir}/pod

%files excluder
/usr/sbin/%{name}-excluder

%pretrans excluder
# we always want to clear this out using the last
#   versions script.  Otherwise excludes might get left in
if [ -s /usr/sbin/%{name}-excluder ] ; then
  /usr/sbin/%{name}-excluder unexclude
fi

%posttrans excluder
# we always want to run this after an install or update
/usr/sbin/%{name}-excluder exclude

%preun excluder
# If we are the last one, clean things up
if [ "$1" -eq 0 ] ; then
  /usr/sbin/%{name}-excluder unexclude
fi

%files docker-excluder
/usr/sbin/%{name}-docker-excluder

%files cluster-capacity
%{_bindir}/hypercc
%{_bindir}/cluster-capacity

%files template-service-broker
%{_bindir}/template-service-broker


%pretrans docker-excluder
# we always want to clear this out using the last
#   versions script.  Otherwise excludes might get left in
if [ -s /usr/sbin/%{name}-docker-excluder ] ; then
  /usr/sbin/%{name}-docker-excluder unexclude
fi

%posttrans docker-excluder
# we always want to run this after an install or update
/usr/sbin/%{name}-docker-excluder exclude

%preun docker-excluder
# If we are the last one, clean things up
if [ "$1" -eq 0 ] ; then
  /usr/sbin/%{name}-docker-excluder unexclude
fi

%files federation-services
%{_bindir}/hyperkube

%changelog
* Fri Sep 18 2015 Scott Dodson <sdodson@redhat.com> 0.2-9
- Rename from openshift -> origin
- Symlink /var/lib/origin to /var/lib/openshift if /var/lib/openshift exists

* Wed Aug 12 2015 Steve Milner <smilner@redhat.com> 0.2-8
- Master configs will be generated if none are found when the master is installed.
- Node configs will be generated if none are found when the master is installed.
- Additional notice file added if config is generated by the RPM.
- All-In-One services removed.

* Wed Aug 12 2015 Steve Milner <smilner@redhat.com> 0.2-7
- Added new ovs script(s) to file lists.

* Wed Aug  5 2015 Steve Milner <smilner@redhat.com> 0.2-6
- Using _unitdir instead of _prefix for unit data

* Fri Jul 31 2015 Steve Milner <smilner@redhat.com> 0.2-5
- Configuration location now /etc/origin
- Default configs created upon installation

* Tue Jul 28 2015 Steve Milner <smilner@redhat.com> 0.2-4
- Added AEP packages

* Mon Jan 26 2015 Scott Dodson <sdodson@redhat.com> 0.2-3
- Update to 21fb40637c4e3507cca1fcab6c4d56b06950a149
- Split packaging of openshift-master and openshift-node

* Mon Jan 19 2015 Scott Dodson <sdodson@redhat.com> 0.2-2
- new package built with tito

* Fri Jan 09 2015 Adam Miller <admiller@redhat.com> - 0.2-2
- Add symlink for osc command line tooling (merged in from jhonce@redhat.com)

* Wed Jan 07 2015 Adam Miller <admiller@redhat.com> - 0.2-1
- Update to latest upstream release
- Restructured some of the golang deps  build setup for restructuring done
  upstream

* Thu Oct 23 2014 Adam Miller <admiller@redhat.com> - 0-0.0.9.git562842e
- Add new patches from jhonce for systemd units

* Mon Oct 20 2014 Adam Miller <admiller@redhat.com> - 0-0.0.8.git562842e
- Update to latest master snapshot

* Wed Oct 15 2014 Adam Miller <admiller@redhat.com> - 0-0.0.7.git7872f0f
- Update to latest master snapshot

* Fri Oct 03 2014 Adam Miller <admiller@redhat.com> - 0-0.0.6.gite4d4ecf
- Update to latest Alpha nightly build tag 20141003

* Wed Oct 01 2014 Adam Miller <admiller@redhat.com> - 0-0.0.5.git6d9f1a9
- Switch to consistent naming, patch by jhonce

* Tue Sep 30 2014 Adam Miller <admiller@redhat.com> - 0-0.0.4.git6d9f1a9
- Add systemd and sysconfig entries from jhonce

* Tue Sep 23 2014 Adam Miller <admiller@redhat.com> - 0-0.0.3.git6d9f1a9
- Update to latest upstream.

* Mon Sep 15 2014 Adam Miller <admiller@redhat.com> - 0-0.0.2.git2647df5
- Update to latest upstream.
