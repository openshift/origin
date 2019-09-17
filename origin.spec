#debuginfo not supported with Go
%global debug_package %{nil}
# modifying the Go binaries breaks the DWARF debugging
%global __os_install_post %{_rpmconfigdir}/brp-compress

%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
# The following should only be used for cleanup of sdn-ovs upgrades
%global kube_plugin_path /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet

# docker_version is the version of docker requires by packages
%global docker_version 1.13
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.6.1
# this is the version we obsolete up to. The packaging changed for Origin
# 1.0.6 and OSE 3.1 such that 'openshift' package names were no longer used.
%global package_refector_version 3.0.2.900
%global golang_version 1.9
# %commit and %os_git_vars are intended to be set by tito custom builders provided
# in the .tito/lib directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit b6d514572c8f57ca7343f36186e5f24389af1fa2
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# os_git_vars needed to run hack scripts during rpm builds
%{!?os_git_vars:
%global os_git_vars OS_GIT_MINOR=9+ OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS=rhel7 OS_GIT_MAJOR=3 OS_GIT_VERSION=v3.9.100 OS_GIT_TREE_STATE=clean OS_GIT_PATCH=100 KUBE_GIT_VERSION=v1.9.1+a0ce1bc657 OS_GIT_CATALOG_VERSION=v0.1.9.1 KUBE_GIT_COMMIT=a0ce1bc OS_GIT_COMMIT=e67ef66cc7 OS_IMAGE_PREFIX=registry.access.redhat.com/openshift3/ose ETCD_GIT_VERSION=v3.2.16 ETCD_GIT_COMMIT=121edf0
}

%if 0%{?skip_build}
%global do_build 0
%else
%global do_build 1
%endif
%if 0%{?skip_prep}
%global do_prep 0
%else
%global do_prep 1
%endif
%if 0%{?skip_dist}
%global package_dist %{nil}
%else
%global package_dist %{dist}
%endif

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

Name:           atomic-openshift
# Version is not kept up to date and is intended to be set by tito custom
# builders provided in the .tito/lib directory of this project
Version:        3.9.101
Release:        1%{?dist}
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
BuildRequires:  goversioninfo
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
Requires:       util-linux
Requires:       socat
Requires:       nfs-utils
Requires:       cifs-utils
Requires:       ethtool
Requires:       device-mapper-persistent-data >= 0.6.2
Requires:       conntrack-tools
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd
Obsoletes:      openshift-node < %{package_refector_version}
Obsoletes:      tuned-profiles-%{name}-node
Provides:       tuned-profiles-%{name}-node

%description node
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
%if 0%{do_prep}
%setup -q
%endif

%build
%if 0%{do_build}
%if 0%{make_redistributable}
# Create Binaries for all supported arches
%{os_git_vars} OS_BUILD_RELEASE_ARCHIVES=n make build-cross
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
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} OS_BUILD_RELEASE_ARCHIVES=n make build-cross
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} hack/build-go.sh vendor/github.com/onsi/ginkgo/ginkgo
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} unset GOPATH; cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog/hack/build-cross.sh
OS_ONLY_BUILD_PLATFORMS="${BUILD_PLATFORM}" %{os_git_vars} unset GOPATH; cmd/cluster-capacity/go/src/github.com/kubernetes-incubator/cluster-capacity/hack/build-cross.sh
%endif

# Generate man pages
%{os_git_vars} hack/generate-docs.sh
%endif

%install

PLATFORM="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
install -d %{buildroot}%{_bindir}

# Install linux components
for bin in oc oadm openshift template-service-broker
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
# Install oadm client executable
install -p -m 755 _output/local/bin/linux/amd64/oadm %{buildroot}%{_datadir}/%{name}/linux/oadm
install -p -m 755 _output/local/bin/darwin/amd64/oadm %{buildroot}/%{_datadir}/%{name}/macosx/oadm
install -p -m 755 _output/local/bin/windows/amd64/oadm.exe %{buildroot}/%{_datadir}/%{name}/windows/oadm.exe
%endif

# Install federation services
install -p -m 755 _output/local/bin/${PLATFORM}/hyperkube %{buildroot}%{_bindir}/

# Install cluster capacity
install -p -m 755 cmd/cluster-capacity/go/src/github.com/kubernetes-incubator/cluster-capacity/_output/local/bin/${PLATFORM}/hypercc %{buildroot}%{_bindir}/
ln -s hypercc %{buildroot}%{_bindir}/cluster-capacity

# Install service-catalog
install -p -m 755 cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog/_output/local/bin/${PLATFORM}/service-catalog %{buildroot}%{_bindir}/

# Install pod
install -p -m 755 _output/local/bin/${PLATFORM}/pod %{buildroot}%{_bindir}/

install -d -m 0755 %{buildroot}%{_unitdir}

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig

for cmd in \
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

# Install man1 man pages
install -d -m 0755 %{buildroot}%{_mandir}/man1
install -m 0644 docs/man/man1/* %{buildroot}%{_mandir}/man1/

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
for bin in oc openshift
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
sed -i "s|@@PACKAGE_LIST-VARIABLE@@|%{name} %{name}-clients %{name}-clients-redistributable %{name}-master %{name}-node %{name}-pod %{name}-recycle %{name}-sdn-ovs %{name}-tests|" $RPM_BUILD_ROOT/usr/sbin/%{name}-excluder
chmod 0744 $RPM_BUILD_ROOT/usr/sbin/%{name}-excluder

# Install docker-excluder script
sed "s|@@CONF_FILE-VARIABLE@@|${OS_CONF_FILE}|" contrib/excluder/excluder-template > $RPM_BUILD_ROOT/usr/sbin/%{name}-docker-excluder
sed -i "s|@@PACKAGE_LIST-VARIABLE@@|docker*1.14* docker*1.15* docker*1.16* docker*1.17* docker*1.18* docker*1.19* docker*1.20*|" $RPM_BUILD_ROOT/usr/sbin/%{name}-docker-excluder
chmod 0744 $RPM_BUILD_ROOT/usr/sbin/%{name}-docker-excluder

# Install migration scripts
install -d %{buildroot}%{_datadir}/%{name}/migration
install -p -m 755 contrib/migration/* %{buildroot}%{_datadir}/%{name}/migration/

%files
%doc README.md
%license LICENSE
%{_bindir}/openshift
%{_bindir}/hyperkube
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
%{_sysconfdir}/bash_completion.d/openshift
%defattr(-,root,root,0700)
%dir %config(noreplace) %{_sysconfdir}/origin
%ghost %dir %config(noreplace) %{_sysconfdir}/origin
%ghost %config(noreplace) %{_sysconfdir}/origin/.config_managed
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
%ghost %config(noreplace) %{_sysconfdir}/origin/master/admin.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/admin.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/admin.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/master/ca.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/ca.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/ca.serial.txt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/etcd.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/etcd.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master-config.yaml
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master.etcd-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master.etcd-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master.kubelet-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master.kubelet-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/master.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-master.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-master.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-master.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-registry.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-registry.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-registry.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-router.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-router.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/openshift-router.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/master/policy.json
%ghost %config(noreplace) %{_sysconfdir}/origin/master/serviceaccounts.private.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master/serviceaccounts.public.key
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
%{_bindir}/service-catalog

%files clients
%license LICENSE
%{_bindir}/oc
%{_bindir}/kubectl
%{_bindir}/oadm
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
%{_datadir}/%{name}/linux/oadm
%{_datadir}/%{name}/macosx/oadm
%{_datadir}/%{name}/windows/oadm.exe
%endif

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
* Tue Sep 17 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.101-1
- 

* Fri Sep 06 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.100-1
- UPSTREAM: 80852: apiextensions: 404 if request scope does not match crd scope
  (stefan.schimanski@gmail.com)

* Fri Aug 16 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.99-1
- 

* Fri Aug 16 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.98-1
- 

* Wed Aug 14 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.97-1
- 

* Mon Aug 12 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.96-1
- 

* Sat Aug 10 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.95-1
- UPSTREAM: 78991: log stale cache Info not Warning Bug 1573460
  (jottofar@redhat.com)

* Thu Aug 08 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.94-1
- 

* Tue Aug 06 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.93-1
- 

* Sat Aug 03 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.92-1
- UPSTREAM: <carry>: run ResourceQuota before ClusterResourceQuota
  (lukasz.szaszkiewicz@gmail.com)
- removes the depth flag while cloning the repo (lukasz.szaszkiewicz@gmail.com)
- UPSTREAM: <carry>: run ResourceQuota before ClusterResourceQuota
  (lukasz.szaszkiewicz@gmail.com)

* Sat Jul 27 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.91-1
- 

* Wed Jul 10 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.90-1
- 

* Sat Jul 06 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.89-1
- 

* Tue Jul 02 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.88-1
- 

* Sat Jun 29 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.87-1
- 

* Fri Jun 28 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.86-1
- 

* Wed Jun 26 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.85-1
- 

* Sat Jun 22 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.84-1
- 

* Sat Jun 15 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.83-1
- 

* Sat Jun 08 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.82-1
- 

* Sat May 04 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.81-1
- UPSTREAM: 76788: Clean links handling in cp's tar code (maszulik@redhat.com)

* Sat Apr 27 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.80-1
- 

* Sat Apr 20 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.79-1
- 

* Wed Apr 17 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.78-1
- 

* Sat Apr 13 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.77-1
- Allow egress-router to connect to its node's IP, via the SDN
  (danw@redhat.com)
- Bindmount /var/lib/iscsi rw for iscsi attach (mawong@redhat.com)

* Sat Mar 30 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.76-1
- 

* Sat Mar 23 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.75-1
- 

* Tue Mar 19 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.74-1
- 

* Sat Mar 16 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.73-1
- Revert "Initialize NetworkPolicy which-namespaces-are-in-use properly on
  restart" (danw@redhat.com)
- Revert "Clean up NetworkPolicies on NetNamespace deletion" (danw@redhat.com)
- UPSTREAM: 63977: pkg: kubelet: remote: increase grpc client default size
  (sjenning@redhat.com)

* Sat Mar 09 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.72-1
- UPSTREAM: 75037: Fix panic in kubectl cp command (maszulik@redhat.com)
- Initialize NetworkPolicy which-namespaces-are-in-use properly on restart
  (danw@redhat.com)
- Clean up NetworkPolicies on NetNamespace deletion (danw@redhat.com)
- Fix bug 1278683 for 3.9 (lxia@redhat.com)
- UPSTREAM: 74023: Fix reconstruction of FC volumes (jsafrane@redhat.com)
- UPSTREAM: 62467: fix nsenter GetFileType issue in containerized kubelet
  (jsafrane@redhat.com)

* Sat Feb 23 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.71-1
- Use BoundedFrequencyRunner to limit the rate of NetworkPolicy updates
  (danw@redhat.com)
- OVS test: Validate stdin values to bundle() call (rpenta@redhat.com)
- Revert https://github.com/openshift/origin/pull/19346 (rpenta@redhat.com)
- Fix fake ovs transaction to support ovs controller testing
  (rpenta@redhat.com)
- Changed ovs.Transaction from pseudo to real atomic transaction
  (rpenta@redhat.com)
- Added internal bundle() method to ovsExec interface (rpenta@redhat.com)
- ovs: add default 30s timeout to ovs-vsctl operations (dcbw@redhat.com)
- log OVS commands at level 4 (danw@redhat.com)

* Sat Feb 16 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.70-1
- 

* Sat Feb 09 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.69-1
- 

* Wed Feb 06 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.68-1
- 

* Sat Feb 02 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.67-1
- UPSTREAM: 67024: add CancelRequest to discovery round-tripper
  (jvallejo@redhat.com)
- UPSTREAM: 66929: add logging to find offending transports
  (jvallejo@redhat.com)

* Sat Jan 26 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.66-1
- UPSTREAM: 72980: Fix Cinder volume limits (hekumar@redhat.com)

* Mon Jan 21 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.65-1
- UPSTREAM: 60510: fix bug where character devices are not recognized
  (jsafrane@redhat.com)
- UPSTREAM: 62304: Remove isNotDir error check (jsafrane@redhat.com)

* Sat Jan 05 2019 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.64-1
- UPSTREAM: 70580: PV Controller -- fix recycling (tsmetana@redhat.com)

* Sat Dec 29 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.63-1
- 

* Sat Dec 22 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.62-1
- 

* Wed Dec 19 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.61-1
- 

* Tue Dec 18 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.60-1
- UPSTREAM: 61378: `--force` only takes effect when `--grace-period=0`
  (jvallejo@redhat.com)

* Sat Dec 15 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.59-1
- 

* Sat Dec 08 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.58-1
- Upstream: 57994: Fix vm cache in concurrent case in azure_util.go
  (jchaloup@redhat.com)

* Sat Dec 01 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.57-1
- 

* Fri Nov 30 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.56-1
- 

* Wed Nov 28 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.55-1
- Ensure Egress IP rules are recreated after firewalld reload (danw@redhat.com)
- Add bbennett@redhat.com (knobunc on github) to 3.9 OWNERS to approve
  backports (bbennett@redhat.com)
- proxy: Don't allow multiple calls to switchService (cdc@redhat.com)
- UPSTREAM: 69565: Fixed subpath in containerized kubelet (jsafrane@redhat.com)
- UPSTREAM: 68741: Fixed subpath cleanup when /var/lib/kubelet is a symlink
  (jsafrane@redhat.com)
- UPSTREAM: <carry>: Add mounter.EvalHostSymlinks (jsafrane@redhat.com)
- UPSTREAM: <carry>: Add Nsenter.EvalSymlinks (jsafrane@redhat.com)
- UPSTREAM: 58646: Change the portworx volume attribute SupportsSELinux to
  false (mawong@redhat.com)
- [3.9] Fix potential segfault in kubelet volume reconstruction
  (tsmetana@redhat.com)
- UPSTREAM: 59466: Gluster: Remove provisioner configuration from info message.
  (mawong@redhat.com)

* Sat Nov 24 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.54-1
- UPSTREAM: 58089: Create proper volumeSpec during ConstructVolumeSpec
  (jsafrane@redhat.com)

* Sat Nov 17 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.53-1
- 

* Sat Nov 17 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.52-1
- Explicitly set the MTU on the tun0 interface (bbennett@redhat.com)
- UPSTREAM: 70291: Do not delete vspehre node on shutdown (hekumar@redhat.com)
- Don't allow pods to send VXLAN packets out of the SDN (danw@redhat.com)
- UPSTREAM: 67825: Fix multiattach issue on vsphere (hekumar@redhat.com)

* Fri Nov 09 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.51-1
- UPSTREAM: 00000: Verify backend upgrade (deads@redhat.com)
- UPSTREAM: 60069: Fix race in healthchecking etcds leading to crashes
  (deads@redhat.com)

* Sat Nov 03 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.50-1
- bump(*) (bparees@redhat.com)
- UPSTREAM: 68626: Fix mount options for netdev (hekumar@redhat.com)

* Sat Oct 27 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.49-1
- Fix check to ignore the current route from the set of displaced routes. fixes
  bugz #1624078 (smitram@gmail.com)

* Sat Oct 20 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.48-1
- service-catalog: bump osb-client version, close connections, fixes 1638726
  (jaboyd@redhat.com)

* Sun Oct 14 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.47-1
- 

* Wed Oct 03 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.46-1
- migrate: ignore resources that cannot be listed and updated
  (mkhan@redhat.com)
- Bug 1631087 - Accept logFormat when passed to audit config
  (maszulik@redhat.com)
- UPSTREAM: 68674: Fix isDeviceOpen check for volumes where device name in node
  is wrong (hekumar@redhat.com)

* Tue Sep 18 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.45-1
- 

* Thu Sep 13 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.44-1
- [3.9] bump(github.com/evanphx/json-patch):
  f195058310bd062ea7c754a834f0ff43b4b63afb (eparis@redhat.com)

* Fri Sep 07 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.43-1
- 

* Thu Sep 06 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.42-1
- UPSTREAM: 68010: apiserver: forward panic in WithTimeout filter
  (stefan.schimanski@gmail.com)
- hack/cherry-pick.sh: simplify PR cherry-pick (stefan.schimanski@gmail.com)
- hack/cherry-pick.sh: apply merge commit by default and add APPLY_PR_COMMITS
  for old mode (stefan.schimanski@gmail.com)
- hack/cherry-pick.sh: add UPSTREAM_BRANCH var (stefan.schimanski@gmail.com)
- UPSTREAM: <carry>: change version of Kubernetes to check against
  (hekumar@redhat.com)
- UPSTREAM: 62919: Fix vsphere detach on 1.8 to 1.9 upgrade
  (hekumar@redhat.com)
- UPSTREAM: 62220: Fix detach disk when VM is not found (hekumar@redhat.com)

* Mon Aug 20 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.41-1
- Remove the DROP_SYN_DURING_RESTART iptables rules (bbennett@redhat.com)
- set full user info for SAR check on template readiness checks
  (bparees@redhat.com)
- Correct quoting issue with Forwarded field in HAproxy 1.8
  (alberto.rodriguez.peon@cern.ch)
- Fix VRRP check script (ichavero@redhat.com)
- UPSTREAM: 66397: Update scheduler to use different limits for m5/c5
  (hekumar@redhat.com)

* Mon Jul 30 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.40-1
- 

* Fri Jul 27 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.39-1
- UPSTREAM: 57432: Add cache for VirtualMachinesClient.Get in azure cloud
  provider (jchaloup@redhat.com)
- UPSTREAM: 63146: Remove patch retry conflict detection (jliggitt@redhat.com)

* Wed Jul 25 2018 AOS Automation Release Team <aos-team-art@redhat.com> 3.9.38-1
- UPSTREAM: 66350: Start cloudResourceSyncsManager before getNodeAnyWay
  (initializeModules) (jchaloup@redhat.com)
- UPSTREAM: 65691: Add validation of resourceGroup option (jsafrane@redhat.com)
- UPSTREAM: 65443: Move configuration of resource group in storage class
  (jsafrane@redhat.com)
- UPSTREAM: 65516: fix azure disk creation issue when specifying external
  resource group (jsafrane@redhat.com)
- UPSTREAM: 65217: add external resource group support for azure disk
  (jsafrane@redhat.com)

* Tue Jul 17 2018 Tim Bielawa <tbielawa@redhat.com> 3.9.37-1
- 

* Tue Jul 17 2018 Tim Bielawa <tbielawa@redhat.com> 3.9.36-1
- 

* Mon Jul 16 2018 Tim Bielawa <tbielawa@redhat.com> 3.9.35-1
- Update system container image to mount flexvolume plugins
  (hekumar@redhat.com)

* Fri Jul 13 2018 Tim Bielawa <tbielawa@redhat.com> 3.9.34-1
- UPSTREAM: 65549: Fix flexvolumes in containerized envs (hekumar@redhat.com)
- UPSTREAM: 65226: Put all the node address cloud provider retrival complex
  logic into cloudResourceSyncManager (jchaloup@redhat.com)
- Fix a crash on a certain type of unsupported NetworkPolicy (danw@redhat.com)

* Mon Jul 09 2018 Tim Bielawa <tbielawa@redhat.com> 3.9.33-1
- Fix deployer pod tolerations (mfojtik@redhat.com)
- Replace Perl with Bash in router echo test server (miciah.masters@gmail.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from c3e3071633..abec8bcc89 (jpeeler@redhat.com)
- up: default openshift imagepolicy admission (mfojtik@redhat.com)

* Thu Jul 05 2018 Tim Bielawa <tbielawa@redhat.com> 3.9.32-1
- UPSTREAM: 63926: Avoid unnecessary calls to the cloud provider
  (miciah.masters@gmail.com)
- UPSTREAM: 64860:checkLimitsForResolvConf for the pod create and update events
  instead of checking periodically (ravisantoshgudimetla@gmail.com)
- UPSTREAM: 59602: Change VMware provider ID to uuid (jsafrane@redhat.com)
- UPSTREAM: 63875: make TestGetServerGroupsWithTimeout more reliable
  (maszulik@redhat.com)
- UPSTREAM: 63848: Deflake discovery timeout test (maszulik@redhat.com)
- UPSTREAM: 63086: Fix discovery default timeout test (maszulik@redhat.com)
- UPSTREAM: 62733: Set a default request timeout for discovery client
  (maszulik@redhat.com)
- Apps: Fix DC recreate strategy to wait only for running pods
  (tnozicka@gmail.com)
- serviceaccounts: do not manage pull secrets created by third parties
  (mfojtik@redhat.com)
- up: default openshift imagepolicy admission (mfojtik@redhat.com)
- Hoist etcd startup first for all-in-one (jliggitt@redhat.com)
- UPSTREAM: 61459: etcd client add dial timeout (xuzhonghu@huawei.com)
- Fix bugz 1582875 - note: commit is specific to enterprise 3.9 haproxy router.
  Fix bug where secured wildcard routes takes over all routes in the specific
  subdomain. (smitram@gmail.com)

* Tue Jun 12 2018 Justin Pierce <jupierce@redhat.com> 3.9.31-1
- Fixing s2i build unit tests (adam.kaplan@redhat.com)
- Revert "UPSTREAM: docker/distribution: <carry>: do not strip docker.io image
  path in client" (sjenning@redhat.com)
- [3.9] Check s2i Assemble-User Is Root (adam.kaplan@redhat.com)
- add support to build different tags in hack script (jpeeler@redhat.com)
- build: handle empty commits in bitbucket webhook payload (mfojtik@redhat.com)
- Update NetworkPolicy code to not crash on ipBlock (danw@redhat.com)
- [3.9] UPSTREAM: 57991 Fix exists status for azure GetLoadBalancer
  (jtanenba@redhat.com)
- Prevent rolebinding deletion if it is protected (simo@redhat.com)
- Prevent rolebinding deletion if it is protected (simo@redhat.com)

* Sat May 26 2018 Justin Pierce <jupierce@redhat.com> 3.9.30-1
- Revert "UPSTREAM: 61459: etcd client add dial timeout" (jliggitt@redhat.com)
- UPSTREAM: 61459: etcd client add dial timeout (xuzhonghu@huawei.com)
- update config test (jvallejo@redhat.com)
- UPSTREAM: <carry>: ensure config usable before proceeding
  (jvallejo@redhat.com)
- UPSTREAM: opencontainers/runc: 1805: fix systemd cpu quota for -1
  (sjenning@redhat.com)
- make the docker registry secret always prime (deads@redhat.com)
- update docker config secret to include image-registry.openshift-image-
  registry.svc (deads@redhat.com)
- UPSTREAM: 57020: ignore images in used by running containers when GC
  (sjenning@redhat.com)
- Add option to ignore image reference errors (agladkov@redhat.com)
- Remove test reference to deleted content (ccoleman@redhat.com)
- hack/verify-gofmt.sh | xargs -n 1 gofmt -s -w (jchaloup@redhat.com)
- Dump route logs when test fails (ccoleman@redhat.com)
- Use the correct example repo (deleted from master) (ccoleman@redhat.com)
- The kube major/minor version is missing from /version (ccoleman@redhat.com)
- UPSTREAM: 63832: Close all kubelet->API connections on heartbeat failure
  (jliggitt@redhat.com)
- UPSTREAM: 63832: Always track kubelet -> API connections
  (jliggitt@redhat.com)
- UPSTREAM: docker/distribution: <carry>: do not strip docker.io image path in
  client (sjenning@redhat.com)
- UPSTREAM: 62874: dockershim/sandbox: clean up pod network even if SetUpPod()
  failed (dcbw@redhat.com)
- Bug fix to sort non-wildcard and wildcard groups separately.
  (smitram@gmail.com)
- UPSTREAM: 59440: Use SetInformers method to register for Node events
  (hekumar@redhat.com)
- reload-haproxy: changes how map files are sorted (jmprusi@keepalive.io)
- Revert "handle SIGINT/TERM in cmd/openshift" (amcdermo@redhat.com)

* Wed May 09 2018 Justin Pierce <jupierce@redhat.com> 3.9.29-1
- UPSTREAM: 61331: Fix a bug where malformed paths don't get written to the
  destination dir. (jliggitt@redhat.com)
- new-app: remove check for volumes in new-app test (mfojtik@redhat.com)
- node, syscontainer: bind mount /opt/cni/bin from the host
  (vrutkovs@redhat.com)

* Sun May 06 2018 Justin Pierce <jupierce@redhat.com> 3.9.28-1
- Ensure Continue token is proxied for openshift RBAC list calls
  (jliggitt@redhat.com)
- UPSTREAM: 62543: Timeout on instances.NodeAddresses cloud provider request
  (jchaloup@redhat.com)
- Bug 1567532 - Unidle handling in router should ignore headless services.
  (rpenta@redhat.com)
- Sync NetworkPolicy test with upstream a bit (danw@redhat.com)

* Thu Apr 26 2018 Justin Pierce <jupierce@redhat.com> 3.9.27-1
- 

* Wed Apr 25 2018 Justin Pierce <jupierce@redhat.com> 3.9.26-1
- Bumping version over out of band build (jupierce@redhat.com)
- Improve patching ovs flow rules in UpdateEgressNetworkPolicyRules
  (rpenta@redhat.com)
- Make DNS to the local node IP bypass auto-egress-IP routing (danw@redhat.com)
- Fix use of cookies in HostSubnet deletion (danw@redhat.com)
- Use cookies in HostSubnet-related OVS flows to avoid misdeletions
  (danw@redhat.com)
- Refactor node HostSubnet code into its own object for ease of testing
  (danw@redhat.com)
- update policy files (deads@redhat.com)
- Revert "UPSTREAM: <carry>: Remove write permissions on daemonsets from
  Kubernetes bootstrap policy" (deads@redhat.com)
- UPSTREAM: <carry>: filter daemonset nodes by namespace node selectors
  (deads@redhat.com)

* Wed Apr 18 2018 Justin Pierce <jupierce@redhat.com> 3.9.24-1
- 

* Wed Apr 18 2018 Justin Pierce <jupierce@redhat.com> 3.9.23-1
- Inject openshift version variables in k8s (deads@redhat.com)
- UPSTREAM: <drop>: Fix openshift recycler images (deads@redhat.com)
- Switch to 1.9 conformance tests for Origin (ccoleman@redhat.com)
- Require docker >= 1.13 (sdodson@redhat.com)
- UPSTREAM: 00000: set IdleConnTimeout for etcd2 client (jliggitt@redhat.com)
- UPSTREAM: 58008: etcd client: add keepalive (ryan.phillips@coreos.com)

* Sun Apr 15 2018 Justin Pierce <jupierce@redhat.com> 3.9.22-1
- UPSTREAM: 62416: kuberuntime: logs: reduce logging level on waitLogs msg
  (sjenning@redhat.com)

* Fri Apr 13 2018 Justin Pierce <jupierce@redhat.com> 3.9.21-1
- update prometheus 2.0.0 -> 2.2.1 (pgier@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from c3e3071633..b65141ce43 (jaboyd@redhat.com)
- UPSTREAM: 59931: do not delete node in openstack, if those still exist in
  cloudprovider (sjenning@redhat.com)
- handle SIGINT/TERM in cmd/openshift (amcdermo@redhat.com)
- UPSTREAM: 59701: Display pvc conditions with describe command
  (hekumar@redhat.com)
- Use a dummy ns.NetNS in sdn_cni_plugin_test.go (danw@redhat.com)

* Tue Apr 10 2018 Justin Pierce <jupierce@redhat.com> 3.9.20-1
- UPSTREAM: <carry> prevent save-artifact tar extraction from overwriting files
  outside the working dir (bparees@redhat.com)
- Further dind start certificate generation serialization (danw@redhat.com)
- dind: serialize master and node cert generation (danw@redhat.com)

* Wed Apr 04 2018 Justin Pierce <jupierce@redhat.com> 3.9.19-1
- 

* Wed Apr 04 2018 Justin Pierce <jupierce@redhat.com> 3.9.18-1
- 

* Wed Apr 04 2018 Justin Pierce <jupierce@redhat.com> 3.9.17-1
- Store logs uncompressed in the networking test artifacts (danw@redhat.com)

* Mon Apr 02 2018 Justin Pierce <jupierce@redhat.com> 3.9.16-1
- UPSTREAM: 61284: Fix creation of subpath with SUID/SGID directories.
  (hchen@redhat.com)
- Additional audit tests (maszulik@redhat.com)
- Register audit/v1beta1 for master config (maszulik@redhat.com)
- Fix initContainer name lookup for ImageChange trigger
  (alexandre.lossent@cern.ch)

* Thu Mar 29 2018 Justin Pierce <jupierce@redhat.com> 3.9.15-1
- setup binary input for custom builds properly (bparees@redhat.com)
- UPSTREAM: 61480: Fix subpath mounting for unix sockets (hekumar@redhat.com)
- switch reversed old/new objects to validation (deads@redhat.com)
- image-strategy: unset image signature annotations (miminar@redhat.com)
- signature-controller: stop adding managed annotation (miminar@redhat.com)
- Default the kubelet IPTablesMasqueradeBit to the same value as the kube-proxy
  IPTablesMasqueradeBit (danw@redhat.com)
- UPSTREAM: 61294: Fix cpu cfs quota flag with pod cgroups (decarr@redhat.com)
- Add timestamps to bash log output (skuznets@redhat.com)
- UPSTREAM: 57978: [vSphere] Renews cached NodeInfo with new vSphere connection
  (hekumar@redhat.com)
- Fix DC selectors for autoscaling (tnozicka@gmail.com)
- UPSTREAM: 60978: Fix use of "-w" flag to iptables-restore (danw@redhat.com)
- NetworkCheck diagnostic: use admin kubeconfig (lmeyer@redhat.com)
- diagnostics: reorg network diags under cluster/network (lmeyer@redhat.com)
- diagnostics: reorg pod diags under client/pod (lmeyer@redhat.com)
- diagnostics: extract commandRunFunc to util pkg (lmeyer@redhat.com)
- diagnostics logs: use local IsTerminalWriter (lmeyer@redhat.com)
- bump jenkins route request timeout based on online testing
  (gmontero@redhat.com)
- nest daemonsets under their services (jvallejo@redhat.com)
- Warn when AuditFilePath is relative (maszulik@redhat.com)
- diagnostics: missing logging project shouldn't be fatal error
  (jwozniak@redhat.com)
- Rearrange egressip internals, add duplication tests (danw@redhat.com)
- Fix egressip handling when a NetNamespace is updated (danw@redhat.com)
- Have multiple egress IP ovscontroller methods rather than one confusing one
  (danw@redhat.com)
- egressip_test updates (danw@redhat.com)
- Add assertOVSChanges helper to egressip_test (danw@redhat.com)

* Thu Mar 22 2018 Justin Pierce <jupierce@redhat.com> 3.9.14-1
- UPSTREAM: 61373: Fix subpath reconstruction (hekumar@redhat.com)

* Wed Mar 21 2018 Justin Pierce <jupierce@redhat.com> 3.9.13-1
- UPSTREAM: revert: 6d2d90f: opencontainers/runc: 1683: Fix race against
  systemd (sjenning@redhat.com)
- UPSTREAM: revert: 9bf708a: opencontainers/runc: 1754: Add timeout while
  waiting for StartTransinetUnit completion signal (sjenning@redhat.com)

* Mon Mar 19 2018 Justin Pierce <jupierce@redhat.com> 3.9.12-1
- 

* Thu Mar 15 2018 Justin Pierce <jupierce@redhat.com> 3.9.11-1
- UPSTREAM: 61193: bugfix(mount): lstat with abs path of parent instead of
  '/..' (291271447@qq.com)

* Thu Mar 15 2018 Justin Pierce <jupierce@redhat.com> 3.9.10-1
- Allow a new OS_PUSH_BASE_REPO flag to push-release (ccoleman@redhat.com)
- UPSTREAM: 58433: bugfix(mount): lstat with abs path of parent
  (hekumar@redhat.com)
- Update policy tests to reflect removal of write access on daemonsets
  (mkhan@redhat.com)
- UPSTREAM: <carry>: Remove write permissions on daemonsets from Kubernetes
  bootstrap policy (mkhan@redhat.com)

* Wed Mar 14 2018 Justin Pierce <jupierce@redhat.com> 3.9.9-1
- UPSTREAM: 61107: Add atomic writer subpath e2e tests (jliggitt@redhat.com)
- UPSTREAM: 61107: Detect backsteps correctly in base path detection
  (jliggitt@redhat.com)
- UPSTREAM: 61045: subpath fixes (jsafrane@redhat.com)

* Tue Mar 13 2018 Justin Pierce <jupierce@redhat.com> 3.9.8-1
- require templateinstance delete, not update, on unbind (bparees@redhat.com)

* Sun Mar 11 2018 Justin Pierce <jupierce@redhat.com> 3.9.7-1
- 

* Sun Mar 11 2018 Justin Pierce <jupierce@redhat.com> 3.9.6-1
- 

* Sat Mar 10 2018 Justin Pierce <jupierce@redhat.com> 3.9.5-1
- 

* Thu Mar 08 2018 Justin Pierce <jupierce@redhat.com> 3.9.4-1
- UPSTREAM: opencontainers/runc: 1754: Add timeout while waiting for
  StartTransinetUnit completion signal (sjenning@redhat.com)
- Don't try to delete (nonexistent) OVS flows for headless/external services
  (danw@redhat.com)
- diagnostics: AggregatedLogging ClusterRoleBindings false negative fix
  (jwozniak@redhat.com)
- UPSTREAM: docker/docker: 36517: ensure hijackedConn implements CloseWrite
  function (jminter@redhat.com)
- UPSTREAM: 60490: Volume deletion should be idempotent (jsafrane@redhat.com)
- UPSTREAM: drop: Add feature gating for subpath (jsafrane@redhat.com)
- sdn: fix CNI IPAM data dir (dcbw@redhat.com)
- UPSTREAM: <drop>: Revert "Merge pull request #18554 from rootfs/pr-58177"
  (hekumar@redhat.com)
- Add migrate command for legacy HPAs (sross@redhat.com)
- UPSTREAM: 57202: Fix format string in describers (jsafrane@redhat.com)
- UPSTREAM: 58977: Fix pod sandbox privilege. (runcom@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from b758460ba7..c3e3071633 (jpeeler@redhat.com)

* Tue Mar 06 2018 Justin Pierce <jupierce@redhat.com> 3.9.3-1
- Allow all nodes to run upstream kube tests (jpeeler@redhat.com)
- adjust jenkins template setting to account for effects of constrained default
  max heap (gmontero@redhat.com)
- UPSTREAM: 59365: Fix StatefulSet set-based selector bug (tnozicka@gmail.com)
- Fix handleDeleteSubnet() to release network from subnet allocator.
  (rpenta@redhat.com)
- audit doesn't respect embedded config (deads@redhat.com)
- Remove Service Catalog Controller Manager service (marko.luksa@gmail.com)
- Configure Service Catalog Controller Manager to bind to port 8080
  (marko.luksa@gmail.com)
- UPSTREAM: 54530: api: validate container phase transitions
  (sjenning@redhat.com)
- UPSTREAM: 60342: Fix nested volume mounts for read-only API data volumes
  (joesmith@redhat.com)
- UPSTREAM: 59170: Fix kubelet PVC stale metrics (jsafrane@redhat.com)
- Fixes: cannot prune builds on buildConfig change (cdaley@redhat.com)
- bump(*) (deads@redhat.com)
- pin dependencies for 3.9 (deads@redhat.com)

* Fri Mar 02 2018 Justin Pierce <jupierce@redhat.com> 3.9.2-1
- 

* Tue Feb 27 2018 Justin Pierce <jupierce@redhat.com> 3.9.1-1
- Moving 3.9 to release mode (jupierce@redhat.com)
- Workaround for #18762 (obulatov@redhat.com)
- apps: stop dc cancellation flake (mfojtik@redhat.com)
- UPSTREAM: 60301: Fix Deployment with Recreate strategy not to wait on Pods in
  terminal phase (mfojtik@redhat.com)
- UPSTREAM: 60457: tests: e2e: empty msg from channel other than stdout should
  be non-fatal (sjenning@redhat.com)
- UPSTREAM: 60306: Only run connection-rejecting rules on new connections
  (danw@redhat.com)
- UPSTREAM: 57461: Don't create no-op iptables rules for services with no
  endpoints (danw@redhat.com)
- UPSTREAM: 56164: Split out a KUBE-EXTERNAL-SERVICES chain so we don't have to
  run KUBE-SERVICES from INPUT (danw@redhat.com)
- UPSTREAM: 57336: Abstract some duplicated code in the iptables proxier
  (danw@gnome.org)
- UPSTREAM: 60430: don't use storage cache during apiserver unit test
  (mfojtik@redhat.com)
- UPSTREAM: 60299: apiserver: fix testing etcd config for etcd 3.2.16
  (stefan.schimanski@gmail.com)
- add tests (jvallejo@redhat.com)
- add daemonsets to status graph (jvallejo@redhat.com)
- dind: switch images to Fedora 27 (dcbw@redhat.com)
- dind: remove CentOS 7 Dockerfile (dcbw@redhat.com)
- UPSTREAM: 57480: Fix build and test errors from etcd 3.2.13 upgrade
  (mfojtik@redhat.com)
- bump(*): update etcd to 3.2.16 and grpc to 1.7.5 (mfojtik@redhat.com)
- add more project validation (hasha@redhat.com)
- tags existing deployment nodes as "found" (jvallejo@redhat.com)
- add more example queries, possible alert queries (gmontero@redhat.com)
- add strategy type to build metrics (gmontero@redhat.com)
- UPSTREAM: 59386: Scheduler - not able to read from config file if configmap
  is not found (ravisantoshgudimetla@gmail.com)
- Minor cleanups to sdn-cni-plugin (danw@redhat.com)
- update nginx-config.template (307292795@qq.com)

* Sun Feb 25 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.53.0
- 

* Sun Feb 25 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.52.0
- relax aggressive timeouts in declarative pipeline example
  (gmontero@redhat.com)
- Skip managed images when importing signatures (obulatov@redhat.com)
- Debugging improvements to egressip_test.go (danw@redhat.com)
- Fix reassignment of egress IP after removal, add test (danw@redhat.com)
- Guarantee that SerialFileGenerator starts at 2 (mkhan@redhat.com)
- add configmap volume description (jvallejo@redhat.com)
- Correctly flush stale ovs rules on Node startup (jtanenba@redhat.com)
- Boring change: Use lowercase for returning user errors (rpenta@redhat.com)
- oc adm client should forbid isolation for 'default' project
  (rpenta@redhat.com)
- SDN master controller should not allow isolation for 'default' project
  (rpenta@redhat.com)

* Fri Feb 23 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.51.0
- Add networking team members to OWNERS file (bbennett@redhat.com)

* Thu Feb 22 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.50.0
- 

* Thu Feb 22 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.49.0
- Add enj to generated man page OWNERS (mkhan@redhat.com)

* Thu Feb 22 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.48.0
- Prevent login spam on large clusters (oats87g@gmail.com)
- Fix govet error - formatting in glog.V(0).Info() call (cdaley@redhat.com)
- Fixes new-app segmentation fault on invalid build strategy
  (cdaley@redhat.com)
- add more graph methods (jvallejo@redhat.com)
- Add a containerized node script for bootstrap equivalent
  (ccoleman@redhat.com)
- The prometheus e2e isn't checking for its pods correctly
  (ccoleman@redhat.com)
- Limit logging to provider name (jliggitt@redhat.com)
- Move stampTooOld into Check function (nakayamakenjiro@gmail.com)
- Output field of struct of discovered systemd unit (nakayamakenjiro@gmail.com)
- Fix AnalyzeLogs to provide more clear debug message
  (nakayamakenjiro@gmail.com)
- Fix typo Seaching to Searching (nakayamakenjiro@gmail.com)
- Fixes index out of range error on oc new-app --template foo
  (cdaley@redhat.com)
- Correctly handle newlines in SerialFileGenerator (mkhan@redhat.com)

* Tue Feb 20 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.47.0
- upgrade prometheus alertmanager v0.9.1 -> v0.13.0 (pgier@redhat.com)
- Fix panic in error printing in DC controller (tnozicka@gmail.com)
- Fixes BuildConfigInstantiateFailure event on race condition
  (cdaley@redhat.com)
- Store pod IP in OVS external-ids and use that on teardown (danw@redhat.com)
- Added dcbw to pkg/cmd/OWNERS (rpenta@redhat.com)
- Revert "allow webconsole to discover cluster information"
  (spadgett@redhat.com)
- sdn: rationalize data directories between kubelet, CNI, and SDN
  (dcbw@redhat.com)
- node: remove un-needed kubelet flags (dcbw@redhat.com)

* Mon Feb 19 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.46.0
- masterPublicURL must be both internally and externally accessible
  (mkhan@redhat.com)
- restore qps and burst for scheduler and kbue-controller-manageR
  (deads@redhat.com)
- the --source-image flag should count as a source input the --strategy flag
  should override the build strategy in all source repositories
  (cdaley@redhat.com)
- Update custom tito tagger for new version (skuznets@redhat.com)
- Fix periodic reconciliation for DCs (tnozicka@gmail.com)
- add publishing rules for openshift/kubernetes repo (mfojtik@redhat.com)
- UPSTREAM: 59923: Rework volume manager log levels (sjenning@redhat.com)
- UPSTREAM: 59873: Fix DownwardAPI refresh race (sjenning@redhat.com)
- note some unused reasons; track reasons for all phases; add reason to
  constant time metric (gmontero@redhat.com)
- auto-panic and become unhealthy on too many grpc connections
  (deads@redhat.com)
- Change rejected routes error message to verbose logging (ichavero@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from b69b4a6c80..b758460ba7 (jaboyd@redhat.com)
- Revert "Allow insecure path Allow routes to override Redirect routes without
  breaking the reverse" (jtanenba@redhat.com)
- preserve namespace on imagestream server-side export (jvallejo@redhat.com)
- make TSB conformant with respect to multiple parallel operations
  (jminter@redhat.com)
- use the normal loopback connection for loopback to the apiserver
  (deads@redhat.com)
- automatically remove hostsubnets with no nodes on startup
  (jtanenba@redhat.com)
- add an admission decorator that allows skipping some admission checks based
  on namespace labels (deads@redhat.com)
- Allow passing -gcflags during build (maszulik@redhat.com)
- diagnostics: introduce AppCreate (lmeyer@redhat.com)
- UPSTREAM: 58375: Recheck if transformed data is stale when doing live lookup
  during update (jliggitt@redhat.com)

* Thu Feb 15 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.45.0
- 

* Thu Feb 15 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.44.0
- Fix invalid SuggestFor for oc image (nakayamakenjiro@gmail.com)

* Thu Feb 15 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.43.0
- Revert "Admission plugin: openshift.io/ImageQualify" (sjenning@redhat.com)
- allow a panic to crash the server after a delay (deads@redhat.com)
- enable additional osin server error logging (mrogers@redhat.com)
- bump(*) (mrogers@redhat.com)
- unexport scheduler and add health warning (jminter@redhat.com)
- stop scheduler from advancing through empty buckets without accept from
  ratelimiter (jminter@redhat.com)
- update osin version in glide.yaml (mrogers@redhat.com)
- Add remove test with mismatched rolebinding name (simo@redhat.com)
- Don't call DeleteHostSubnetRules on a replayed Deleted event
  (danw@redhat.com)
- add registry.centos.org to default whitelist (bparees@redhat.com)
- fix typo in HACKING.md (mrogers@redhat.com)
- move pkg/build/{prune,cmd} into pkg/oc/cli/builds (mfojtik@redhat.com)
- move pkg/apps/prune to pkg/oc (mfojtik@redhat.com)
- Bug 1538922 - Fix diagnostics for AggregatedLogging (jwozniak@redhat.com)
- Refactor minResourceLimits to filter by resourceName parameter
  (amcdermo@redhat.com)
- Inject LimitRange lister implementation via plugin configuration
  (amcdermo@redhat.com)
- Fix integration tests as CRO will now floor to CPU & Memory limits
  (amcdermo@redhat.com)
- Add admission tests for namespace minimums (amcdermo@redhat.com)
- Do not mutate resource limits below namespace minimums (amcdermo@redhat.com)
- Delete unused LimitRangerActions (amcdermo@redhat.com)
- Replace loop index with generation number of istag
  (nakayamakenjiro@gmail.com)
- UPSTREAM: 59767: kubelet: check for illegal phase transition
  (sjenning@redhat.com)
- add origin test (jvallejo@redhat.com)
- UPSTREAM: 59506: fix --watch on multiple requests (jvallejo@redhat.com)
- Fix oc policy remove-user to remove rolebindings too (simo@redhat.com)
- implements ExistenceChecker intf for deployments and replicasets
  (jvallejo@redhat.com)
- add publishing rules for repo synchronization (mfojtik@redhat.com)
- return containerID from completed pod (deads@redhat.com)
- complete the cluster up struct in the Complete method (deads@redhat.com)
- simplify openshift controller manager startup (deads@redhat.com)
- Make image pruner tolerate since to empty docker image reference
  (agladkov@redhat.com)
- JenkinsPipelineStrategy builds should not be pruned on BuildConfig save
  (cdaley@redhat.com)
- Update web console template labels (spadgett@redhat.com)
- generated (deads@redhat.com)
- add --local flag for adm router and registry (deads@redhat.com)
- UPSTREAM: 58177: Redesign and implement volume reconstruction work
  (hchen@redhat.com)
- Make it clear deprecation apply also for later servers (simo@redhat.com)
- UPSTREAM: 59350: Do not recycle volumes that are used by pods
  (jsafrane@redhat.com)
- cli: fix status for standalone deployment (mfojtik@redhat.com)
- Fix cleanup of auto egress IPs when deleting a NetNamespace (danw@redhat.com)
- do initialization steps for cluster up during initialization
  (deads@redhat.com)
- UPSTREAM: <carry>: patch the upstream SA token controller and use it
  (deads@redhat.com)
- UPSTREAM: 59569: Do not ignore errors from EC2::DescribeVolume in DetachDisk
  (tsmetana@redhat.com)
- Re-enable formerly-temporarily-disabled tests, kill some dead code
  (danw@redhat.com)
- Multiple template creation (sejug@redhat.com)
- UPSTREAM: 58439: Fix loading structured admission plugin config
  (jliggitt@redhat.com)
- UPSTREAM: 58439: Surface error loading admission plugin config
  (jliggitt@redhat.com)
- Add SELinux label to local-storage provisioner (jsafrane@redhat.com)
- remove-role-from-user outputs error when target was not found
  (nakayamakenjiro@gmail.com)
- fix the front proxy CA name (mrogers@redhat.com)
- Grafna deployment update. (mrsiano@gmail.com)
- Parametrize local-storage template with image name (jsafrane@redhat.com)
- UPSTREAM: 58794: Resize mounted volumes (hekumar@redhat.com)
- Add .NET Core CentOS ImageStreams (tom.deseyn@gmail.com)
- UPSTREAM: 58685: Fill size attribute for the OpenStack V3 API volumes
  (tsmetana@redhat.com)
- Never evict pods on dind nodes (danw@redhat.com)

* Fri Feb 09 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.42.0
- update oc in build local images (deads@redhat.com)
- host diagnostics: update master unit names (lmeyer@redhat.com)
- add declarative pipeline example to build extended test suite
  (gmontero@redhat.com)
- Change the haproxy reload detection to tolerate routes named localhost
  (bbennett@redhat.com)
- UPSTREAM: 58316: set fsGroup by securityContext.fsGroup in azure file
  (hchen@redhat.com)
- Allow import image into empty imagestream (agladkov@redhat.com)
- add retry for 401 errors to image imported to try pull image without
  authentication. This is to eliminate case when we try pull public images with
  wrong/expired secret and it blocks all imports (m.judeikis@gmail.com)
- NewRequest signature applied in origin code (maszulik@redhat.com)
- UPSTREAM: 51042: Allow passing request-timeout from NewRequest all the way
  down (maszulik@redhat.com)
- Add tnozicka to pkg/image approvers since we own part of that code
  (tnozicka@gmail.com)
- Add tests for annotation trigger reconciliation (tnozicka@gmail.com)
- UPSTREAM: 59297: Improve error returned when fetching container logs during
  pod termination (joesmith@redhat.com)
- Fix annotation trigger to reconcile on container image change
  (tnozicka@gmail.com)
- adjust newapp/newbuild error messages (arg classification vs. actual
  processing) (gmontero@redhat.com)
- UPSTREAM: 58415: Improve messaging on resize (hekumar@redhat.com)
- oc new-build --push-secret option (mdame@redhat.com)
- UPSTREAM: 59449: Fix to register priority function ResourceLimitsPriority
  correctly. (avagarwa@redhat.com)
- add support for deployments in oc status (mfojtik@redhat.com)
- Restrict login redirects to server-relative URLs (jliggitt@redhat.com)
- bump(*) (rchopra@redhat.com)
- update cockroachdb/cmux for router metrics bzbz1532060 (rchopra@redhat.com)
- update unauthorized err message - oc login (jvallejo@redhat.com)
- UPSTREAM: 58991: restore original object on apply err (jvallejo@redhat.com)
- Add DOCKER_SERVICE to master system-container (mgugino@redhat.com)
- Fix volume code to consider existing pvc (hekumar@redhat.com)
- cli: fix kube client name (mfojtik@redhat.com)
- router: use admin client to get router pod dump (mfojtik@redhat.com)
- adjust declarative pipeline sample; employ now required use of 'script'
  directive (gmontero@redhat.com)
- use upstream generated openapi (deads@redhat.com)
- update defaulting script (deads@redhat.com)
- UPSTREAM: 59279: nodelifecycle: set OutOfDisk unknown on node timeout
  (sjenning@redhat.com)
- UPSTREAM: 58720: Ensure that the runtime mounts RO volumes read-only
  (joesmith@redhat.com)
- Restart console pod when config changes (spadgett@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from d969acde90..b69b4a6c80 (jaboyd@redhat.com)
- UPSTREAM: <carry>: Short-circuit HPA oapi/v1.DC (sross@redhat.com)

* Wed Feb 07 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.41.0
- show error when cluster up fails (deads@redhat.com)
- cli: remove traces of openshift admin command (mfojtik@redhat.com)

* Wed Feb 07 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.40.0
- cli: truncate groups in get clusterrolebindings (mfojtik@redhat.com)
- make sure we can unbind a delete templateinstance (bparees@redhat.com)
- Add source secret validation to new build (nakayamakenjiro@gmail.com)
- add hyperkube to locally built images (deads@redhat.com)
- pkg/cmd/server/config has moved to pkg/cmd/server/apis/config
  (rchopra@redhat.com)
- cli: do not suggest liveness probes for controller owned pods
  (mfojtik@redhat.com)
- check all buildrequest options for validity against build type
  (bparees@redhat.com)
- return gone on unbind from non-existent templateinstance (bparees@redhat.com)
- Perform golang check in a unified way (maszulik@redhat.com)
- UPSTREAM: 56872: Fix event generation (hekumar@redhat.com)

* Tue Feb 06 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.39.0
- apps: do not inject oversized env vars into deployer pod (mfojtik@redhat.com)
- bootstrap: Remove unused ErrSocatNotFound (walters@verbum.org)
- better failure debug for fetch url tests (bparees@redhat.com)
- node selector annotation for any label. (mrsiano@gmail.com)

* Mon Feb 05 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.38.0
- expect build success in test (bparees@redhat.com)
- add suggestion to describe pod for container names (jvallejo@redhat.com)
- UPSTREAM: 58533: add suggestion to describe pod for container names
  (jvallejo@redhat.com)
- Generated files (simo@redhat.com)
- Remove empty role bindings when removing subjects (simo@redhat.com)
- Make some policy commands behave "better" (simo@redhat.com)
- Deprecate a bunch of policy commands (simo@redhat.com)
- Add infos count to `oc status` (mdame@redhat.com)
- Replace icon-cogs with fa-cogs (spadgett@redhat.com)
- generated (deads@redhat.com)
- generated (deads@redhat.com)
- demonstrate adding generated conversions for other types (deads@redhat.com)
- update conversiongen to use upstream (deads@redhat.com)
- generated (deads@redhat.com)
- update deepcopy gen scripts (deads@redhat.com)
- refactor to match normal api structure (deads@redhat.com)
- bump(*) (deads@redhat.com)
- add glide.yaml updates (deads@redhat.com)
- apps: move pkg/apps/cmd to pkg/oc (mfojtik@redhat.com)
- fix oadm panic problem (haowang@redhat.com)
- UPSTREAM: 58617: Make ExpandVolumeDevice() idempotent if existing volume
  capacity meets the requested size (hchiramm@redhat.com)
- Update README.md (ccoleman@redhat.com)
- Test case reuses a hostname from another test (ccoleman@redhat.com)
- only run one httpd instance in perl hot deploy test (bparees@redhat.com)
- fix #18291. Use correct playbook for openshift version (jcantril@redhat.com)
- tls update will be possible with 'create' permissions on custom-host. Checks
  on changing host stay the same. (rchopra@redhat.com)
- Add test case for oc explain cronjob (maszulik@redhat.com)
- UPSTREAM: 58753: Fix kubectl explain for cronjobs (maszulik@redhat.com)
- Service monitoring best practices for infrastructure (ccoleman@redhat.com)

* Fri Feb 02 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.37.0
- Use the Upstream header for scope impersonation (simo@redhat.com)
- UPSTREAM: 58994: Race condition between listener and client in
  remote_runtime_test (deads@redhat.com)
- Support --write-flags on openshift start node (ccoleman@redhat.com)
- UPSTREAM: 58930: Don't wait for certificate rotation on Kubelet start
  (ccoleman@redhat.com)
- Drop auto-egress-IP rules when egress IP is removed from NetNamespace
  (danw@redhat.com)

* Fri Feb 02 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.36.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.36.0]

* Wed Jan 31 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.35.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.35.0]

* Tue Jan 30 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.34.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.34.0]

* Tue Jan 30 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.33.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.33.0]

* Tue Jan 30 2018 Justin Pierce <jupierce@redhat.com> 3.9.0-0.32.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.32.0]

* Sat Jan 27 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.31.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.31.0]

* Fri Jan 26 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.30.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.30.0]

* Fri Jan 26 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.29.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.29.0]

* Fri Jan 26 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.28.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.28.0]

* Fri Jan 26 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.27.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.27.0]

* Fri Jan 26 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.26.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.26.0]

* Fri Jan 26 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.25.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.25.0]

* Wed Jan 24 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.24.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.24.0]

* Tue Jan 23 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.23.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.23.0]

* Fri Jan 19 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.22.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.22.0]

* Wed Jan 17 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.21.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.21.0]

* Mon Jan 15 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.20.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.20.0]

* Fri Jan 12 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.19.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.19.0]

* Fri Jan 12 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.18.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.18.0]

* Fri Jan 12 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.17.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.17.0]

* Wed Jan 03 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.16.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.16.0]

* Wed Jan 03 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.15.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.15.0]

* Wed Jan 03 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.14.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.14.0]

* Tue Jan 02 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.13.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.13.0]

* Tue Jan 02 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.12.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.12.0]

* Mon Jan 01 2018 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.11.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.11.0]

* Thu Dec 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.10.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.10.0]; bump origin-web-console f58befc

* Thu Dec 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.9.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.9.0]; bump origin-web-console 885d6a1

* Tue Dec 12 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.8.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.8.0]; bump origin-web-console 89817ca

* Tue Dec 12 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.7.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.7.0]; bump origin-web-console 6d27089

* Tue Dec 12 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.6.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.6.0]; bump origin-web-console 437c01f

* Mon Dec 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.5.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.5.0]; bump origin-web-console 21dd1cd

* Mon Dec 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.4.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.4.0]; bump origin-web-console cbd6e1b

* Mon Dec 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.3.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.3.0]; bump origin-web-console 9007fcd

* Mon Dec 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.2.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.2.0]; bump origin-web-console ef11cc9

* Fri Dec 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.9.0-0.1.0
- Automatic commit of package [atomic-openshift] release [3.9.0-0.1.0]; bump origin-web-console 6ca62a9

* Thu Nov 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.13.0
- 

* Thu Nov 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.12.0
- 

* Thu Nov 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.11.0
- 

* Thu Nov 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.10.0
- UPSTREAM: 53989: Remove repeated random string generations in scheduler
  volume predicate (jsafrane@redhat.com)
- UPSTREAM: 53135: Fixed counting of unbound PVCs towards limit of attached
  volumes (jsafrane@redhat.com)
- Version prefix matters when sorting tags names (miminar@redhat.com)
- Sort istags alphabetically during schema conversion (miminar@redhat.com)
- disable multiarch import tests (bparees@redhat.com)
- stop double building hyperkube (deads@redhat.com)
- dind: set fail-swap-on=false (dcbw@redhat.com)
- make the default system:admin client cert a system:masters (deads@redhat.com)
- prevent k8s.io/kubernetes/cmd since we didn't run them before
  (deads@redhat.com)
- minor completion changes (deads@redhat.com)
- UPSTREAM: 55974: Allow constructing spdy executor from existing transports
  (deads@redhat.com)
- update openapi generation script to exclude dir (deads@redhat.com)
- Use written node config in 'openshift start' (jliggitt@redhat.com)
- UPSTREAM: 55796: Correct ConstructVolumeSpec() (hchiramm@redhat.com)
- UPSTREAM: <carry>: disable failing etcd test for old level (deads@redhat.com)
- bump(*): glide (deads@redhat.com)
- use script to link to staging folder for patches (deads@redhat.com)
- glide.yaml (deads@redhat.com)
- Add template for local storage (jsafrane@redhat.com)
- Rename "local storage" to "HostPath storage" example (jsafrane@redhat.com)
- Add multitenant<->networkpolicy migration helper scripts (danw@redhat.com)

* Wed Nov 22 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.9.0
- Enable cert rotation for node bootstrap (jliggitt@redhat.com)
- update dind image (deads@redhat.com)
- Make "openshift start node --write-config" tolerate swap on
  (jliggitt@redhat.com)
- Revert "set fail-swap-on to false for cluster up" (jliggitt@redhat.com)
- generated (deads@redhat.com)
- remove openshift cli and friends (deads@redhat.com)
- pkg/security/OWNERS: add simo5 to the list of approvers.
  (vsemushi@redhat.com)
- UPSTREAM: 56045: Fix getting logs from daemonset (mfojtik@redhat.com)
- generated (deads@redhat.com)
- switch to hyperkube (deads@redhat.com)
- Remove soltysh from OWNERS (maszulik@redhat.com)
- Normalize image ref before pulling in cluster up (jliggitt@redhat.com)
- update generated docs (jvallejo@redhat.com)
- remove pkg/cmd/server/admin/overwrite_bootstrappolicy.go and
  pkg/cmd/server/admin/legacyetcd (jvallejo@redhat.com)
- generated (deads@redhat.com)
- Remove dockerregistry Dockerfile and build (jliggitt@redhat.com)
- set fail-swap-on to false for cluster up (mfojtik@redhat.com)
- Revert "interesting: restore ability to start with swap on by default"
  (jliggitt@redhat.com)
- try to modify the build scripts and not turn purple (deads@redhat.com)
- remove kubectl from openshift (but not oc) (deads@redhat.com)
- admission_test.go(TestAdmitSuccess): remove hardcoded SELinux level.
  (vsemushi@redhat.com)
- admission_test.go(testSCCAdmission): modify to signalize about errors.
  (vsemushi@redhat.com)
- admission_test.go(TestAdmitSuccess): compare SecurityContexts instead of
  particular members. (vsemushi@redhat.com)
- admission_test.go(saSCC): extract function. (vsemushi@redhat.com)
- switch to external user client (deads@redhat.com)
- admission_test.go(saExactSCC): extract function. (vsemushi@redhat.com)
- admission_test.go(createSCCListerAndIndexer): introduce and use function.
  (vsemushi@redhat.com)
- admission_test.go: rename variable to better describe its type.
  (vsemushi@redhat.com)
- admission_test.go(createSCCLister): extract function. (vsemushi@redhat.com)
- admission_test.go(setupClientSet): extract function. (vsemushi@redhat.com)
- admission_test.go(TestAdmitFailure): reduce code by (enchancing and) using
  existing function. (vsemushi@redhat.com)
- admission_test.go(TestAdmit): split to TestAdmitSuccess and TestAdmitFailure.
  (vsemushi@redhat.com)
- admission_test.go(TestAdmit): eliminate duplicated code by using existing
  method. (vsemushi@redhat.com)
- admission_test.go(testSCCAdmission): print test case name when test fails.
  (vsemushi@redhat.com)
- switch easy admission plugins to external clients (deads@redhat.com)
- Force removal of temporary build containers (cewong@redhat.com)

* Mon Nov 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.8.0
- 

* Mon Nov 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.7.0
- fixme: use openshift/origin-docker-registry:latest as registry image
  (mfojtik@redhat.com)
- Generated files (jliggitt@redhat.com)
- interesting: restore ability to start with swap on by default
  (jliggitt@redhat.com)
- interesting: buildconfig/instantiate spdy executor change
  (maszulik@redhat.com)
- interesting: remove double printer handler registration (mfojtik@redhat.com)
- interesting: Master and node changes (maszulik@redhat.com)
- interesting: API server changes (maszulik@redhat.com)
- interesting: fixup template data types (jliggitt@redhat.com)
- interesting: generate canonical temporary build tag (jliggitt@redhat.com)
- interesting: remove bad tests from test/cmd/certs.sh (mfojtik@redhat.com)
- interesting: bump reconcile qps (jliggitt@redhat.com)
- interesting: run reflectors in a goroutine (maszulik@redhat.com)
- interesting: make 'oc registry' generate valid DaemonSet
  (jliggitt@redhat.com)
- interesting: ApproximatePodTemplateForObject (jliggitt@redhat.com)
- interesting: etcd_storage_path test updates (maszulik@redhat.com)
- interesting: add cohabitating resources for daemonsets/replicasets, bump
  cronjob storage (jliggitt@redhat.com)
- interesting: ignore authorization.openshift.io role/binding objects for GC
  (jliggitt@redhat.com)
- interesting: default the triggers in dc integration test
  (jliggitt@redhat.com)
- interesting: replace ParseNormalizedNamed with reference.WithName
  (maszulik@redhat.com)
- interesting: switch dockerutils to deal with two auth config types
  (jliggitt@redhat.com)
- interesting: drop scheduled jobs support (maszulik@redhat.com)
- interesting: oc: fix forwarder (mfojtik@redhat.com)
- interesting: HPA v2alpha1 removed (jliggitt@redhat.com)
- interesting: NetworkPolicy moved from extensions to networking
  (maszulik@redhat.com)
- interesting: errors are structured now, fixup TestFrontProxy test
  (jliggitt@redhat.com)
- boring: avoid spurious diff of nil/[] in extended test (jliggitt@redhat.com)
- boring: update integration tests to check hpa permissions in autoscaling
  group (jliggitt@redhat.com)
- boring: fix error messages in authorization test (mfojtik@redhat.com)
- boring: update TestRootRedirect paths, re-enable openapi in integration,
  update preferred RBAC version (jliggitt@redhat.com)
- boring: Update registered aggregated APIs with new versions
  (jliggitt@redhat.com)
- boring: govet fixes (maszulik@redhat.com)
- boring: define localSchemeBuilder, ensure LegacySchemeBuilder has
  RegisterDeepCopies/RegisterDefaults/RegisterConversions (jliggitt@redhat.com)
- boring: test-cmd fixes (maszulik@redhat.com)
- boring: regenerate policy (maszulik@redhat.com)
- boring: exclude new alpha kubectl command (maszulik@redhat.com)
- boring: quota conversions error return (maszulik@redhat.com)
- boring: Admission interface changes (maszulik@redhat.com)
- boring: update clientgen type annotations (maszulik@redhat.com)
- boring: docker API changes (jliggitt@redhat.com)
- boring: deepcopy calls (jliggitt@redhat.com)
- boring (maszulik@redhat.com)
- boring: k8s.io/api import restrictions changes (jliggitt@redhat.com)
- hack/update-generated-clientsets.sh: remove shortGroup and version from
  generator (mfojtik@redhat.com)
- hack/update-generated-protobuf.sh: reuse upstream proto generator
  (maszulik@redhat.com)
- genconversion: add k8s.io/api/core/v1 (jliggitt@redhat.com)
- gendeepcopy: omit k8s packages (maszulik@redhat.com)
- UPSTREAM: k8s.io/gengo: 73: handle aliases (maszulik@redhat.com)
- UPSTREAM: <drop>: disable flaky InitFederation unit test (deads@redhat.com)
- UPSTREAM: <drop>: etcd testing (maszulik@redhat.com)
- UPSTREAM: <carry>: allow controller context injection to share informers
  (jliggitt@redhat.com)
- UPSTREAM: <carry>: allow multiple containers to union for swagger
  (maszulik@redhat.com)
- UPSTREAM: <carry>: switch back to use encode/json to avoid serialization
  errors (mfojtik@redhat.com)
- UPSTREAM: 55974: Allow constructing spdy executor from existing transports
  (jliggitt@redhat.com)
- UPSTREAM: 55772: Only attempt to construct GC informers for watchable
  resources (jliggitt@redhat.com)
- UPSTREAM: 53576: Revert "Validate if service has duplicate targetPort"
  (jliggitt@redhat.com)
- UPSTREAM: 55703: use full gopath for externalTypes (maszulik@redhat.com)
- UPSTREAM: 55704: Return original error instead of negotiation one
  (maszulik@redhat.com)
- UPSTREAM: 55248: increase iptables max wait from 2 seconds to 5 (fix)
  (bbennett@redhat.com)
- UPSTREAM: google/cadvisor: 1785: fix long du duration message
  (sjenning@redhat.com)
- UPSTREAM: google/cadvisor: 1766: adaptive longOp for du operation
  (sjenning@redhat.com)
- UPSTREAM: docker/distribution: 2384: Fallback to GET for manifest
  (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: 2402: Allow manifest specification
  (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: 2382: Don't double add scopes
  (ccoleman@redhat.com)
- UPSTREAM: containers/image: <carry>: Disable gpgme on windows/mac
  (ccoleman@redhat.com)
- UPSTREAM: <drop>: run hack/copy-kube-artifacts.sh (jliggitt@redhat.com)
- bump(k8s.io/kubernetes): 0d5291cc63b7b3655b11bc15e8afb9a078049d09 - v1.8.1
  (jliggitt@redhat.com)
- bump(github.com/Sirupsen/logrus) - drop in favor of
  github.com/sirupsen/logrus (maszulik@redhat.com)
- bump(github.com/containers/image): 8df46f076f47521cef216839d04202e89471da2e
  (maszulik@redhat.com)
- bump(github.com/google/cadvisor): cda62a43857256fbc95dd31e7c810888f00f8ec7 -
  this needs to be revisit, if all the patches we have are in place
  (maszulik@redhat.com)
- bump(github.com/docker/docker): 4f3616fb1c112e206b88cb7a9922bf49067a7756 -
  this is a potential source of problems, be careful (maszulik@redhat.com)
- bump(github.com/docker/distribution):
  edc3ab29cdff8694dd6feb85cfeb4b5f1b38ed9c (maszulik@redhat.com)
- bump(github.com/aws/aws-sdk-go): 63ce630574a5ec05ecd8e8de5cea16332a5a684d
  (maszulik@redhat.com)
- bump(github.com/openshift/imagebuilder):
  b6a142b9d7f3d57a2bf86ae9c9342f98e0c97c5b (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image):
  e3140d019517368c7c3f72476f9cae7a8b1269d0 (maszulik@redhat.com)
- Update copy-kube-artifacts (maszulik@redhat.com)
- Update godep-save.sh to current state (maszulik@redhat.com)
- shim pkg/build/builder source-to-image types (jliggitt@redhat.com)
- lock pkg/build/builder to github.com/openshift/source-to-image
  e3140d019517368c7c3f72476f9cae7a8b1269d0 (jliggitt@redhat.com)
- lock pkg/build/builder to github.com/docker/distribution
  1e2bbed6e09c6c8047f52af965a6e301f346d04e (jliggitt@redhat.com)
- lock pkg/build/builder to github.com/docker/engine-api
  dea108d3aa0c67d7162a3fd8aa65f38a430019fd (mfojtik@redhat.com)
- lock pkg/build/builder to github.com/Azure/go-ansiterm
  7e0a0b69f76673d5d2f451ee59d9d02cfa006527 (jliggitt@redhat.com)
- lock pkg/build/builder to github.com/Sirupsen/logrus
  aaf92c95712104318fc35409745f1533aa5ff327 (maszulik@redhat.com)
- interesting: remove registry as it now lives in external repo
  (jliggitt@redhat.com)

* Mon Nov 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.6.0
- Adding OS_GIT_PATCH env var for build (jupierce@redhat.com)
- poll for log output in extended tests (bparees@redhat.com)
- Allow '-n none' for the dind to set up no network plugin
  (bbennett@redhat.com)

* Sun Nov 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.5.0
- 

* Sun Nov 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.4.0
- Removing gitserver from build images (jupierce@redhat.com)
- Adding new commit vars to RPM spec build (jupierce@redhat.com)
- Fixed a typo in OWNERS (rajatchopra and knobunc were wrong)
  (bbennett@redhat.com)
- break dependency on version cmd for non-cli pkgs (jvallejo@redhat.com)
- update images being tested (bparees@redhat.com)
- Allow assign-macvlan annotation to specify an interface (danw@redhat.com)
- add skip_pv marker to skip PV creation (m.judeikis@gmail.com)
- Bug 1509799 - Fix WaitAndGetVNID() in sdn node (rpenta@redhat.com)
- Fix project sync interval in router (rpenta@redhat.com)
- Router: Changed default resource resync interval from 10mins to 30mins
  (rpenta@redhat.com)

* Fri Nov 17 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.3.0
- 

* Thu Nov 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.2.0
- Revert "Imagestream tag exclude from pruning" (mfojtik@mfojtik.io)
- dockergc: storage driver support limited to overlay2 (sjenning@redhat.com)
- Avoid parsing the whole dump-flows output in the OVS health check
  (danw@redhat.com)
- Add new option to exclude imagestream tag from pruning by regular expression
  (agladkov@redhat.com)
- Add python 3.6 S2I image (alexandre.lossent@cern.ch)
- Print the namespace in the endpoints change log message (bbennett@redhat.com)

* Wed Nov 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.8.0-0.1.0
- Adding 3.8 releaser (jupierce@redhat.com)
- tsb: use external clients (jminter@redhat.com)
- generated (deads@redhat.com)
- Remove pre CNI docker cleanup code from openshift SDN (rpenta@redhat.com)
- make assetconfig a top level type (deads@redhat.com)
- always install the jenkins sample template (bparees@redhat.com)
- Delegated auth for router metrics allows anonymous (ccoleman@redhat.com)
- add warning to Zookeeper example (jminter@redhat.com)
- remove cors from webconsole (deads@redhat.com)
- Fix push-release (ccoleman@redhat.com)
- UPSTREAM: 50390: Admit sysctls for other runtime. (runcom@redhat.com)
- fix up template instance controller permissions (bparees@redhat.com)
- Improve the `oc auth` subcommands CLI example usage: Replaced the `kubectl`
  to `oc` (teleyic@gmail.com)
- retry build watch on error/expiration (bparees@redhat.com)
- Old routers may not have permission to do SAR checks for metrics
  (ccoleman@redhat.com)
- fix off by one error in oc adm top images edge creation (bparees@redhat.com)
- add toleration for slow logs in scl tests (bparees@redhat.com)
- check RepoTags len before accessing (sjenning@redhat.com)
- Bumping origin.spec for 3.8 (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  721cde05fe8c386935adc209638700b2476dd228 (eparis+openshiftbot@redhat.com)
- TestPrintRoleBindingRestriction output check test enhance
  (shiywang@redhat.com)
- don't create output imagestrem if already exists with newapp; better circular
  tag detection (gmontero@redhat.com)
- Allow registry-admin to manage RBAC roles/bindings (mkhan@redhat.com)
- bump(github.com/openshift/origin-web-console):
  76b140d4ccf2f2025c25c2ad0ec9ed89a521d063 (eparis+openshiftbot@redhat.com)
- Bump origin spec files to be consistent with shared code
  (ccoleman@redhat.com)
- Handle OPTIONS as additional argments (nakayamakenjiro@gmail.com)
- include namespace in --list-pods for node output (jvallejo@redhat.com)
- Remove double quotations from docker env to run node
  (nakayamakenjiro@gmail.com)
- Cleaning up port/protocol split/validation code (cdaley@redhat.com)
- tighten secondary build vendors (deads@redhat.com)
- allow rsrs/kind format in oc rsync (jvallejo@redhat.com)
- prevent err message when removing env vars (jvallejo@redhat.com)
- Adding describer for TemplateInstances (cdaley@redhat.com)
- move error cause to top of err message (jvallejo@redhat.com)
- UPSTREAM: 49885: Ignore UDP metrics in kubelet (runcom@redhat.com)
- remove bad builder dependencies (deads@redhat.com)
- Fix dns lookup in PodCheckDns diagnostic (rpenta@redhat.com)
- Since the 'oc deploy' is deprecated. It is better for providing usage 'oc set
  trigger'. Forgetting the 'oc deploy'. (teleyic@gmail.com)
- Fixed the wrong name of building image. According to the implementation and
  running behavior. the building image is openshift/origin-release
  (teleyic@gmail.com)
- Fixed the typo of the image name. According to the definition in constants.sh
  and running behavior, default image name is 1.8. (teleyic@gmail.com)
- Fixed the type definition (teleyic@gmail.com)
- docs(HACKING.md): fix formatting of heading (surajd.service@gmail.com)
- Change image for complete-dc-hooks.yaml to centos7 which will be already pre-
  pulled (tnozicka@gmail.com)
- allow template labels to be parameterised (jminter@redhat.com)

* Thu Nov 09 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.5-1
- clean up use of persistent in template display names (ux review)
  (gmontero@redhat.com)
- add app label to quickstart, jenkins templates (gmontero@redhat.com)

* Wed Nov 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.4-1
- 

* Wed Nov 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.3-1
- sdn: add metric for "failed to find netid for namespace: XXXX in vnid map"
  errors (dcbw@redhat.com)
- unit: increate timeout for getting master version (miminar@redhat.com)
- Add missing catalog service account roles (erik@nsk.io)
- Added an OWNERS file with networking team members for the router
  (fiji@limey.net)

* Wed Nov 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.2-1
- Bumping version to avoid conflict with tito tag (jupierce@redhat.com)

* Wed Nov 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-1
- Change release field from pre-release (jupierce@redhat.com)
- Fix deployment trigger controller crash (ironcladlou@gmail.com)

* Wed Nov 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.198.0
- bump(github.com/openshift/origin-web-console):
  eb4047d1b20a27e162b2abc1ae7d9b225ac69fd9 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  766b9b8347efd4b5c4f7954505bb3d08332ab792 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 55248: increase iptables max wait from 2 seconds to 5 (fix)
  (bbennett@redhat.com)
- fixup extended tests to use bindable template (jminter@redhat.com)
- remove expose annotations from quickstarts and add bindable: false
  (jminter@redhat.com)
- add template.openshift.io/bindable annotation, default is true
  (jminter@redhat.com)
- add support for JenkinsPipeline strategy env update (jvallejo@redhat.com)
- verify that imagechangetriggers trigger all build types (jminter@redhat.com)
- extended: reenabled image signature workflow test (miminar@redhat.com)
- verify-signature: fixed insecure fall-back (miminar@redhat.com)
- add event when build image trigger fails (jminter@redhat.com)
- allow image trigger controller to create custom builds (jminter@redhat.com)

* Tue Nov 07 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.197.0
- introduce a configurable delay between creating resources to prevent spurious
  GC (bparees@redhat.com)
- limit retries of imagesignature import and don't log failures
  (bparees@redhat.com)
- Prevent unbounded growth in SCC due to duplicates (ccoleman@redhat.com)
- SCC can't be patched via JSONPatch because users is nil (ccoleman@redhat.com)
- catalog: add cluster service broker admin role (jpeeler@redhat.com)

* Mon Nov 06 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.196.0
- Let the kubelet initialize its own clients (ccoleman@redhat.com)

* Mon Nov 06 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.195.0
- 

* Sun Nov 05 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.194.0
- 

* Sun Nov 05 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.193.0
- UPSTREAM: 55028: kubelet: dockershim: remove orphaned checkpoint files
  (sjenning@redhat.com)

* Sat Nov 04 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.192.0
- preserve error type in loginoptions (jvallejo@redhat.com)
- Network component should refresh certificates if they expire
  (ccoleman@redhat.com)
- clear pod ownerRefs before creating debug pod (jvallejo@redhat.com)
- UPSTREAM: 54979: Certificate store handles rel path incorrectly
  (ccoleman@redhat.com)
- UPSTREAM: 54921: rename metric reflector_xx_last_resource_version
  (ironcladlou@gmail.com)
- yaml-enable the registry rate limiting config (bparees@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 892b0368f0..3064247d05 (jaboyd@redhat.com)
- Skip building release archives during RPM build (ccoleman@redhat.com)
- Remove the need for release tars in normal code paths (ccoleman@redhat.com)
- Fix the "supress health checks when only one backing service" logic
  (bbennett@redhat.com)
- Revert "Skip health checks when there is one server that backs the route"
  (bbennett@redhat.com)
- Avoid compiling all of kube twice for images (ccoleman@redhat.com)
- merge imagestreamtag list on patch (bparees@redhat.com)

* Fri Nov 03 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.191.0
- Add timestamps to migration command's reporting (mkhan@redhat.com)
- Correctly handle NotFound errors during migration (mkhan@redhat.com)
- UPSTREAM: 54828: trigger endpoint update on pod deletion
  (joesmith@redhat.com)
- Remove API initialization from dockerregistry (obulatov@redhat.com)

* Thu Nov 02 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.190.0
- fixing added whitespace (smunilla@redhat.com)
- catalog: add RBAC rules for serviceinstances (jpeeler@redhat.com)
- Partially revert node IP startup check (danw@redhat.com)
- UPSTREAM: 54812: Allow override of cluster level (default, whitelist)
  tolerations by namespace level empty (default, whitelist) tolerations.
  (avagarwa@redhat.com)
- Bug 1508061: Fix panic when accessing controller args (mfojtik@redhat.com)
- Fix crash with invalid serviceNetworkCIDR (danw@redhat.com)
- Fix auto-egress-IP / EgressNetworkPolicy interaction (danw@redhat.com)
- Fix up destination MAC of auto-egress-ip packets (danw@redhat.com)
- UPSTREAM: drop: fix for bz1507257 hacked from upstream PR47850, drop these
  changes in favour of that PR because this one does not carry the entire
  dependent chain. Conflicts were removed manually. (dcbw@redhat.com)
- UPSTREAM: google/cadvisor: 1785: fix long du duration message
  (sjenning@redhat.com)
- origin.spec: Master configs now under /etc/origin/master/
  (smilner@redhat.com)

* Wed Nov 01 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.189.0
- Log OVS errors at a better level (danw@redhat.com)
- Allow EXPOSE <number>/<protocol> in Dockerfile (cdaley@redhat.com)
- Router - reduce log output (pcameron@redhat.com)
- unit-tests: fixed flake and race in image pruning (miminar@redhat.com)
- Skip health checks when there is one server that backs the route
  (bbennett@redhat.com)
- Fixed a TODO comment by reviewing the code (bbennett@redhat.com)
- Fix incorrect comment in template (ichavero@redhat.com)
- UPSTREAM: 54763: make iptables wait flag generic; increase the max wait time
  from 2 seconds to 5 seconds (rchopra@redhat.com)
- image-pruning: derefence imagestreamtags (miminar@redhat.com)
- DeploymentConfig replicas should be optional, other fields too
  (ccoleman@redhat.com)
- Revert "Made the router skip health checks when there is one endpoint"
  (bbennett@redhat.com)
- fixes #16902.  Add etcd section to inventory for 'oc cluster up'
  (jcantril@redhat.com)
- Fix duplicate timeout directive (ichavero@redhat.com)
- image-pruning: adding replica sets to the graph (miminar@redhat.com)
- image-pruning: add upstream deployments to the graph (miminar@redhat.com)
- image-pruning: add daemonsets to the graph (miminar@redhat.com)
- image-pruning: delete istag references to absent images (miminar@redhat.com)

* Tue Oct 31 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.188.0
- Use import-verifier to keep us from breaking build-cross in pkg/network
  (danw@redhat.com)
- return error on long-form or invalid sa name (jvallejo@redhat.com)
- Fix cross build (danw@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 510060232e..892b0368f0 (jpeeler@redhat.com)
- include err message suggestion oc login err output (jvallejo@redhat.com)
- Set X-DNS-Prefetch-Control header on console assets (jforrest@redhat.com)
- test: extended: avoid using docker directly (runcom@redhat.com)

* Mon Oct 30 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.187.0
- use new s2i base image (bparees@redhat.com)
- use the output of ParseNetworkInfo to create the cluster network object
  instead of the raw input from the master config file (jtanenba@redhat.com)
- prevents a segfault caused by not setting the cidr in master.networkInfo when
  the address is not in cannonical form (jtanenba@redhat.com)
- Reduce node iptables logging in V(2) (pcameron@redhat.com)
- UPSTREAM: 48813: maxinflight handler should let panicrecovery handler call
  NewLogged (shyamjvs@google.com)

* Sun Oct 29 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.186.0
- retry scc update in extended test on conflict (jminter@redhat.com)
- Bug 1505266 - Validate node IP is local during sdn node initialization
  (rpenta@redhat.com)
- Refactor: Move sdn node Hostname/SelfIP initialization to setNodeIP()
  (rpenta@redhat.com)
- bump(github.com/openshift/source-to-image):
  aaa1d47a1eccb19859d6a64e64def7da734d95ef (jminter@redhat.com)
- Update console OPENSHIFT_CONSTANTS flags for cluster up (spadgett@redhat.com)
- Change the router reload suppression so that it doesn't block updates
  (bbennett@redhat.com)
- Fixed the wrong link, which will cause 404 response (teleyic@gmail.com)
- UPSTREAM: 54597: kubelet: check for illegal container state transition
  (amcdermo@redhat.com)
- Made the router test take Env variables to alter logging
  (bbennett@redhat.com)

* Sat Oct 28 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.185.0
- Fix Hybrid Proxy Logging Verbosity (sross@redhat.com)

* Fri Oct 27 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.184.0
- bump(github.com/openshift/origin-web-console):
  14188ec5bc5e40836631f5d194e9a6d5c86162f8 (eparis+openshiftbot@redhat.com)
- Bug - propagate docker build image failure (jwozniak@redhat.com)
- Handle Egress IP already being present when trying to add it
  (danw@redhat.com)
- strip template prefix from TSB annotations (bparees@redhat.com)

* Fri Oct 27 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.183.0
- 

* Thu Oct 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.182.0
- UPSTREAM: 54593: Removed containers are not always waiting
  (joesmith@redhat.com)
- add Limit & Limit/Request columns (jvallejo@redhat.com)

* Thu Oct 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.181.0
- 

* Thu Oct 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.180.0
- bump(github.com/openshift/origin-web-console):
  68b29c4b8cf3c1a78893f9b4ae04875099dcdb26 (eparis+openshiftbot@redhat.com)
- fix cluster up extended test (bparees@redhat.com)
- update .dockercfg content to config.json (jvallejo@redhat.com)
- UPSTREAM: 53916: update .dockercfg data to config.json format
  (jvallejo@redhat.com)
- convert unstructured objs before exporting (jvallejo@redhat.com)
- UPSTREAM: 53464: output empty creationTimestamps as null
  (jvallejo@redhat.com)
- use unstructured builder - oc export (jvallejo@redhat.com)

* Thu Oct 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.179.0
- Add some basic headers to OSIN provided pages (simo@redhat.com)
- Refactor hard prune (agladkov@redhat.com)
- apps: deployment config stuck in the new state should respect timeoutSecods
  (mfojtik@redhat.com)
- Fix the check for "pod has HostPorts" (danw@redhat.com)
- Remove dead code (simo@redhat.com)
- Verify layer sizes in the integrated registry (obulatov@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6960e4f7d85d9aa6506a28091b5197a7f32b548d (eparis+openshiftbot@redhat.com)
- catalog: handle change to single catalog binary (jpeeler@redhat.com)
- rpm: Remove 1.13 from excluder (smilner@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from aa27078754..510060232e (jpeeler@redhat.com)
- validate user-specified resource, if given (jvallejo@redhat.com)
- UPSTREAM: 51750: output `<none>` for colums not found (jvallejo@redhat.com)
- parse resource name before removing deleted secret (jvallejo@redhat.com)
- UPSTREAM: 53606: implement ApproximatePodTemplateObject upstream
  (jvallejo@redhat.com)
- use deployment pod template if 0 replicas (jvallejo@redhat.com)
- UPSTREAM: 53720: Optimize random string generator to avoid multiple locks.
  This is a modified version of the upstream 53720, as SafeEncodeString
  function does not exist in 3.7. (avagarwa@redhat.com)
- UPSTREAM: 53793: User separate client for leader election in scheduler 1.7 PR
  is https://github.com/kubernetes/kubernetes/pull/53884 (avagarwa@redhat.com)
- UPSTREAM: 53989: Remove repeated random string generations in scheduler
  volume predicate (avagarwa@redhat.com)

* Wed Oct 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.178.0
- prevent client from looking up specific node (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0f2d9ba88a90961267471ed0f4b83e3397ae55a3 (eparis+openshiftbot@redhat.com)
- Fix "Bad parameters: you must choose at least one stream" error when showing
  the logs. (vsemushi@redhat.com)
- Router metrics should be protected by delegated auth (ccoleman@redhat.com)
- Specify both root CA and service account ca for prometheus
  (ccoleman@redhat.com)

* Tue Oct 24 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.177.0
- wait for group cache in templateinstance tests (jminter@redhat.com)
- hack/lib/cmd.sh - replacing "minute" with "$minute" in "local
  duration=${#:-minute}". (nhosoi@redhat.com)
- image: fix signature import from secure registries (mfojtik@redhat.com)
- fix flake in run_policy failed build handling test (bparees@redhat.com)
- Fix initialization of iptables under networkpolicy plugin (danw@redhat.com)
- Add debug to build authentication (cewong@redhat.com)
- consistent [registry] and [Feature:Image*] tags on image/registry tests
  (bparees@redhat.com)
- Remove abstractj from OWNERS (mkhan@redhat.com)
- delete templateinstances in foreground where necessary in extended tests
  (jminter@redhat.com)
- Allow passing a flag to skip a binary build when creating rpms
  (cewong@redhat.com)
- Split integration test output into two parts (ccoleman@redhat.com)
- Correctly parse nested go tests (ccoleman@redhat.com)
- stop calling junit merge. (bparees@redhat.com)

* Mon Oct 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.176.0
- address refactored mysql replica scripts (bparees@redhat.com)

* Mon Oct 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.175.0
- Fixed the doc usage errors (teleyic@gmail.com)
- fix doc links, avoid 404 (teleyic@gmail.com)
- Generate origin listers (mkhan@redhat.com)
- UPSTREAM: 54257: Use GetByKey() in typeLister_NonNamespacedGet
  (cheimes@redhat.com)
- adjust prometheus ext test existence check to query StatefulSets
  (gmontero@redhat.com)

* Sun Oct 22 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.174.0
- Mute a warning in NetworkPolicy handling (danw@redhat.com)

* Sun Oct 22 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.173.0
- bump(github.com/openshift/origin-web-console):
  b2c1cf101ec02d28aa4eba9d6a92642cedd0564d (eparis+openshiftbot@redhat.com)
- more pipeline bld ext tst dbg improvements (gmontero@redhat.com)
- Try to collect some debug info if e2e service test fails (danw@redhat.com)
- Add provider annotations to image-streams (sspeiche@redhat.com)

* Sun Oct 22 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.172.0
- UPSTREAM: 45611: remove use of printf semantics for view-last-applied cmd
  (jvallejo@redhat.com)
- fix flake: Router test "address already in use" (jminter@redhat.com)
- fix examples (bparees@redhat.com)
- Fix base iptables rule ordering to fix the OPENSHIFT-ADMIN-OUTPUT-RULES table
  (danw@redhat.com)

* Sat Oct 21 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.171.0
- UPSTREAM: 54308: The garbage collector creates too many pods
  (ccoleman@redhat.com)
- extended: annotated registry tests (miminar@redhat.com)
- Node service could be started when net.ipv4.ip_forward=0
  (pcameron@redhat.com)
- UPSTREAM: google/cadvisor: 1770: Monitor diff directory for overlay2
  (sjenning@redhat.com)
- UPSTREAM: 52503: Get fallback termination msg from docker when using journald
  log driver (joesmith@redhat.com)

* Fri Oct 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.170.0
- 

* Fri Oct 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.169.0
- don't lowercase metric labels (bparees@redhat.com)
- UPSTREAM: 49016: PV controller: resync informers manually
  (jsafrane@redhat.com)
- strip template prefix from TSB annotations (bparees@redhat.com)
- Add integration test for the request token endpoints (mrogers@redhat.com)
- cmd: ex: standalone docker garbage collector (sjenning@redhat.com)
- enhance template fuzz testing (jminter@redhat.com)
- clarify the all images help text (bparees@redhat.com)
- catalog: edit view role to have required rbac rules (jpeeler@redhat.com)
- warn on missing service cert signer in oadm diagnostics (deads@redhat.com)

* Fri Oct 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.168.0
- bump(github.com/openshift/origin-web-console):
  f127c701898b224faa396d394df9b498d62e9d37 (eparis+openshiftbot@redhat.com)
- Temporarily disable checking for multiple deployer pods until we have a fix
  for the controllers (tnozicka@gmail.com)
- add details to scl test steps (bparees@redhat.com)
- check for pending, not running state for next build (bparees@redhat.com)
- wait for group cache to avoid flake in templateinstance test
  (jminter@redhat.com)
- Router - A/B weights distribution improvement (pcameron@redhat.com)
- add missing rbac rule for builder (jminter@redhat.com)
- catalog: add versioning for release build (jpeeler@redhat.com)
- UPSTREAM: 53167: Do not GC exited containers in running pods
  (sjenning@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 3aacfedec6..aa27078754 (jaboyd@redhat.com)
- Verify that EgressIPs are on the expected subnet (danw@redhat.com)
- Do Egress IP link initialization stuff from Start() (danw@redhat.com)
- Tweak OVS flows for egress IPs (danw@redhat.com)

* Thu Oct 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.167.0
- 

* Thu Oct 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.166.0
- 

* Thu Oct 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.165.0
- 

* Thu Oct 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.164.0
- 

* Thu Oct 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.163.0
- don't run conformance tests w/ serial unless they are part of the focus
  (bparees@redhat.com)

* Thu Oct 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.162.0
- 

* Wed Oct 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.161.0
- 

* Wed Oct 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.160.0
- bump(github.com/openshift/origin-web-console):
  a7bc1f069a129945d6904c9efe0fbc86c581887f (eparis+openshiftbot@redhat.com)
- UPSTREAM: 53233: Remove containers from deleted pods once containers have
  exited (joesmith@redhat.com)
- Parse $JUNIT_REPORT=true in the Bash init script (skuznets@redhat.com)
- Conformance test against 1.7 again now that tests passed
  (ccoleman@redhat.com)
- Router - hsts for "edge" or "reencrypt" only (pcameron@redhat.com)

* Wed Oct 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.159.0
- update completions (jvallejo@redhat.com)
- UPSTREAM: 52440: add --dry-run option -> oadm <drain,cordon,uncordon>
  (jvallejo@redhat.com)
- add bitbucket v5.4 support (bparees@redhat.com)
- update generated completions (jvallejo@redhat.com)
- UPSTREAM: 48033: Refactor and simplify generic printer for unknown objects
  (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  77ce2ce4b376b00203d3cfc22263dce6b5fc8344 (eparis+openshiftbot@redhat.com)
- Fix defaulting of legacy ClusterNetwork fields (danw@redhat.com)
- Fix an error message (danw@redhat.com)
- UPSTREAM: 50583: Make endpoints controller update based on semantic equality
  (jliggitt@redhat.com)
- UPSTREAM: <drop>: drop in 1.9 rebase.  Shims enough admission webhook to work
  without modifying api (deads@redhat.com)
- UPSTREAM: 53896: decode admission responses into a fresh object
  (deads@redhat.com)
- UPSTREAM: 50476:  fix the webhook unit test; the server cert needs to have a
  valid CN; fix a fuzzer (deads@redhat.com)
- UPSTREAM: 53823: allow fail close webhook admission (deads@redhat.com)
- turn admission webhooks on in cluster up (deads@redhat.com)
- UPSTREAM: 52673: default service resolver for webhook admission
  (deads@redhat.com)
- continue on nil configuration (jvallejo@redhat.com)
- move build testdata files into builds subdirs (bparees@redhat.com)
- UPSTREAM: 53857: kubelet sync pod throws more detailed events
  (joesmith@redhat.com)
- UPSTREAM: 50350: Wait for container cleanup before deletion
  (joesmith@redhat.com)
- UPSTREAM: 48970: Recreate pod sandbox when the sandbox does not have an IP
  address. (joesmith@redhat.com)
- UPSTREAM: 48589: When faild create pod sandbox record event.
  (joesmith@redhat.com)
- UPSTREAM: 48584: Move event type (joesmith@redhat.com)
- UPSTREAM: 47599: Rerun init containers when the pod needs to be restarted
  (joesmith@redhat.com)
- Use glog local name instead of log (danw@redhat.com)

* Tue Oct 17 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.158.0
- remove unnecessary anonymous function (deads@redhat.com)
- UPSTREAM: 53831: Fix volume reconciler test flake (hekumar@redhat.com)
- UPSTREAM: 48757: Fix flaky test in reconciler_test (hekumar@redhat.com)
- Sharded router based on namespace labels should notice routes immediately
  (rpenta@redhat.com)
- Distinguish SCCs that AllowHostNetwork and AllowHostPorts from those that do
  not, in the score calculation. (jpazdziora@redhat.com)

* Mon Oct 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.157.0
- 

* Mon Oct 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.156.0
- replace cmdutil.ErrExit with kcmdutil.ErrExit (bparees@redhat.com)
- Enable asynchronous deprovision in TSB (jminter@redhat.com)
- extended: log registry pod to artifacts directory (miminar@redhat.com)
- Fixed the wrong link of source (teleyic@gmail.com)
- DiagnosticPod: handle interrupt (lmeyer@redhat.com)
- NetworkCheck: warn -> info on wrong network plugin (lmeyer@redhat.com)
- NetworkCheck: handle interrupt (lmeyer@redhat.com)
- clusterresourceoverride: exempt known infra projects from enforcement
  (sjenning@redhat.com)
- UPSTREAM: 53753: Reduce log spam in qos container manager
  (joesmith@redhat.com)
- Require network API objects to have IPv4 addresses (danw@redhat.com)

* Sun Oct 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.155.0
- bump(github.com/openshift/origin-web-console):
  8b7c0d947baf3950503e83863d08878c553de779 (eparis+openshiftbot@redhat.com)
- cli: Mirror images across registries or to S3 (ccoleman@redhat.com)
- bump(github.com/aws/aws-sdk-go/service/s3): add two packages
  (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: 2384: Fallback to GET for manifest
  (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: 2402: Allow manifest specification
  (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: 2382: Don't double add scopes
  (ccoleman@redhat.com)

* Sat Oct 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.154.0
- 

* Fri Oct 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.153.0
- Update the generated completion to remove deprected commands
  (simo@redhat.com)
- Warn about deprecated commands (simo@redhat.com)

* Fri Oct 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.152.0
- 

* Fri Oct 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.151.0
- bump(github.com/openshift/origin-web-console):
  e679211bf020fbb4896d1b23c807ee8514c586d3 (eparis+openshiftbot@redhat.com)
- Ensure openshift start network can run in a pod (ccoleman@redhat.com)
- Add a prototypical network-daemonset (ccoleman@redhat.com)
- Auto-create openshift-node and given nodes read on node-config
  (ccoleman@redhat.com)
- Add --bootstrap-config-name to kubelet (ccoleman@redhat.com)
- Set a default certificate duration for bootstrapping (ccoleman@redhat.com)
- Allow network to cross compile on master (ccoleman@redhat.com)
- The proxy health server should be on, it does not leak info
  (ccoleman@redhat.com)
- Tolerate being unable to remove /var/run/openshift-sdn (ccoleman@redhat.com)
- Allow resync in oc observe without --names (ccoleman@redhat.com)
- UPSTREAM: 53037: Verify client cert before reusing existing bootstrap
  (ccoleman@redhat.com)
- UPSTREAM: 49899: Update the client cert used by the kubelet on expiry
  (ccoleman@redhat.com)

* Fri Oct 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.150.0
- remove experimental extended builds api (bparees@redhat.com)
- Make admin project creation wait for SAR (mkhan@redhat.com)
- bump(github.com/openshift/source-to-image):
  a0e78cce863f296bfb9bf77ac5acd152dc059e32 (gmontero@redhat.com)
- Remove "template.openshift.io/template-instance" label (jminter@redhat.com)
- Fix crash on last node when deleting second-to-last node (danw@redhat.com)
- wait for pod before waiting for endpoint (bparees@redhat.com)
- Allow the script output path to be overridden (skuznets@redhat.com)
- update discovery with shortnames and categories (deads@redhat.com)
- extended: fixed registry tests (miminar@redhat.com)
- remove tech preview label from jenkins strategy (bparees@redhat.com)
- Ensure that KUBECONFIG and KUBERNETES_MASTER are unset (skuznets@redhat.com)
- fallback to orig err msg if no err causes found (jvallejo@redhat.com)
- Fix typos in pkg/network (rpenta@redhat.com)
- UPSTREAM: <drop>: Adapt etcd testing util to v3.2.8 (maszulik@redhat.com)
- bump(github.com/coreos/etcd): v3.2.8 (maszulik@redhat.com)
- bump(github.com/coreos/etcd): v3.2.1 (maszulik@redhat.com)
- bump(github.com/docker/distribution): remove unnecessary vendor dirs from
  inside docker/distribution (maszulik@redhat.com)
- Fix emicklei/go-restful-swagger12 and remove coreos/etcd from godep-
  restore.sh (maszulik@redhat.com)
- Rebase process description (maszulik@redhat.com)
- UPSTREAM: 52515: changes to upstream commits to fix unit test errors with
  3.7. (avagarwa@redhat.com)
- UPSTREAM: 52515: Clarify predicates name to clean confusing.
  (avagarwa@redhat.com)
- Fix go vet warnings (danw@redhat.com)
- Make govet check glog.Info/Warning calls (danw@redhat.com)
- Image policy should ignore unchanged images on update (ccoleman@redhat.com)
- Write log using t.Log in registry unit tests (obulatov@redhat.com)

* Thu Oct 12 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.149.0
- bump(github.com/openshift/origin-web-console):
  ba1a82013693cce1265bd0d49d18638cd71c767d (eparis+openshiftbot@redhat.com)
- UPSTREAM: 53731: Use locks in fake dbus (ccoleman@redhat.com)
- The DNS subsystem should manage keeping dnsmasq in sync (ccoleman@redhat.com)
- openvswitch, syscontainer: allow access to the devices (gscrivan@redhat.com)
- node, syscontainer: allow access to devices (gscrivan@redhat.com)
- origin, syscontainer: allow access to standard devices (gscrivan@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bcb969bb2dd103823418447569ef9957860aec6b (eparis+openshiftbot@redhat.com)
- catalog: name changes from catalog v0.1.0-rc1 sync (jaboyd@redhat.com)
- Record events on openshift-sdn startup and pod restart (ccoleman@redhat.com)
- UPSTREAM: 53682: Use proper locks when updating desired state
  (hekumar@redhat.com)
- Update minimum docker version required for the node (cewong@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 7011d9e816..3aacfedec6 (jaboyd@redhat.com)
- Use config map for election with start controllers (mkhan@redhat.com)
- Don't allow claiming node IP as egress IP (danw@redhat.com)
- Revert "<carry>: Set external plan name for service-catalog walkthrough"
  (jaboyd@redhat.com)
- Origin changes after cherry-picks (maszulik@redhat.com)
- UPSTREAM: 53457: Ignore notFound when deleting firewall (maszulik@redhat.com)
- UPSTREAM: 53332: Ignore pods for quota that exceed deletion grace period
  (maszulik@redhat.com)
- UPSTREAM: 52604: Use separate client for node status loop
  (maszulik@redhat.com)
- Print the entirety of Go test stderr in warning (skuznets@redhat.com)
- UPSTREAM: 53299: Correct APIGroup for RoleBindingBuilder Subjects
  (maszulik@redhat.com)
- UPSTREAM: 52947: Preserve leading and trailing slashes on proxy subpaths
  (maszulik@redhat.com)
- UPSTREAM: 52775: Fix panic in ControllerManager when GCE external
  loadbalancer healthcheck is nil (maszulik@redhat.com)
- UPSTREAM: 52545: use specified discovery information if possible
  (maszulik@redhat.com)
- UPSTREAM: 52602: etcd3 store: retry w/live object on conflict
  (maszulik@redhat.com)
- UPSTREAM: 51199: Makes Hostname and Subdomain fields of v1.PodSpec settable
  when empty and updates the StatefulSet controller to set them when empty
  (maszulik@redhat.com)
- UPSTREAM: 52823: Third party resources should not be part of conformance
  (maszulik@redhat.com)
- Change golang version from 1.8.3 to 1.8.1 in origin.spec (jhadvig@redhat.com)
- Fix stale omitempty comment in server types (rpenta@redhat.com)
- Remove unneeded extension-apiserver-authentication-reader from service
  catalog example template. Upstream role was added in #16517.
  (jminter@redhat.com)

* Wed Oct 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.148.0
- Move browser safe proxy logic into an authorizer (mkhan@redhat.com)
- Add a healthcheck to detect when OVS is restarted (ccoleman@redhat.com)
- report pod state on template test failure (jminter@redhat.com)
- resolve image secrets in the buildcontroller (bparees@redhat.com)
- Move network type check to inside the network code (ccoleman@redhat.com)
- Use glog local name instead of log (ccoleman@redhat.com)
- report on object readiness in blocking template tests (jminter@redhat.com)
- Fix old replicas count in scaled RC event message (miciah.masters@gmail.com)
- Fix flake in the extended tests (agladkov@redhat.com)
- Update imagestream only once per prune process (agladkov@redhat.com)
- Fix reimporting from insecure registries (obulatov@redhat.com)
- Let servicecatalog-serviceclass-viewer also view plans (spadgett@redhat.com)
- Update/fix HPA controller policy (sross@redhat.com)
- Take into account errors (agladkov@redhat.com)
- sdn: only sync HostPorts when we need to (dcbw@redhat.com)
- Add template-service-broker image (sdodson@redhat.com)
- Create template-service-broker subpackage (sdodson@redhat.com)

* Tue Oct 10 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.147.1
- bump(github.com/openshift/origin-web-console):
  bb25fca825823d626e23ea2e22e78806f2f50065 (eparis+openshiftbot@redhat.com)

* Tue Oct 10 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.147.0
- report registry error when retry gives up (bparees@redhat.com)

* Mon Oct 09 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.146.1
- bump(k8s.io/kubernetes): c84beffa03930fdedb9d523dde9f34506913f2b0
  (maszulik@redhat.com)

* Mon Oct 09 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.146.0
- 

* Mon Oct 09 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.145.0
- bump(github.com/openshift/origin-web-console):
  ea7477e5ebc06d3c2b6b5c7df28ca9c806aa276c (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2e0fbe8ed749c8ee0b4d36a88af53a58ac0d2012 (eparis+openshiftbot@redhat.com)
- fix select error handling after WaitForRunningBuild (gmontero@redhat.com)
- Add pmorie to OWNERS in required packages for service-catalog updates
  (pmorie@redhat.com)
- rework jenkins ext test job retrieval around annotation (gmontero@redhat.com)
- clean up buildconfig webhook handlers (bparees@redhat.com)
- UPSTREAM: 52168: Fix incorrect status msg in podautoscaler
  (mattjmcnaughton@gmail.com)
- <carry>: Set external plan name for service-catalog walkthrough
  (jaboyd@redhat.com)
- Base random controller leader ID on machine info (mkhan@redhat.com)
- remove tons of dead code and small code nit fixes (mfojtik@redhat.com)
- image: fix infinite recursive call (mfojtik@redhat.com)
- Router stats-port=0 error (pcameron@redhat.com)
- catalog: add RBAC for serviceplans and serviceinstances/reference
  (jaboyd@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 06b897d198..7011d9e816 (jaboyd@redhat.com)
- apps: tweak some deployment extended test for speed (mfojtik@redhat.com)
- Register APIService for apiregistration.k8s.io/v1beta1 (obulatov@redhat.com)

* Fri Oct 06 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.144.2
- UPSTREAM: 53135: Fixed counting of unbound PVCs towards limit of attached
  volumes. (jsafrane@redhat.com)

* Fri Oct 06 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.144.1
- Fix extended test namespace provisioning - wait for SA using selfSAR
  (tnozicka@gmail.com)
- bump(github.com/openshift/origin-web-console):
  b642a34b5aed01d2bf704609f38974fe554f81fe (eparis+openshiftbot@redhat.com)
- UPSTREAM: google/cadvisor: 1766: adaptive longOp for du operation
  (sjenning@redhat.com)
- Generate version.txt in conformance-k8s (ccoleman@redhat.com)
- UPSTREAM: 53401: Fix spam of multiattach errors in event logs
  (hekumar@redhat.com)
- UPSTREAM: 51633: update GC controller to wait until controllers have been
  initialized (deads@redhat.com)

* Fri Oct 06 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.144.0
- switch build logs to use client, not storage (deads@redhat.com)
- ensure builder labels override with proper priority (bparees@redhat.com)
- align jenkins ext launch with new generic pod dump on failure
  (gmontero@redhat.com)
- UPSTREAM: 53446: kubelet: add latency metrics to network plugin manager
  (sjenning@redhat.com)
- bump(github.com/openshift/source-to-image):
  df6fb76a860d38f73ef066c199c261a6988cc4ab (bparees@redhat.com)
- sdn: metrics fixes for review comments (dcbw@redhat.com)
- build-go.sh: Fix unbound variable STARTTIME (miciah.masters@gmail.com)
- make context message less noisy (bparees@redhat.com)
- use the upstream admission plugin construction (deads@redhat.com)
- apps: switch back to comparing encoded template config instead of comparing
  rc template (mfojtik@redhat.com)
- UPSTREAM: 53442: add nested encoder and decoder to admission config
  (deads@redhat.com)
- only allow one admission chain for the apiserveR (deads@redhat.com)
- apps: update generations in extended test (mfojtik@redhat.com)
- apps: remove deployment trigger controller (mfojtik@redhat.com)
- add retry to openshift build prometheus ext test initial service access
  (gmontero@redhat.com)
- UPSTREAM: kubernetes-incubator/cluster-capacity: <drop>: update OWNERS
  (skuznets@redhat.com)
- wait for builder service account on necessary templateinstance/tsb tests
  (jminter@redhat.com)
- Generated updates (maszulik@redhat.com)
- UPSTREAM: <drop>: generated updates (maszulik@redhat.com)
- OpenShift changes after the rebase to 1.7.6 (maszulik@redhat.com)
- UPSTREAM: <carry>: openapi generation for
  createNamespacedDeploymentConfigRollback duplication problem
  (maszulik@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Azure dependencies
  (maszulik@redhat.com)
- UPSTREAM: <drop>: Adapt etcd testing util to v3.2.1 (jliggitt@redhat.com)
- UPSTREAM: <drop>: aggregate openapi through servers. 1.8 should fix tthis for
  CRD (maszulik@redhat.com)
- UPSTREAM: <carry>: update namespace lifecycle to allow review APIs
  (deads@redhat.com)
- UPSTREAM: 53318: create separate transports for liveness and readiness probes
  (sjenning@redhat.com)
- UPSTREAM: 52864: dockershim: fine-tune network-ready handling on sandbox
  teardown and removal (dcbw@redhat.com)
- UPSTREAM: 47806: kubelet: fix inconsistent display of terminated pod IPs by
  using events instead (dcbw@redhat.com)
- UPSTREAM: 53069: Align imagefs eviction defaults with image gc defaults
  (sjenning@redhat.com)
- UPSTREAM: 51035: Show events when describing service accounts
  (mrogers@redhat.com)
- UPSTREAM: 51972: ProducesObject should only update the returned API object
  resource documentation (jminter@redhat.com)
- UPSTREAM: 52112: Allow watch cache disablement per type (ccoleman@redhat.com)
- UPSTREAM: 51796: Fix pod and node names switched around in error message.
  (jminter@redhat.com)
- UPSTREAM: 52691: FC plugin: Return target wwn + lun at GetVolumeName()
  (hchen@redhat.com)
- UPSTREAM: 52687: Refactoring and improvements for iSCSI and FC storage
  plugins (hchen@redhat.com)
- UPSTREAM: 52675: Fix FC WaitForAttach not mounting a volume
  (hchen@redhat.com)
- UPSTREAM: 50036: Bring volume operation metrics (hekumar@redhat.com)
- bump(k8s.io/kubernetes): a08f5eeb6246134f4ae5443c0593d72fd057ea7c
  (maszulik@redhat.com)
- bump(github.com/emicklei/go-restful-swagger12):
  885875a92c2ab7d6222e257e41f6ca2c1f010b4e (maszulik@redhat.com)
- bump(github.com/google/cadvisor): c683567ed073eb6bcab81cccee79cd64a0e33811
  (maszulik@redhat.com)
- bump(github.com/docker/distribution):
  1e2bbed6e09c6c8047f52af965a6e301f346d04e (maszulik@redhat.com)
- bump(github.com/containers/image): dbd0a4cee2480da39048095a326506ae114d635a
  (maszulik@redhat.com)
- bump(github.com/fatih/structs): 7e5a8eef611ee84dd359503f3969f80df4c50723
  (maszulik@redhat.com)
- Update hack/godep-restore.sh to match fork names (maszulik@redhat.com)
- Made the router skip health checks when there is one endpoint
  (bbennett@redhat.com)
- have tsb provision tests timeout after 20 minutes (jminter@redhat.com)
- image-pruning: Improve help and error reporting (miminar@redhat.com)
- apps: add unit test for deployment config metrics (mfojtik@redhat.com)
- Run more e2e tests than we were before by simplifying our filters
  (ccoleman@redhat.com)
- UPSTREAM: <drop>: Fix gc test until 1.8 (ccoleman@redhat.com)
- use PodSecurityPolicySubjectReview in build controller to avoid actually
  submitting a pod (jminter@redhat.com)
- filter out 'turn this on' config structs (deads@redhat.com)
- UPSTREAM: <carry>: allow a filter function on admission registration
  (deads@redhat.com)
- oc process: show multiple parameter errors where present (jminter@redhat.com)
- Sleep at the end of every `hack/env` invocation for logs
  (skuznets@redhat.com)
- add tests (jvallejo@redhat.com)
- Use `os::cmd` in verification scripts (skuznets@redhat.com)
- update completions (jvallejo@redhat.com)
- add --sub-path opt to set-volume cmd (jvallejo@redhat.com)

* Wed Oct 04 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.143.1
- bump(github.com/openshift/origin-web-console):
  19ffee3e26eec1a757e1c2dc3e0df8e471b68293 (eparis+openshiftbot@redhat.com)

* Wed Oct 04 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.143.0
- wrap aftereach in a context so it runs before the k8s namespace cleanup
  (bparees@redhat.com)
- format error returned from failed templateinstance less unpleasantly
  (jminter@redhat.com)
- regenerate files (danw@redhat.com)
- Misc auto egress IP fixes (danw@redhat.com)
- Only watch EgressIPs with multitenant and networkpolicy plugins
  (danw@redhat.com)
- Switch to stateful set in prometheus (ccoleman@redhat.com)
- Test for bug 1487408 (obulatov@redhat.com)
- Pruning should take all the images into account (agladkov@redhat.com)
- Switch from defaulting to converting clusterNetworkCIDR (ccoleman@redhat.com)
- reverse default for jenkins ext test mem monitor (gmontero@redhat.com)
- annotate TemplateInstance objects created by TSB with instance UUID
  (jminter@redhat.com)
- separate openshift_template_instance_status_condition_total and
  openshift_template_instance_total metrics (jminter@redhat.com)

* Tue Oct 03 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.142.0
- add e2e test for rsh to statefulset (mfojtik@redhat.com)
- cli: add statefulsets to PodForResource (mfojtik@redhat.com)
- more build metric massage; add unit test; adjust integration test
  (gmontero@redhat.com)
- apps: record cause of rollout and deployer pods timestamps back to rc
  (mfojtik@redhat.com)
- apps: fix logic error in last failed rollout metric (mfojtik@redhat.com)
- UPSTREAM: 53318: create separate transports for liveness and readiness probes
  (sjenning@redhat.com)
- Fix missing sizes for manifest schema 1 images (miminar@redhat.com)

* Tue Oct 03 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.141.0
- bump(github.com/openshift/origin-web-console):
  9c144f35a308a00a12b1fcdde253cb740157f2bd (eparis+openshiftbot@redhat.com)

* Tue Oct 03 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.140.0
- Fix route checking in alreadySetUp (danw@redhat.com)

* Mon Oct 02 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.139.0
- 

* Mon Oct 02 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.138.0
- Make DC extended ControllerRef test more resilient (tnozicka@gmail.com)

* Mon Oct 02 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.137.0
- 

* Sun Oct 01 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.136.0
- bump(github.com/openshift/origin-web-console):
  752a6aee510437c5fdfdf401727bd28267acf106 (eparis+openshiftbot@redhat.com)
- Wipe out any existing content in the workdir before cloning
  (bparees@redhat.com)

* Sat Sep 30 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.135.0
- Deleted eventQueue, no longer used (rpenta@redhat.com)
- Replaced event queue based watching resources in router with shared informers
  (rpenta@redhat.com)
- remove legacy client from integration tests (deads@redhat.com)
- remove legacy client! (deads@redhat.com)
- remove last of the client usage (deads@redhat.com)
- convert to groupified scc (deads@redhat.com)
- fix LSAR client (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  58d1eebe8b46a16b53c0f47051f48824002fc394 (eparis+openshiftbot@redhat.com)
- tolerate cross-platform building of crio/cgroup build logic
  (bparees@redhat.com)
- sdn: promote setup log messages to V(2) (dcbw@redhat.com)
- Implement the node side of automatic egress IP support (danw@redhat.com)
- Regenerate files (danw@redhat.com)
- refactor apiserver start to avoid multiple overwrites and side channels
  (deads@redhat.com)
- convert CLI to use generated clients (deads@redhat.com)
- UPSTREAM: <carry>: update namespace lifecycle to allow review APIs
  (deads@redhat.com)
- add NetNamespace.EgressIPs and HostSubnet.EgressIPs (danw@redhat.com)
- UPSTREAM: 52864: dockershim: fine-tune network-ready handling on sandbox
  teardown and removal (dcbw@redhat.com)
- switch to the upstream factory client method where possible
  (deads@redhat.com)
- fix generated clients (deads@redhat.com)
- cli: update generate and bootstrap to use generated clients
  (mfojtik@redhat.com)
- don't explicitly set the jvm architecture, let the image pick one
  (bparees@redhat.com)
- Router support for Strict-Transport-Security (hsts) (pcameron@redhat.com)
- fix up image gc settings in defaults tests (sjenning@redhat.com)
- UPSTREAM: 53069: Align imagefs eviction defaults with image gc defaults
  (sjenning@redhat.com)
- fix tag race condition in images.sh (bparees@redhat.com)
- sdn: add some prometheus metrics (dcbw@redhat.com)
- auto generated files (jtanenba@redhat.com)
- Allowing multiple CIDR addresses for allocation of Nodes
  (jtanenba@redhat.com)
- update rolebindingaccessor to use generated clients (deads@redhat.com)
- Allow oc extract to output to stdout (ccoleman@redhat.com)
- UPSTREAM: 47806: kubelet: fix inconsistent display of terminated pod IPs by
  using events instead (dcbw@redhat.com)
- Modify nonroot, hostaccess, and hostmount-anyuid SCCs to drop some
  capabilities. (vsemushi@redhat.com)

* Thu Sep 28 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.134.0
- Return error instead of glog.Fatalf (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  36bd0fb71e0811c5838d02ea8b7cdfae5275c6d6 (eparis+openshiftbot@redhat.com)
- metrics, readme changes to prep for prometheus alerts for openshift build
  subsystem (gmontero@redhat.com)
- tsb: return error description on failed async operation (jminter@redhat.com)
- setup crio networking for build containers (bparees@redhat.com)
- bump(github.com/kubernetes-incubator/crio):
  a8ee86b1cce0c13bd541a99140682a92635ba9f7 (bparees@redhat.com)
- UPSTREAM: fsouza/go-dockerclient: <carry>: support volume mounts in docker
  build api, RH specific (bparees@redhat.com)

* Thu Sep 28 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.133.0
- Refine prometheus auth metrics (mrogers@redhat.com)
- Wire advanced audit to asset and oauth api servers (maszulik@redhat.com)
- Add PKCE support to oc (mkhan@redhat.com)
- Fix hack/godep-save.sh (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image):
  06c9446cd6b580cbcb13a1489efcb3e943b470af - fix deps (maszulik@redhat.com)
- switch some test-integration clients (deads@redhat.com)
- update imagestreammappingclient to work (deads@redhat.com)
- convert test-extensions to generated client (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d8d6ca4d2f7e9f49db2084f82e914d41b25c7129 (eparis+openshiftbot@redhat.com)
- image: add image signature importer controller (mfojtik@redhat.com)
- bump(github.com/opencontainers/image-spec):
  ef2b9a1d696677abd664a0879758d2b115b1ded3 (mfojtik@redhat.com)
- bump(github.com/docker/go-connections):
  3ede32e2033de7505e6500d6c868c2b9ed9f169d (mfojtik@redhat.com)
- bump(github.com/containers/image): dbd0a4cee2480da39048095a326506ae114d635a
  (mfojtik@redhat.com)
- add pod state/log dumping to all build/image extended tests
  (bparees@redhat.com)
- Generated changes to bootstrappolicy test data (mkhan@redhat.com)
- Bootstrap Kube namespaced roles and bindings (mkhan@redhat.com)
- implement prometheus metrics for the TemplateInstance controller
  (jminter@redhat.com)
- build: fix field selector in startbuild (mfojtik@redhat.com)
- switch reconciliation to generated clients (deads@redhat.com)
- build: register PodProxyOptions to build schema (mfojtik@redhat.com)
- cli: fix more clients (mfojtik@redhat.com)
- project and chaindescriber_test (mfojtik@redhat.com)
- cli: fix rest of the cli and describers (mfojtik@redhat.com)
- cli: use generated clients in describer (mfojtik@redhat.com)
- build: make start-build use generated client (mfojtik@redhat.com)
- build: use rest.Interface in webhooks client (mfojtik@redhat.com)
- apps: fix prune and rollout latest clients (mfojtik@redhat.com)
- build: remove generated instantiateBinary method cause it has unsupoported
  signature (mfojtik@redhat.com)
- cli: use generated clients in projects (mfojtik@redhat.com)
- project: add client for projectRequests (mfojtik@redhat.com)
- cli: fix images (mfojtik@redhat.com)
- Support HOST:80 in .docker/config.json like Docker (ccoleman@redhat.com)

* Wed Sep 27 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.132.0
- Switch to 1.8 conformance (ccoleman@redhat.com)
- UPSTREAM: 51035: Show events when describing service accounts
  (mrogers@redhat.com)
- Add API events for SA OAuth failures (mrogers@redhat.com)

* Tue Sep 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.131.0
- Use the release-1.7 branch for conformance (ccoleman@redhat.com)
- Minor cleanup to top images* - sorting, size, tabs (ccoleman@redhat.com)
- Change timeSpec name and coding (pcameron@redhat.com)
- Use an annotation to provide a route cookie (pcameron@redhat.com)
- Do not include accept-proxy in portlist when using proxy protocol
  (magnus.bengtsson@expressen.se)

* Tue Sep 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.130.0
- enable base prometheus metrics for template service broker
  (jminter@redhat.com)
- Add NODE_SELECTOR parameter on TSB template Set empty openshift.io/node-
  selector annotation on namespace during TSB tests (jminter@redhat.com)

* Tue Sep 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.129.0
- add template-service-broker command (deads@redhat.com)

* Tue Sep 26 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.128.0
- bump(github.com/openshift/origin-web-console):
  cf9c610fe4fcce12b88069e5108ae488abd59e71 (eparis+openshiftbot@redhat.com)
- simplify controller startup (deads@redhat.com)
- remove most legacy client usage from diagnostics (deads@redhat.com)
- move eventQueue to the only package using it (deads@redhat.com)
- remove legacy client usage (deads@redhat.com)
- add new scaler namespacer with external type (deads@redhat.com)
- use reconcilation to ensure rbac resources (deads@redhat.com)
- switch to generated apps listers (deads@redhat.com)
- tidy up owners files and move naughty pacakge (deads@redhat.com)
- remove exit (pweil@redhat.com)
- add deploymentconfig lister methods (deads@redhat.com)
- Update generated files (stefan.schimanski@gmail.com)
- bump(github.com/openshift/origin-web-console):
  c1a018662d8f73d402774706101575505361080d (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1dae20b4c822489dede0ce84c8fad1af35d2927a (eparis+openshiftbot@redhat.com)
- Enable deepcopy-gen in packages where needed (stefan.schimanski@gmail.com)
- Add +k8s:deepcopy-gen:interfaces tags to runtime.Object impls in tests
  (stefan.schimanski@gmail.com)
- Add +k8s:deepcopy-gen:interfaces tags to runtime.Object impls
  (stefan.schimanski@gmail.com)
- Set access token expiration correctly for code and implicit flows
  (jliggitt@redhat.com)
- bump(github.com/openshift/source-to-image):
  9dfd4eed18adfc112b8ed42b4a0945d46ef011d0 (bparees@redhat.com)
- switch rest of the controllers to use generated clients (mfojtik@redhat.com)
- api: pass proper parameterCodec to api group info (mfojtik@redhat.com)
- cli: switch to generated clients (mfojtik@redhat.com)
- update generated completions (jvallejo@redhat.com)
- add tests (jvallejo@redhat.com)
- add --dry-run --output opts -> modify_scc (jvallejo@redhat.com)
- add --dry-run --output support to modify_roles (jvallejo@redhat.com)
- Mark all of the SDN code "+build linux", to fix unit tests on OS X
  (danw@redhat.com)
- add --output & --dry-run options oc-adm-policy... (jvallejo@redhat.com)
- remove scheduled import patch test (bparees@redhat.com)
- bump(k8s.io/kubernetes): b0608fa189530bca78d7459a87318652b116171e
  (deads@redhat.com)
- api: pass proper parameterCodec to api group info (mfojtik@redhat.com)
- cli: switch to generated clients (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0ffc72d9d1063529fbe0ba79de040d297ed5c786 (eparis+openshiftbot@redhat.com)
- remove last of the bad api deps (deads@redhat.com)
- remove dependency on kube api helpers from api types (deads@redhat.com)
- remove last docker distribution dependency from api (deads@redhat.com)
- remove legacy client from clusterquota (deads@redhat.com)
- removing legacy client from strategyrestrictions (deads@redhat.com)
- restrictuser admission legacy client removal (deads@redhat.com)
- remove legacy client deps (deads@redhat.com)
- project: switch to generated clientset (mfojtik@redhat.com)
- UPSTREAM: 50036: Bring volume operation metrics (hekumar@redhat.com)
- oc new-app expose message logic (m.judeikis@gmail.com)
- remove last docker distribution dependency from api (deads@redhat.com)
- UPSTREAM: 52691: FC plugin: Return target wwn + lun at GetVolumeName()
  (hchen@redhat.com)
- UPSTREAM: 52687: Refactoring and improvements for iSCSI and FC storage
  plugins (hchen@redhat.com)
- UPSTREAM: 52675: Fix FC WaitForAttach not mounting a volume
  (hchen@redhat.com)
- UPSTREAM: opencontainers/runc: 1344: fix cpu.cfs_quota_us changed when
  systemd daemon-reload using systemd (sjenning@redhat.com)
- catalog: enable OriginatingIdentity (jaboyd@redhat.com)
- Reduce the log level of regex debugging statements (bbennett@redhat.com)
- Add a suite test that tests the upstream conformance (ccoleman@redhat.com)
- catalog: added new admission controller BrokerAuthSarCheck and updated
  bindata (jaboyd@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from ae6b643caf..06b897d198 (jaboyd@redhat.com)
- Drop double conversion in boostrap policy (cheimes@redhat.com)
- Basic audit extended test (maszulik@redhat.com)
- Enable full advanced audit in origin (maszulik@redhat.com)
- Select manifest for default platform from manifest lists
  (obulatov@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Revert "disable manifest list
  registration" (obulatov@redhat.com)
- UPSTREAM: 51782: A policy with 0 rules should return an error
  (maszulik@redhat.com)
- UPSTREAM: 52030: Fill in creationtimestamp in audit events
  (maszulik@redhat.com)
- UPSTREAM: 51119: Allow audit to log authorization failures
  (maszulik@redhat.com)
- UPSTREAM: 48605: support json output for log backend of advanced audit
  (maszulik@redhat.com)
- sdn: disable hostport handling when CRIO is used (dcbw@redhat.com)
- tls edge support add nginx to build local images script (rchopra@redhat.com)
- contain annotations to the appropriate group (deads@redhat.com)
- nginx router based on template (rchopra@redhat.com)
- remove user specification via request parameter for TSB (gmontero@redhat.com)
- Make focus and skip a lot easier to use in extended tests
  (ccoleman@redhat.com)
- Switch to the upstream style for Features (ccoleman@redhat.com)
- UPSTREAM: onsi/ginkgo: 371: Allow tests to be modified (ccoleman@redhat.com)
- Use utilruntime.HandleError for errors in login.go (mrogers@redhat.com)
- Print more details when network diagnostics test setup fails
  (rpenta@redhat.com)
- Bug 1481147 - Fix default pod image for network diagnostics
  (rpenta@redhat.com)
- Rebase cluster-capacity to 0.3.0 version that is rebased to kubernetes 1.7.
  (avagarwa@redhat.com)
- UPSTREAM: 52297: Use cAdvisor constant for crio imagefs (sjenning@redhat.com)
- UPSTREAM: 52073: Fix cross-build (sjenning@redhat.com)
- UPSTREAM: 51728: Enable CRI-O stats from cAdvisor (sjenning@redhat.com)
- Added networking team members to the relevant oc adm commands
  (bbennett@redhat.com)
- Disable the watch cache for most resources by default (ccoleman@redhat.com)
- React to changes in watch cache initialization (ccoleman@redhat.com)
- UPSTREAM: 52112: Allow watch cache disablement per type (ccoleman@redhat.com)
- Instead of launching kubelet directly, exec (ccoleman@redhat.com)
- Remove two deprecated / unnecessary default overrides in node
  (ccoleman@redhat.com)
- Support --v and --vmodule silently (ccoleman@redhat.com)
- UPSTREAM: 52597: Support flag round tripping (ccoleman@redhat.com)
- UPSTREAM: 51796: Fix pod and node names switched around in error message.
  (jminter@redhat.com)
- ensure node exists if --node-name given (jvallejo@redhat.com)
- Exit if there is no ClusterID and allow-untagged-cluster isn't set.
  (rrati@redhat.com)
- UPSTREAM: 49215: Require Cluster ID for AWS (rrati@redhat.com)
- UPSTREAM: 48612: Warn if cluster ID is missing for AWS (rrati@redhat.com)
- api docs should show right return value for build instantiate{binary,}
  (jminter@redhat.com)
- UPSTREAM: 51972: ProducesObject should only update the returned API object
  resource documentation (jminter@redhat.com)

* Thu Sep 21 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.127.0
- bump(github.com/openshift/origin-web-console):
  444e25c4351d52160eef428f1b526cf37d0519a2 (eparis+openshiftbot@redhat.com)
- remove dependency on github.com/fsouza/go-dockerclient from imageapi
  (deads@redhat.com)
- remove image api dependency on docker/distribution/schemaX (deads@redhat.com)
- move existing image utils to the single point where they are used
  (deads@redhat.com)
- Add security team to reviewers for security pkgs (mkhan@redhat.com)
- remove authorization dependency on validation (deads@redhat.com)
- cli: provide generated clients via ClientAccessFactory (maszulik@redhat.com)
- move authorization conversion out to fix dependencies (deads@redhat.com)
- servcert: switch to listers (mfojtik@redhat.com)

* Wed Sep 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.6
- move some naming helpers to apihelpers (deads@redhat.com)
- Fix image layers order (agladkov@redhat.com)
- hack/build-images.sh: fix image build order (jdetiber@redhat.com)
- Add Prometheus metrics for authentication attempts (mrogers@redhat.com)
- UPSTREAM: revert: bc8249cc6f35fabffff5ccf4e13978ff9a6a2e32: "UPSTREAM:
  <carry>: allow PV controller recycler template override"
  (jsafrane@redhat.com)
- UPSTREAM: <carry>: increase timeout in TestCancelAndReadd even more
  (maszulik@redhat.com)
- Add OpenShift's recycler templates to Kubernetes controller config
  (jsafrane@redhat.com)
- UPSTREAM: 51553: Expose PVC metrics via kubelet prometheus
  (mawong@redhat.com)
- UPSTREAM: 51448: Add PVCRef to VolumeStats (mawong@redhat.com)

* Wed Sep 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.5
- bump(github.com/openshift/origin-web-console):
  1ceb0ff2382a33f7548741192b8e2c93c790ad22 (eparis+openshiftbot@redhat.com)
- catalog: RBAC - added get for service-catalog-controller role for brokers,
  instances and credentials (jaboyd@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6f40623444af501083edb4979a9b833ccfc5f026 (eparis+openshiftbot@redhat.com)
- cli: prefer public docker image repository (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e1ba88dcfd8e36525631d0ed676d8bd073a292ef (eparis+openshiftbot@redhat.com)
- Switch pkg/image to use clientsets (maszulik@redhat.com)
- Rename MakeImageStreamImageName to be consistent with JoingImageStreamTag
  (maszulik@redhat.com)
- Switch imageimport to use k8s SAR client (maszulik@redhat.com)
- UPSTREAM: google/cadvisor: 1706: oomparser: don't get stuck for certain
  processes (sjenning@redhat.com)
- Use writeable HOME directory for builder unit test (cewong@redhat.com)
- provide a specific order for template parameters in the TSB
  (bparees@redhat.com)
- UPSTREAM: opencontainers/runc: 1378: Expose memory.use_hierarchy in
  MemoryStats (sjenning@redhat.com)
- UPSTREAM: google/cadvisor: 1741: add CRI-O handler (sjenning@redhat.com)
- UPSTREAM: google/cadvisor: 1728: Expose total_rss when hierarchy is enabled
  (sjenning@redhat.com)
- remove install dependencies from api packages (deads@redhat.com)
- Generated files (jliggitt@redhat.com)
- Per-client access token expiration (jliggitt@redhat.com)
- Allow running a subset of the integration tests (mkhan@redhat.com)
- registry: add dockerregistryurl optional to middleware config
  (mfojtik@redhat.com)
- add ownerrefs to SSCS (deads@redhat.com)
- remove install dependency from images (deads@redhat.com)
- registry: use the privileged client to get signatures (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1b695d8c30d0638a78facc397693a38f26d7efdf (eparis+openshiftbot@redhat.com)
- Wire node authorizer (jliggitt@redhat.com)
- Enable NodeRestriction admission (jliggitt@redhat.com)
- UPSTREAM: 49638: Remove default binding of system:node role to system:nodes
  group (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5bf079c931b132c88b53521eddb60328023ceaa9 (eparis+openshiftbot@redhat.com)
- Add `oc create imagestreamtag` (ccoleman@redhat.com)
- only install component specific templates when requested (bparees@redhat.com)
- UPSTREAM: 52221: Always populate volume status from node (hekumar@redhat.com)
- Split networking out from node initialization into its own package
  (ccoleman@redhat.com)
- Separate config and options specific code for node from start
  (ccoleman@redhat.com)
- UPSTREAM: 48583: Record 429 and timeout errors to prometheus
  (ccoleman@redhat.com)
- dump pod state and logs on failure (bparees@redhat.com)
- node,syscontainer: enable Type=notify (gscrivan@redhat.com)
- node,syscontainer: initialize dnsmasq (gscrivan@redhat.com)
- Convert subjectchecker to use rbac.Subject (simo@redhat.com)
- update field selectors for all resources (deads@redhat.com)
- update route field selectors (deads@redhat.com)
- dump pod state on test failure (bparees@redhat.com)
- Use discovery based version gating for policy commands (mrogers@redhat.com)
- handle build describer edge-case (jvallejo@redhat.com)
- catalog: added new admission controllers and updated bindata
  (jaboyd@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from ef63307bdb..ae6b643caf (jaboyd@redhat.com)
- build: replace legacy client with internal client set (mfojtik@redhat.com)
- UPSTREAM: 48226: Log get PVC/PV errors in MaxPD predicate only at high
  verbosity. (avagarwa@redhat.com)
- bump(github.com/gophercloud/gophercloud):
  ed590d9afe113c6107cd60717b196155e6579e78 (ppospisi@redhat.com)
- template: add special purpose process template client (mfojtik@redhat.com)
- build: add special purpose client for build logs (mfojtik@redhat.com)
- build: generate missing client methods (mfojtik@redhat.com)
- apps: add rollouts metrics (mfojtik@redhat.com)
- UPSTREAM: 50334: Support iscsi volume attach and detach
  (mitsuhiro.tanino@hds.com)
- Re-enable NodePort test (danw@redhat.com)
- UPSTREAM: <drop>: add debugging to NodePort test (danw@redhat.com)
- UPSTREAM: 49025: fix NodePort test on baremetal installs (danw@redhat.com)
- Disallow @ character in URL patterns, so that people don't mistakenly try to
  add URL patterns of the form username@hostname/path. (jminter@redhat.com)
- run kube controllers separately based on their command (deads@redhat.com)
- Promote image trigger annotation to GA and correct syntax
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: allow controller context injection to share informers
  (deads@redhat.com)
- Drop pkg/util/ipcmd, port to vishvananda/netlink (danw@redhat.com)
- bump(github.com/vishvananda/netlink):
  933b978eae8c18daa1077a0eb7186b689cd9f82d (danw@redhat.com)
- UPSTREAM: <carry>: Fix to avoid REST API calls at log level 2.
  (avagarwa@redhat.com)
- UPSTREAM: 49420: Fix c-m crash while verifying attached volumes
  (hekumar@redhat.com)
- allow sys_chroot cap on SCCs (pweil@redhat.com)
- Change git command for description extraction (jpeeler@redhat.com)

* Fri Sep 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.4
- Fix some minor syntax warnings (jpeeler@redhat.com)
- Teach build-local-images to build service-catalog (jpeeler@redhat.com)
- Add App struct for application-scoped data (obulatov@redhat.com)
- image: add registry-url flag for verify-image-signature command
  (mfojtik@redhat.com)
- dind: fix token race, enable GENEVE UDP port, and update OVN repo
  (dcbw@redhat.com)
- UPSTREAM: 48524: fix udp service blackhole problem when number of backends
  changes from 0 to non-0 (danw@redhat.com)
- Require conntrack-tools in node package (danw@redhat.com)
- prometheus annotations dropped from router service (pcameron@redhat.com)
- Install conntrack-tools in dind node image (danw@redhat.com)
- Generate escaped regexes for cors config (jliggitt@redhat.com)
- UPSTREAM: 51644: do not update init containers status if terminated
  (sjenning@redhat.com)
- UPSTREAM: 49640: Run mount in its own systemd scope (jsafrane@redhat.com)

* Fri Sep 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.3
- bump(github.com/openshift/origin-web-console):
  9148f611dd81dc4f9939dab6c6395a7319063bc3 (eparis+openshiftbot@redhat.com)
- fix http codes for router e2e (sjenning@redhat.com)
- improve debug for maven jenkins pipeline failures (gmontero@redhat.com)
- dind: add support for CRI-O runtime via "-c crio" start argument
  (dcbw@redhat.com)
- Avoid printing node list for LoadBalancer in log file (ichavero@redhat.com)
- unify api helpers and snip some bad deps (deads@redhat.com)

* Thu Sep 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.2
- wait longer for db availability (bparees@redhat.com)
- better failure logging for extended build test (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b16134b1745edcc800b2a2340f9f61e261a804ba (eparis+openshiftbot@redhat.com)
- Decrement retries count during migration (mkhan@redhat.com)
- increase timeouts for pod deletion (bparees@redhat.com)
- Not found errors must match object in migration (mkhan@redhat.com)
- increase waiting for pods to start due to delays from running in parallel
  (bparees@redhat.com)
- Disable TestImageStreamImportDockerHub integration test to unblock the queue
  (maszulik@redhat.com)
- do not enable pod presets, they've been removed from 3.7 (bparees@redhat.com)
- UPSTREAM: docker/distribution: <carry>: disable manifest list registration
  (mfojtik@redhat.com)
- image: set error when we receive unknown schema for the image
  (mfojtik@redhat.com)
- Helm Tiller template (jminter@redhat.com)
- Use mapping for LDAP sync/prune w/ Openshift group (mkhan@redhat.com)
- UPSTREAM: 52344: Do not log spam image pull backoff (ccoleman@redhat.com)
- snip bad dependencies from dockerregistry (deads@redhat.com)
- Add TLS timeout to build image operations retriable errors
  (cewong@redhat.com)
- catalog: rename resources and add admission controller (jpeeler@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 7e650e7e39..ef63307bdb (jpeeler@redhat.com)
- Handle the changed --haproxy.scrape-uri argument (- to --)
  (bbennett@redhat.com)
- add some policy cmd-test (shiywang@redhat.com)
- UPSTREAM: 49142: Slow-start batch pod creation of rs, rc, ds, jobs
  (joesmith@redhat.com)
- Send HTTP Unauthorized (401) for router metrics URL (bbennett@redhat.com)

* Wed Sep 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.1
- bump(github.com/openshift/origin-web-console):
  9673fad07166ed5e958b9024848bcb3359069e5c (eparis+openshiftbot@redhat.com)
- handle kube-gen rename (deads@redhat.com)
- apps: use patch when pausing dc on delete (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ca54beccf85b385203197649736ef8aee532e37e (eparis+openshiftbot@redhat.com)
- oc new-app should not time out when using a proxy (cdaley@redhat.com)
- add dynamic rest mapper to the admission plugin initializer
  (deads@redhat.com)
- bump(k8s.io/code-generator): UPSTREAM: <drop>: handle kube-gen rename
  (deads@redhat.com)
- bump(k8s.io/code-generator): UPSTREAM: <drop>: rename generators to match
  upstream (deads@redhat.com)
- ignore API server timeout errors in templateinstance controller readiness
  checking (jminter@redhat.com)
- ensure `oc get` handles mixed resource types (jvallejo@redhat.com)
- Fix getByKey to return all errors (maszulik@redhat.com)
- Mark package level flags as hidden for completions (ccoleman@redhat.com)
- Added networking team members to the hack directory OWNERS
  (bbennett@redhat.com)
- remove dead TSB code (deads@redhat.com)
- UPSTREAM: 48502: Add a refreshing discovery client (deads@redhat.com)

* Mon Sep 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.126.0
- wire CORS through (deads@redhat.com)
- UPSTREAM: 52127: Fix deployment timeout reporting (mkargaki@redhat.com)
- use offsets for test failure reporting (bparees@redhat.com)
- better failure reporting in postgres test (use offset, dump pods)
  (bparees@redhat.com)
- Do not ignore RBAC during storage migration (mkhan@redhat.com)
- run the run_policy tests serially (bparees@redhat.com)
- bump(github.com/kubernetes/kubernetes):930b5c4b2db (ccoleman@redhat.com)
- Remove policy and policybinding from bootstrap policy (simo@redhat.com)
- Move RoleBindingRestriction admission to work on RBAC only (simo@redhat.com)
- Fix deployment minReadySecond check for availableReplicas after all pods are
  ready (tnozicka@gmail.com)
- Allow credential mapping from dockercfg for canonical ports
  (ccoleman@redhat.com)
- tweaks and rebase (deads@redhat.com)
- Unify and simplify legacy api installation (stefan.schimanski@gmail.com)
- Update etcd path test to always use kindWhiteList
  (stefan.schimanski@gmail.com)
- Remove Store.ImportPrefix everywhere (stefan.schimanski@gmail.com)
- new-app: fix stack overflow when resolving imagestream reference to a
  different imagestream (cewong@redhat.com)
- UPSTREAM: 50094: apimachinery: remove pre-apigroups import path logic
  (stefan.schimanski@gmail.com)
- add more useful queries (jeder@redhat.com)
- catalog: add update permission for bindings/finalizers (jpeeler@redhat.com)
- use rbac for TSB templates (deads@redhat.com)
- limit imports to dockerregistry (deads@redhat.com)
- update Apache image stream naming, Apache QuickStart, and add Ruby 2.4 image
  stream (cdaley@redhat.com)
- Allow more control over the scopes requested by image import
  (ccoleman@redhat.com)
- UPSTREAM: 52092: Fix resource quota controller panic (Drop in 1.8)
  (ironcladlou@gmail.com)
- snip TSB links (deads@redhat.com)
- UPSTREAM: 49416: FC volume plugin: remove block device at DetachDisk
  (hchen@redhat.com)
- Grant access to privileged SCC to system:admin user and members of
  system:masters group. (vsemushi@redhat.com)
- Users with sudoer role now is able to execute commands on behalf of
  system:masters group (--as-group). (vsemushi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  db9abb597886a9d6ddb556ebc35aae6e98fc46ab (eparis+openshiftbot@redhat.com)
- UPSTREAM: 45345: Support "fstype" parameter in dynamically provisioned PVs
  (jsafrane@redhat.com)
- fix help for create imagestream (bparees@redhat.com)
- cleanup imagestreamtag desc (bparees@redhat.com)
- deprecate imagestream dockerImageRepository field (bparees@redhat.com)
- Updating docker --build-arg test due to docker code change
  (cdaley@redhat.com)
- Ensure that StepExecPostCommit is not recorded if no PostCommit exists
  (cdaley@redhat.com)
- Use rbac.PolicyRule directly for DiscoveryRule (mrogers@redhat.com)
- Rename pkg/deploy -> pkg/apps (maszulik@redhat.com)
- remove trailing newline from oc-get-users (jvallejo@redhat.com)
- Add registry team to OWNERS of end-to-end tests (obulatov@redhat.com)
- ClusterRegistry diagnostic: fix address mismatch (lmeyer@redhat.com)
- use the upstream handler chain (deads@redhat.com)
- cleanup remaining storage impls (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  77a9ae80dad4584bfc457f4ea0fab006e65adbdb (eparis+openshiftbot@redhat.com)
- Check the order of bootstrapped SCCs. (jpazdziora@redhat.com)
- Correctly validate identity provider username (mkhan@redhat.com)
- UPSTREAM: google/cadvisor: 1700: Reduce log spam when unable to get network
  stats (sjenning@redhat.com)
- Fixing build pruning tests (cdaley@redhat.com)
- update controller roles (deads@redhat.com)
- UPSTREAM: 49133: add controller permissions to set blockOwnerDeletion
  (deads@redhat.com)
- switch to upstream impersonation (deads@redhat.com)
- UPSTREAM: 49219: Use case-insensitive header keys for --requestheader-group-
  headers. (deads@redhat.com)
- dind: simplify network plugin argument (dcbw@redhat.com)
- dind: add support for ovn-kubernetes network plugin (dcbw@redhat.com)
- Add JENKINS_SERVICE_NAME as env var (shebert@redhat.com)
- bump(github.com/openshift/origin-web-console):
  73c7420693a657ede2157b89d306e2937affc031 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 50843: FlexVolume: Add ability to control 'SupportsSELinux' during
  driver's init phase (mawong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2d2e2975a51af07c6960257319ca9262039ad665 (eparis+openshiftbot@redhat.com)
- clarify imagestreamtag descriptions (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b919687019fbf02c787a88f2af4e6db256dd6c8a (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2fbc58a641d45e1650f41f63567798d6d7ac3fb4 (eparis+openshiftbot@redhat.com)
- move authorization storage to separate server (deads@redhat.com)
- move oauth storage to server (deads@redhat.com)
- move image storage to apiserver (deads@redhat.com)
- move the docker registry v1 client (deads@redhat.com)
- switch image api to use SAR client, not registry (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0e7d693bad94e070cda8ed1661e1519ea705026d (eparis+openshiftbot@redhat.com)
- apps: update pkg/deploy/cmd to use generated client (mfojtik@redhat.com)
- generate rollback client (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4f8b2cfba882fe73471b4e2eac2ec7c23d8b68c3 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4d49437d637c7809bcf6367777b8fc02f8f3e8a0 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6ff495e81538daf7af8473930e8d51b9621fc2bc (eparis+openshiftbot@redhat.com)
- registry: use imagestream clientset to get secrets (mfojtik@redhat.com)
- message tweaks for kube (deads@redhat.com)
- use configmaps for scheduler lease (deads@redhat.com)
- allow cluster up/tsb to tolerate any version of 3.7.x, prerelease or
  otherwise (bparees@redhat.com)
- switch route extra auth check to SAR client (deads@redhat.com)
- Move project creation to use RBAC objects (simo@redhat.com)
- build controller: use a buildconfig queue to track which buildconfigs to kick
  (cewong@redhat.com)
- Lazily initialize Osin client for token endpoint (mkhan@redhat.com)
- adding --source-secret flag to new-app and new-build (cdaley@redhat.com)
- add network apiserver (deads@redhat.com)
- Fix end-to-end tests for Docker 17.x (obulatov@redhat.com)
- use stock requestinfo and authorization filters (deads@redhat.com)
- UPSTREAM: 51932: fix format of forbidden messages (deads@redhat.com)
- UPSTREAM: 51803: make url parsing in apiserver configurable
  (deads@redhat.com)
- apps: use generate clientset in controllers and api server
  (mfojtik@redhat.com)
- Update auto-generated files. (vsemushi@redhat.com)
- SecurityContextConstraints: add AllowedFlexVolumes field.
  (vsemushi@redhat.com)
- regenerate fake deploymentconfig (mfojtik@redhat.com)
- UPSTREAM: <drop>: Fix result type in fake clientset generator
  (mfojtik@redhat.com)
- Add a new changelog generator (ccoleman@redhat.com)
- perform tsb registration via a template (bparees@redhat.com)
- Add node-exporter example to prometheus (ccoleman@redhat.com)
- cmd test flake: 0-length response instead of expected timeout
  (bruno@abstractj.org)
- UPSTREAM: 51727: ensure all unstructured resources (jvallejo@redhat.com)
- Add short ttl cache to token authenticator on success (jliggitt@redhat.com)
- UPSTREAM: 50258: Simplify bearer token auth chain, cache successful
  authentications (jliggitt@redhat.com)
- UPSTREAM: 50258: Add union token authenticator (jliggitt@redhat.com)
- UPSTREAM: 50258: Add token cache component (jliggitt@redhat.com)
- UPSTREAM: 50258: Add token group adder component (jliggitt@redhat.com)
- WIP (simo@redhat.com)
- Remove usless test and resolved comments (simo@redhat.com)
- Make SCC with less capabilities more restrictive. (jpazdziora@redhat.com)
- Make space in the point logic for capabilities accounting.
  (jpazdziora@redhat.com)
- UPSTREAM: 49475: Fixed glusterfs mount options (jsafrane@redhat.com)
- UPSTREAM: 48940: support fc volume attach and detach (hchen@redhat.com)
- UPSTREAM: 49127: Make definite mount timeout for glusterfs volume mount
  (jsafrane@redhat.com)
- UPSTREAM: 48709: glusterfs: retry without auto_unmount only when it's not
  supported (jsafrane@redhat.com)

* Tue Sep 05 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.125.0
- 

* Tue Sep 05 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.124.0
- Remove build docker volumes if OS_BUILD_ENV_CLEAN_BUILD_VOLUME is set.
  (jchaloup@redhat.com)
- bump(github.com/openshift/origin-web-console):
  59d0df1a953242fbc846a2ca7c8b588e62d437d5 (eparis+openshiftbot@redhat.com)
- Allow openshift start master to work on non-linux platforms
  (jliggitt@redhat.com)
- use upstream authentication filter (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4be379341fba646c034a4ce08b5cc0c75123e7a4 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 51148: Enable finalizers independent of GC enablement
  (deads@redhat.com)
- generated (deads@redhat.com)
- UPSTREAM: 51636: add reconcile command to kubectl auth (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  74e3a084c5287100ccabcb9a28c4a40252b262f1 (eparis+openshiftbot@redhat.com)
- Use forwarded Host header without any changes (obulatov@redhat.com)
- bump(k8s.io/kubernetes): 80709908fd80e48ea9a619e23892747856969487
  (deads@redhat.com)
- use unique filenames for junit output (bparees@redhat.com)
- The Docker-Distribution-API-Version header is optional (ccoleman@redhat.com)
- Use contemporary Bash helpers in hack/verify-generated-swagger-spec.sh
  (skuznets@redhat.com)
- stop overwriting the HPA controller and just write our own (deads@redhat.com)
- set junit output dir to its own dir (bparees@redhat.com)
- include token in tsb registration (bparees@redhat.com)
- allow any 3.7 to have the TSB (deads@redhat.com)
- UPSTREAM: 51705: Address panic in TestCancelAndReadd (mkhan@redhat.com)
- Adding --incremental and --no-cache to start-build (cdaley@redhat.com)
- prevent references from openshift master to other binaries (deads@redhat.com)
- Revert "skip build tests on GCE because new images are needed there"
  (bparees@redhat.com)
- image: add image stream secrets client (mfojtik@redhat.com)
- apps: add Instantiate, GetScale and UpdateScale client method for deployment
  config (mfojtik@redhat.com)
- UPSTREAM: 51638: allow to generate extended methods in client-go
  (mfojtik@redhat.com)
- Regenerate files (danw@redhat.com)
- Rename pkg/sdn to pkg/network, for consistency with its API (danw@redhat.com)
- ensure new endpoint is registered before testing it (bparees@redhat.com)
- default to legacy decoder; fallback to universal / unstructured add dynamic
  mapper for unstructured objects (jvallejo@redhat.com)
- Revert "skip tsb tests on GCE until new images are published"
  (bparees@redhat.com)
- append user labels when --show-labels given (jvallejo@redhat.com)
- Make Deployment's MinReadySeconds test more tollerant to infra
  (tnozicka@gmail.com)
- Add debugging info to Deployment's extended tests (tnozicka@gmail.com)
- build controller: use client lister to get builds for policy
  (cewong@redhat.com)
- upping loglevel for warning from stage and step info (cdaley@redhat.com)
- Added networking members to the images OWNERS (bbennett@redhat.com)
- cleanup some legacy client usage (deads@redhat.com)
- add test case for private image source inputs (bparees@redhat.com)
- UPSTREAM: 51473: Fix cAdvisor prometheus metrics (ccoleman@redhat.com)
- hack/env: remove tmp volume if not user specified (jdetiber@redhat.com)
- move the TSB templates to an install location (deads@redhat.com)
- Fix go vet errors (mkhan@redhat.com)
- Wait longer for healthz during integration tests (mkhan@redhat.com)
- run controller by wiring up to a command (deads@redhat.com)
- UPSTREAM: 51535: allow disabling the scheduler port (deads@redhat.com)
- UPSTREAM: 51534: update scheduler to return structured errors instead of
  process exit (deads@redhat.com)
- Prevent oauth-proxy from listening on http port in prometheus deployment
  (zgalor@redhat.com)
- don't require template name/namespace to be set on nested template within
  templateinstance (jminter@redhat.com)
- add build prometheus metrics (gmontero@redhat.com)
- Remove EPEL from our base images (skuznets@redhat.com)
- Validate pod's volumes only once and also fix field path in the error
  message. (vsemushi@redhat.com)
- Remove getAPIGroupLegacy() from scope conversion (mrogers@redhat.com)
- Update completions (mrogers@redhat.com)
- Add --rolebinding-name to policy commands (mrogers@redhat.com)
- Fix imports (obulatov@redhat.com)
- UPSTREAM: docker/distribution: 2299: Fix signalling Wait in regulator.enter
  (obulatov@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: <carry>: add README.md to
  docker/distribution/vendor (obulatov@redhat.com)
- bump(github.com/docker/distribution):
  48294d928ced5dd9b378f7fd7c6f5da3ff3f2c89 (obulatov@redhat.com)

* Wed Aug 30 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.123.0
- bump(github.com/libopenstorage/openstorage):
  a53f5e5662367da02b95470980c5dbaadfe96c99 (aditya@portworx.com)
- UPSTREAM: revert: dcb5eef2d8a6d14816d8d1f767f0d0016b84dcdf: "UPSTREAM:
  <drop>: hack out portworx to avoid double proto registration"
  (aditya@portworx.com)
- Enable Portworx Volumes. (aditya@portworx.com)
- Split up SDN master/node/proxy/CNI code (danw@redhat.com)
- Split master/node code from subnets.go into separate files (danw@redhat.com)
- Split out pkg/sdn/common with code shared between node/master
  (danw@redhat.com)
- respect request context deadline when iterating on build logs
  (bparees@redhat.com)

* Wed Aug 30 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.122.0
- 

* Wed Aug 30 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.121.0
- Handle final image (ccoleman@redhat.com)
- Break release into pieces, build images in parallel (ccoleman@redhat.com)
- add tags for client generation (deads@redhat.com)
- generated code (deads@redhat.com)
- Added test cases for router FilterNamespaces() (rpenta@redhat.com)
- Remove endpoints and route key format assumptions in the template plugin and
  router code (rpenta@redhat.com)
- Fix filter namespaces in template router (rpenta@redhat.com)
- Remove unnecessary requires from dependent packages (ccoleman@redhat.com)
- bump integration test TO to 45 min per recent results (gmontero@redhat.com)
- Remove unnecessary imageStreamLister interface (maszulik@redhat.com)
- setup docker pull secrets in image extraction init container
  (bparees@redhat.com)
- Bump integration test loglevel down to 4 (ccoleman@redhat.com)
- registry: report publicDockerImageRepository to image stream if configured
  (mfojtik@redhat.com)

* Tue Aug 29 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.120.0
- bump(github.com/openshift/origin-web-console):
  04d694f69cc8b78b54a52e8025fba93df5a71280 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  72f8452a5382842c1b14b1a80ef391a2df7df502 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9e9fa0f60b1e399f2da431281d1ec9050759cad3 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  89e0cd3183a4c8cbe941371d22d375dd1666ded1 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d105f641f57b67ce7b09a44bc303c599f5a6909d (eparis+openshiftbot@redhat.com)
- move build storage where it is owned (deads@redhat.com)
- resolve `groups "impersonategroup" already exists` error (jminter@redhat.com)

* Tue Aug 29 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.119.0
- bump(github.com/openshift/origin-web-console):
  a1ccb4527e0f6c29c27e30eece813dc1c7e44eae (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  40158bbbcd2573dc386c82c02d9c81ca12871717 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c6ac85fec6ebff414312f658f467833ee09d3b8b (eparis+openshiftbot@redhat.com)
- move apps storage to the apps group (deads@redhat.com)
- UPSTREAM: Revert "UPSTREAM: <drop>: keep old pod available"
  (tnozicka@gmail.com)
- Fix DC's MinReadySeconds test (tnozicka@gmail.com)
- Fix MinReadySeconds for DeploymentConfigs (tnozicka@gmail.com)

* Mon Aug 28 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.118.0
- use oauth client instead of registry (deads@redhat.com)
- move all of templateservicebroker under a single package (deads@redhat.com)
- remove more special cases from the controller init path (deads@redhat.com)
- registry: switch registry to use external clientset (mfojtik@redhat.com)
- Use groups/extra information for authorization in template service broker
  (gmontero@redhat.com)
- Capture the time for build-rpm-release (ccoleman@redhat.com)
- Collect controller metrics on the masters (ccoleman@redhat.com)
- update extended test README (jminter@redhat.com)
- refactor extended test TSB startup (jminter@redhat.com)
- additional template instance controller readiness checking testing
  (jminter@redhat.com)
- don't use persistent templates for templateservicebroker (jminter@redhat.com)
- add all template tests to Conformance (jminter@redhat.com)
- Mark `oc deploy` deprecated and hide from help (ccoleman@redhat.com)
- code cleanup from code review/walkthrough (bparees@redhat.com)
- Refactor template controller to follow the same setup patterns
  (maszulik@redhat.com)
- Switch imagecontrollers to use clientsets (maszulik@redhat.com)
- add legacy field selector for names (deads@redhat.com)
- Remove release image definitions from this repo (skuznets@redhat.com)
- Generate clients for ImageStreamImport (maszulik@redhat.com)
- registry: use k8s authorization (mfojtik@redhat.com)
- image: provide versioned helpers (mfojtik@redhat.com)
- registry: add client based in external clientset (mfojtik@redhat.com)
- bump next build timeout further (bparees@redhat.com)
- End-to-end router tests are just unit tests (ccoleman@redhat.com)
- Additional debugging for project test flake (ccoleman@redhat.com)
- Wait longer in TestGCDefaults for heavily loaded servers
  (ccoleman@redhat.com)
- mktemp is not LCD bash (ccoleman@redhat.com)
- When starting server in integration tests, assign random ports
  (ccoleman@redhat.com)
- It should be possible to customize the Kubelet port (ccoleman@redhat.com)
- Allow unix domain sockets to be passed to etcd start (ccoleman@redhat.com)
- Test runner for parallel integration (ccoleman@redhat.com)
- provide a tag when pulling images so we only pull a single image
  (bparees@redhat.com)
- add generated clients (deads@redhat.com)
- add missing oauth client tags (deads@redhat.com)
- remove dead oauth code (deads@redhat.com)
- allow secrets with "." characters to be used in builds (jminter@redhat.com)
- don't alias Env slices in build controller strategy (jminter@redhat.com)
- stop special casing the pv controller in controller init (deads@redhat.com)
- Make service e2e tests retry to avoid flakes (danw@redhat.com)
- UPSTREAM: <carry>: allow PV controller recycler template override
  (deads@redhat.com)
- GetBootstrapSecurityContextConstraints: change return type to a slice of
  pointers. (vsemushi@redhat.com)
- Add a router cmd test to exercise ignoreError() locally (mrogers@redhat.com)
- remove deploymentconfig registry (deads@redhat.com)
- Merge pkg/sdn/plugin/pod_linux.go into pod.go, drop pod_unsupported.go
  (danw@redhat.com)
- Don't build pkg/sdn/plugin on non-Linux (danw@redhat.com)
- Deployment extended test can fail due to cancel (ccoleman@redhat.com)
- Move OpenShift-internal SDN APIs out of pkg/sdn/apis/network/
  (danw@redhat.com)
- Update completions and docs (ffranz@redhat.com)
- More oc plugin tests (ffranz@redhat.com)
- Enable plugins in oc (ffranz@redhat.com)
- UPSTREAM: 47267: flag support in kubectl plugins (ffranz@redhat.com)

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.117.0
- Treat the missing binary message as an info not warning (skuznets@redhat.com)
- default oc set image to --source=docker (bparees@redhat.com)

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.116.0
- 

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.115.0
- 

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.114.0
- bump(github.com/openshift/origin-web-console):
  cf0278b99ca052da01fc8161e3c890712a2552d6 (eparis+openshiftbot@redhat.com)

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.113.0
- remove rhel image testing, ci.dev images are not being well maintained
  (bparees@redhat.com)
- remove use of closure from imageecosystem (bparees@redhat.com)

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.112.0
- remove dead code (bparees@redhat.com)

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.111.0
- Set default for DeploymentConfigSpec.RevisionHistoryLimit in apps/v1 to 10.
  Fix tests now that we are really not doing cascade delete for legacy API.
  (Brings group API into tests.) (tnozicka@gmail.com)

* Fri Aug 25 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.110.0
- remove broken tests from conformance (bparees@redhat.com)
- Image policy is resolving images on replica sets by default
  (ccoleman@redhat.com)
- Consider openshift/origin-base a part of a release for pushing
  (skuznets@redhat.com)
- Ensure AddPostStartHook succeeds (simo@redhat.com)
- UPSTREAM: 51208: Add an OrDie version for AddPostStartHook (simo@redhat.com)
- fix bad scope nil deref in endpoints update (sjenning@redhat.com)
- Remove redundant SA check in router cmd (mrogers@redhat.com)
- HybridProxy: Deal with removed service ObjectRef (sross@redhat.com)
- Rebase hybrid proxy onto event-driven proxy code (sross@redhat.com)

* Thu Aug 24 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.109.0
- 

* Thu Aug 24 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.108.0
- 

* Thu Aug 24 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.107.0
- 

* Thu Aug 24 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.106.0
- move rbac field selectors to last point of use (deads@redhat.com)
- simple legacy rbac moves (deads@redhat.com)
- remove extraneous field selector conversions (deads@redhat.com)
- Add Back node-bootstrapper Service Account creation (simo@redhat.com)
- Clarify router cmd ignoreError comments. (mrogers@redhat.com)
- Remove Resource normalization (simo@redhat.com)
- Stop using NormalizeResources in rulevalidation (simo@redhat.com)
- UPSTREAM: 51197: provide a default field selector for name and namespace
  (deads@redhat.com)
- generate API docs using new infrastructure (jminter@redhat.com)
- replace API doc generation infrastructure (jminter@redhat.com)
- securitycontextconstraints_test.go: use the existing method instead of own
  version of it. (vsemushi@redhat.com)
- UPSTREAM: 51144: Fix unready endpoints bug introduced in #50934
  (joesmith@redhat.com)
- Retry longer for metrics to become available (ccoleman@redhat.com)
- Update stale comment: We use colon instead of underscore between namespace
  and route name for uniquely identifying the route in the router
  (rpenta@redhat.com)
- UPSTREAM: 50934: Skip non-update endpoint updates (sjenning@redhat.com)
- Router tests should not panic when router has no endpoints
  (ccoleman@redhat.com)

* Wed Aug 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.105.0
- bump(github.com/openshift/origin-web-console):
  c9a5d1ae122b158d0cf5ade72e0d907cf77fbdc2 (eparis+openshiftbot@redhat.com)
- add logging information to debug failing port-forward (deads@redhat.com)
- Correct splitting for release candidates (jupierce@redhat.com)
- generated (deads@redhat.com)
- Adjust NetworkPolicy OVS flows for compatibility with (as-yet-unreleased) OVS
  2.8 (danw@redhat.com)
- create TSB config types (deads@redhat.com)
- Stop using NormalizeResources in bootstrap policy (simo@redhat.com)
- run focused tests in parallel where possible (bparees@redhat.com)
- Retry image stream updates when pruning images (maszulik@redhat.com)
- Router template optimization (yhlou@travelsky.com)
- oc cluster up: pass server loglevel to template service broker
  (jminter@redhat.com)
- fix templateinstance timeout logic (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5972dde58479ce9905ec35f243d3db576b831bb3 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bb4cdb5709f557e69b173d29c042ff66da1bce4c (eparis+openshiftbot@redhat.com)
- UPSTREAM: google/cadvisor: 1722: Skip subcontainer update on v2 calls
  (sjenning@redhat.com)
- add tests (jvallejo@redhat.com)
- fix newapp resource convert for extensions and apps groups
  (jvallejo@redhat.com)
- Wait longer for kubelet registration in SDN node startup (danw@redhat.com)
- Update origin spec, update golang requirement and add buildrequires for
  goversioninfo (jdetiber@redhat.com)
- Update release images, use binaries from CentOS PaaS SIG multiarch build repo
  where possible (jdetiber@redhat.com)
- Use centos-paas7-multiarch-el7-build instead of centos-paas-sig-openshift-
  origin36 repo for source image (jdetiber@redhat.com)
- add missing template.alpha.openshift.io/wait-for-ready annotations to
  examples (jminter@redhat.com)
- set openshift default metrics opts (jvallejo@redhat.com)
- UPSTREAM: 50620: allow default option values - oc adm top node|pod
  (jvallejo@redhat.com)
- Use oc adm instead of oadm which might not exist in various installations.
  (jpazdziora@redhat.com)

* Sun Aug 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.104.0
- 

* Sat Aug 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.103.0
- move old direct etcd methods for policy closer to source (deads@redhat.com)
- remove unnecessary fields (deads@redhat.com)
- Use RBAC serialization in bootstrap policy tests (mkhan@redhat.com)
- fix serial suite name reference (bparees@redhat.com)
- Mount /dev in system container (magnus.bengtsson@expressen.se)
- Make sdn pod teardown robust (rpenta@redhat.com)

* Fri Aug 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.102.0
- 

* Fri Aug 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.101.0
- Revert "Merge pull request #15845 from deads2k/server-32-scrub-kuber"
  (skuznets@redhat.com)
- Add enj to pkg/cmd/OWNERS (mkhan@redhat.com)

* Fri Aug 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.7.0-0.100.0
- Bumping spec beyond test builds (jupierce@redhat.com)
- UPSTREAM: 50911: add diff details to pod validation error (deads@redhat.com)
- remove unnecessary fields (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2ecf7441edb33017fb5e7328d8c1aaee9fb0ac88 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f89e0186fe542f7e2aae7cdc9c72d25a03508e1b (eparis+openshiftbot@redhat.com)
- UPSTREAM: revert: c502d10: <carry>: match kube rbac setup in e2es with
  openshift kinds (deads@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 8f07b7b..7e650e7 (jpeeler@redhat.com)
- scrub controller methods (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  05eb65d46758ce8bd25c2b01384d89ac9f660dd0 (eparis+openshiftbot@redhat.com)
- hack/lib: dedup os::util::host_platform and os::build::host_platform
  (jdetiber@redhat.com)
- hack/env: mount a volume for /tmp (jdetiber@redhat.com)
- switch user group to a separate apiserver (deads@redhat.com)
- Enable Scaleio volume (hchen@redhat.com)
- Fix the confusing orders. (zhang.wanmin@zte.com.cn)
- Handle reconciliation annotation during conversion (simo@redhat.com)
- (re)generated stuff (simo@redhat.com)
- test/cmd fixes (mkhan@redhat.com)
- flakes fixes (simo@redhat.com)
- Update admission to use moved GC helper (mkhan@redhat.com)
- UPSTREAM: 49902: Allow update to GC fields for RBAC resources
  (mkhan@redhat.com)
- Change authorizer to use Kubernetes facilities (simo@redhat.com)
- UPSTREAM: 50710: Refactor RBAC authorizer entry points (mkhan@redhat.com)
- Version gate legacy oc commands to < 3.7 (simo@redhat.com)
- Bootstrap Origin policies in post start hook (mkhan@redhat.com)
- UPSTREAM: 50702: Allow injection of policy in RBAC post start hook
  (mkhan@redhat.com)
- Use dynamic error wrapper on proxied endpoints (mkhan@redhat.com)
- Proxy {Cluster}Role{Binding}s to Native Kube RBAC (simo@redhat.com)
- UPSTREAM: 50639:  Extend SetHeader Requests method ito accept multiple values
  (simo@redhat.com)
- Cleanup: Move conversion function (simo@redhat.com)
- Cleanup: Remove custom code and use available utility code (simo@redhat.com)
- Cleanup: Check for error conditions in aggregator (mkhan@redhat.com)
- UPSTREAM: 48480: Ensure namespace exists as part of RBAC reconciliation
  (simo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5671a61789204bd958f424dbe0991cbfe4386110 (eparis+openshiftbot@redhat.com)
- UPSTREAM: fatih/structs: <carry>: add fatih/structs to vendor
  (sejug@redhat.com)
- Working default config (sejug@redhat.com)
- Basic clusterloader functionality (sejug@redhat.com)
- bump(github.com/openshift/origin-web-console):
  82888c43251569427f96babc612700e2e2c8e36b (eparis+openshiftbot@redhat.com)
- update packaged templates (jminter@redhat.com)
- implement template completion detection (jminter@redhat.com)
- populate Status.Objects in templateInstance (jminter@redhat.com)
- switch webhook to use clients (deads@redhat.com)
- stop running the TSB in the main apiserver (deads@redhat.com)
- move the oauth server out of the main API server for structure
  (deads@redhat.com)
- add 2 new queries (jeder@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bf2a6f22c242dff089af5f35ec599f5f0e515e27 (eparis+openshiftbot@redhat.com)
- fix swagger version reporting (jminter@redhat.com)
- catalog: adjust RBAC permissions and controller parms (jpeeler@redhat.com)
- make the build generator rely on clients, not registries (deads@redhat.com)
- Fix alerts proxy configuration to include proper SAR and oauth
  (ccoleman@redhat.com)
- cesar2 (bparees@redhat.com)
- UPSTREAM: <drop>: skip TSB namespace when checking for scheduled pods
  (deads@redhat.com)
- Don't use all caps for consts. This isn't C. (danw@redhat.com)
- switch user registries to loopback clients (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e89bb67e7f2069bbdd85cacf54d9ffe755f567b2 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6cd2686f5a0d798b8a2aea0d5744a55d2bc2da64 (eparis+openshiftbot@redhat.com)
- Check for golang 1.8. (jpazdziora@redhat.com)
- gabe3 (bparees@redhat.com)
- image-pruner: Reenable registry-url validation (miminar@redhat.com)
- image-pruner: Determine protocol just once (miminar@redhat.com)
- remove --kubeconfig when start_master or start_allinonwq (123456a?)
- extended: Skip test instead of failing (miminar@redhat.com)
- create a template group apiserver (deads@redhat.com)
- Extend prometheus template with alertmanager and prometheus-alert-buffer
  (zgalor@redhat.com)
- fix git source secret handling (bparees@redhat.com)
- generated code (deads@redhat.com)
- skip tsb tests on GCE until new images are published (deads@redhat.com)
- run TSB test as part of the normal conformance bucket (deads@redhat.com)
- deeper specification of authentication chain in TSB (deads@redhat.com)
- add clients (deads@redhat.com)
- fixing race condition in build pruning test (cdaley@redhat.com)
- tighten interfaces for template instance API (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  56c925a9d562fef151e7438fb8ca07f8aacfb395 (eparis+openshiftbot@redhat.com)
- enforce package import restrictions on TSB (deads@redhat.com)
- remove unnecessary method (deads@redhat.com)
- generate more image clients and fix users clientset (mfojtik@redhat.com)
- UPSTREAM: 50139: skip generation of informers and listers on resources with
  missing verbs (mfojtik@redhat.com)
- make unit test list comparison order-independent (bparees@redhat.com)
- handle streaming logs for containers that ultimately fail
  (bparees@redhat.com)
- handle missing build contextdir and fetch source errors (bparees@redhat.com)
- Correctly propogate error code from image builds up (skuznets@redhat.com)
- openshift always serves securely, so disallow non-tls serving info
  (deads@redhat.com)
- Properly handle errors in policy listing (simo@redhat.com)
- gabe2 (bparees@redhat.com)
- setup secrets properly (bparees@redhat.com)
- separate the asset server from the rest of the servers (deads@redhat.com)
- gabe1 (bparees@redhat.com)
- cesar1 (bparees@redhat.com)
- cleanup refactors (bparees@redhat.com)
- replace build context setup with init containers (bparees@redhat.com)
- skip build tests on GCE because new images are needed there
  (bparees@redhat.com)
- add db name to the provided secret (bparees@redhat.com)
- Update etcd stores to use DefaultQualifiedResource (mkhan@redhat.com)
- UPSTREAM: 49868: Status objects for 404 API errors will have the correct
  APIVersion (mkhan@redhat.com)
- disable TSB client cert and front proxy auth until aggregation is on by
  default (deads@redhat.com)
- bump(github.com/openshift/source-to-image):
  06c9446cd6b580cbcb13a1489efcb3e943b470af (bparees@redhat.com)
- update the import verifier to work better for the API checking use-case
  (deads@redhat.com)
- More queries for prometheus (ccoleman@redhat.com)
- catalog: remove options no longer supported (jpeeler@redhat.com)
- stop skipping build valuefrom test on gce (bparees@redhat.com)
- Change to include -RELEASE in OS_GIT_VERSION (jupierce@redhat.com)
- Bumping golang version requirement (jupierce@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from 568a7b9..8f07b7b (jpeeler@redhat.com)
- update node_config test to be arch agnostic (jdetiber@redhat.com)
- start the rachet on our API types (deads@redhat.com)
- delete dead code and move non-generic utilities from shared shared package
  (deads@redhat.com)
- Update local SAR to scope to a namespace correctly (ccoleman@redhat.com)
- add pweil- to owners (pweil@redhat.com)
- bump(github.com/openshift/origin-web-console):
  215a1b55761a1de74546d9b3549900b913f101cb (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e5867ae3f6c0e97eb7869ec0e36b64d37e8d52cd (eparis+openshiftbot@redhat.com)
- UPSTREAM: <drop>: regenerated openapi (jdetiber@redhat.com)
- remove old style group cache (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  45dd1acb1efd57876654b0772c1f7f5d63d3698f (eparis+openshiftbot@redhat.com)
- provider_test.go: use existing method instead of own copy of it.
  (vsemushi@redhat.com)
- Merge scc_validation_test.go into validation_test.go (vsemushi@redhat.com)
- Mark subjectaccessreview/resourceaccessreview as root-scoped
  (jliggitt@redhat.com)
- SDN test should wait longer for namespaces (ccoleman@redhat.com)
- make cluster-up work with separate TSB (deads@redhat.com)
- Add --scheduled to import-image (elyscape@gmail.com)
- enable the TSB storage and controller by default (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bba7c7f311efe182cbf2ac736cb8480244826d9d (eparis+openshiftbot@redhat.com)
- bump(github.com/googleapis/gnostic):0c5108395e2debce0d731cf0287ddf7242066aba
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  419c1e6fb6f7a9a5a39fd7fa571c8fc297720ab2 (eparis+openshiftbot@redhat.com)
- simplify tsb flags (deads@redhat.com)
- make tsb run in separate server (deads@redhat.com)
- Remove no-longer-used openshift-sdn-ovs script (danw@redhat.com)
- update the TSB role to have required permissions (deads@redhat.com)
- add template for TSB (deads@redhat.com)
- Registry owners extended (miminar@redhat.com)
- hack: tolerate badly gofmt'd files appear anywhere (miminar@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8cb608bd5ec24a3225f81657aff08955718142b2 (eparis+openshiftbot@redhat.com)
- make wiring more obvious in TSB (deads@redhat.com)
- UPSTREAM: 49972: remove dead log handler and increase verbosity
  (deads@redhat.com)
- UPSTREAM: revert: ffead55: <carry>: Fix to avoid REST API calls at log level
  2. (deads@redhat.com)
- start a command for running the template service broker (deads@redhat.com)
- print healthz on failures (deads@redhat.com)
- UPSTREAM: 50259: provide the failing health as part of the controller error
  (deads@redhat.com)
- handle bootstrap openshift namespace roles (deads@redhat.com)
- fixups after bumping imagebuilder (jminter@redhat.com)
- Switch to the advanced audit backend (maszulik@redhat.com)
- bump(github.com/openshift/imagebuilder):
  c3e2e96f351aa1b355fb90169ec9390b7eff5fc5 (jminter@redhat.com)
- respect the reconcile protect annotation on bindings (deads@redhat.com)
- unconditionally reconcile cluster roles (deads@redhat.com)
- Add limit for number of concurrent connections to registry
  (obulatov@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8b2ea907b8a9153fdf192343590c9b13a88d4ea6 (eparis+openshiftbot@redhat.com)
- start trying to fix asset server (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7a25f13bfbf365be965426b0472391d8fd92f585 (eparis+openshiftbot@redhat.com)
- Adding new tito releaser for 3.7 (jupierce@redhat.com)
- Generated changes for public pull spec in image stream status
  (maszulik@redhat.com)
- Add public pull spec field to image stream status (maszulik@redhat.com)
- Test case for mixed/uppercase host name (pcameron@redhat.com)
- Remove oatmealraisin from OWNERS (mkhan@redhat.com)
- Setting as 3.7 pre-release (jupierce@redhat.com)
- add test for `oc get all` (jvallejo@redhat.com)
- Fix clientset generation script (cewong@redhat.com)
- Add myself as an OWNER under the root (skuznets@redhat.com)
- Properly escaping the arguments supplied to rsync -e (cdaley@redhat.com)
- Add an import verification tool (skuznets@redhat.com)
- Update generated files. (vsemushi@redhat.com)
- SecurityContextConstraints: update description of the Priority field.
  (vsemushi@redhat.com)
- switch to post-starthooks (deads@redhat.com)
- Remove ncdc from OWNERS (mkhan@redhat.com)
- SDN should not set net.ipv4.ip_forward (pcameron@redhat.com)
- UPSTREAM: 49992: Correctly handle empty watch event cache
  (jliggitt@redhat.com)
- create template-service-broker SA during API server startup
  (jminter@redhat.com)
- OWNERS for CLI completions (ffranz@redhat.com)
- don't special case / in authorizer (deads@redhat.com)
- remove dead listers (deads@redhat.com)
- remove unnecessary attribute functions (deads@redhat.com)
- remove dead listers (deads@redhat.com)
- UPSTREAM: 50019: create default selection functions for storage
  (deads@redhat.com)
- replace use of resource#NewBuilder with factory#NewBuilder
  (jvallejo@redhat.com)
- issue-13136 cluster up allows any docker registry CIDRs that are within
  172.30.0.0/16 range (bornemannjs@gmail.com)
- UPSTREAM: 49919: Fix duplicate metrics collector registration attempt error
  (deads@redhat.com)
- Test NetworkPolicy plugin (if the kernel supports it) (danw@redhat.com)
- Update NetworkPolicy test from upstream with v1 semantics (danw@redhat.com)
- Re-remove old extended networking OVS test (danw@redhat.com)
- move controller serving to controller package (deads@redhat.com)
- cleanup server wiring using "with" (deads@redhat.com)
- UPSTREAM: <drop>: regenerate clientsets using updated codegen
  (mfojtik@redhat.com)
- deploy: remove deprecated generatedeploymentconfig api (mfojtik@redhat.com)
- Revert "Owners for CLI completions" (bparees@users.noreply.github.com)
- Retry build status update on non-conflict error (cdaley@redhat.com)
- Owners for CLI completions (ffranz@redhat.com)
- remove downstream shortcutexpander (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3130e603143f791d897ea8e79127db03005bdaa6 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  48b661c0ebaf0ae1700e9662a43e99715abe5d8a (eparis+openshiftbot@redhat.com)
- sdn: move sandbox kill-on-failed-update to runtime socket from direct docker
  (dcbw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  12f4910ce25499fc98e9442e1e195b7d4a45c4d9 (eparis+openshiftbot@redhat.com)
- Make releasing slightly easier for retries (ccoleman@redhat.com)
- clean up cluster up loglevel 1 (bparees@redhat.com)
- Enable IngressConfiguredRouter test (skuznets@redhat.com)
- separate kube and openshift (deads@redhat.com)
- UPSTREAM: <drop>: aggregate openapi through servers.  1.8 should fix this for
  CRD (deads@redhat.com)
- UPSTREAM: <drop>: don't hold API server start on superslow openapi
  aggregation. Should be baster in 1.8 (deads@redhat.com)
- UPSTREAM: <drop>: allow duplicate openapi paths.  this is moved in 1.8
  (deads@redhat.com)
- UPSTREAM: 00000: allow nil openapispec (deads@redhat.com)
- Fix extended tests: Docker 17.03 is newer than 1.9 (obulatov@redhat.com)
- registry: add Oleg and Michal as approvers for registry (mfojtik@redhat.com)
- move templateservicebroker to its own server (deads@redhat.com)
- UPSTREAM: emicklei/go-restful-swagger12: <carry>: shim to allow multiple
  containers to union for swagger (deads@redhat.com)
- image: match the integrated registry when using image name in image policy
  (mfojtik@redhat.com)
- Extended test for registry garbage collector (miminar@redhat.com)
- Add -prune option to dockerregistry (obulatov@redhat.com)
- bump(github.com/openshift/origin-web-console):
  084ad24d44ca6963963e8d044c6cf7f265ac7544 (eparis+openshiftbot@redhat.com)
- UPSTREAM: revert: fbf1f04: <drop>: disable openapi aggregation"
  (deads@redhat.com)
- UPSTREAM: revert: 7fc9d38a7a: <drop>: drop post 3.7 rebase.  allows disabled
  aggregator (deads@redhat.com)
- Remove extensions.jobs (maszulik@redhat.com)
- remove mfojtik from bunch of OWNERs (mfojtik@redhat.com)
- fix command name is kubectl (shiywang@redhat.com)
- Enable configmap leader election and make default (ccoleman@redhat.com)
- Convert tests to etcd3 where necessary (ccoleman@redhat.com)
- Make A/B deployment proportional to service weight (pcameron@redhat.com)
- oc sibling commands (example rsync) fail if there is a space in the path to
  the oc command (cdaley@redhat.com)
- patch SC roles (bparees@redhat.com)
- Switch back to using master etcd in integration (ccoleman@redhat.com)
- Refactor etcd logic so we can run the master etcd (ccoleman@redhat.com)
- When running an all-in-one or master integration test, use etcd3
  (ccoleman@redhat.com)
- Prevent accidentally starting the master twice in the same process
  (ccoleman@redhat.com)
- Increase the range of ports for parallel integration (ccoleman@redhat.com)
- Support unix domain sockets in etcd startup for master (ccoleman@redhat.com)
- Refactor etcd startup to allow for better embedded use (ccoleman@redhat.com)
- Integration tests should start etcd exactly once per test
  (ccoleman@redhat.com)
- make the router integration tests run as part of test-end-to-end
  (deads@redhat.com)
- cluster up: fix version parsing (jdetiber@redhat.com)
- debug for extended test pv creation (bparees@redhat.com)
- Corrected the regex used for the v4v6 env parsing (bbennett@redhat.com)
- Merge pkg/security/scc into pkg/security/securitycontextconstraints package.
  (vsemushi@redhat.com)
- migration: fixes to manifest migration script (miminar@redhat.com)
- Fix my github username in OWNERS files (danw@redhat.com)
- UPSTREAM: <drop>: update kubernetes types for new codegen syntax
  (mfojtik@redhat.com)
- regenerate openshift clientsets (mfojtik@redhat.com)
- update types to follow new upstream genclient syntax (mfojtik@redhat.com)
- UPSTREAM: <drop>: pickup clientgen changes (mfojtik@redhat.com)
- disable openapi in integration tests because it is slow (deads@redhat.com)
- Add print handlers (bruno@abstractj.org)
- oc policy can-i --list output is not parser friendly (jvallejo@redhat.com)
- UPSTREAM: <drop>: make openapi handler serialize in the background
  (deads@redhat.com)
- make apiserver start less noisy (deads@redhat.com)
- Accidental deletion of core files (ccoleman@redhat.com)
- UPSTREAM: 49448: Use an interface in lock election (ccoleman@redhat.com)
- Changing from no cert to edge encryption should not panic
  (ccoleman@redhat.com)
- Require goversioninfo for Windows cross builds (ccoleman@redhat.com)
- Remove myself from a bunch of OWNERS files (mkargaki@redhat.com)
- Indent egress-router.sh: consistent spacing, convert all tabs to spaces
  (rpenta@redhat.com)
- Add validations to Egress router script (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  dcabd685ca5efa0d076200d87554e6eb91f838e1 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 49724: skip WaitForAttachAndMount for terminated pods in syncPod
  (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1da78a2c19161afe4f26ea6a01ac3a4417fd6832 (eparis+openshiftbot@redhat.com)
- Update list of allowed runAsUser types in the error message.
  (vsemushi@redhat.com)
- CONTRIBUTING.adoc: update minimum Go version. (vsemushi@redhat.com)
- stop aggregating openapi and just delegate to kube (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4bcfb4f8bdf1d58e88fd331426407462f06728f5 (eparis+openshiftbot@redhat.com)
- UPSTREAM: <drop>: disable openapi aggregation (deads@redhat.com)
- godeps: fix invalid import path for kube-gen package (mfojtik@redhat.com)
- Fix panic when tag is nil when creating istag (maszulik@redhat.com)
- Remove duplicate code and reuse templating from upstream (ffranz@redhat.com)
- Truncate rev.Git.Message on the first line. Fixes #13841
  (matthias.bertschy@gmail.com)
- UPSTREAM: <carry>: Do not error out on pods in kube-system
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f62c98fb34e368038a821785ab5850bf219168ec (eparis+openshiftbot@redhat.com)
- Include origin-egress-http-proxy image in the release (rpenta@redhat.com)
- respect --image flag value for service catalog image name
  (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  dca60cac4f15bd4d4cc430d61d534ed2b8224da2 (eparis+openshiftbot@redhat.com)
- retry integration failures to suppress the long tail of flakes
  (deads@redhat.com)
- print typed podlist for correct serialization (jvallejo@redhat.com)
- add proxy transport to aggregator (deads@redhat.com)
- UPSTREAM: 49495: make it possible to allow discovery errors for controllers
  (deads@redhat.com)
- add upstream namespaced rbac resoruces (deads@redhat.com)
- generate aggregator certs (deads@redhat.com)
- remove dead code (deads@redhat.com)
- make each SA client QPS/Burst proportional to the old limit
  (deads@redhat.com)
- regenerate openapi (mfojtik@redhat.com)
- Remove myself from the reviewers list for a bunch of components.
  (vsemushi@redhat.com)
- add type assertions for registries (deads@redhat.com)
- update generators for new import path and use upstream generators for listers
  and informers (mfojtik@redhat.com)
- bump(k8s.io/kube-gen): d2e5420de791a3a5234eb1c3d84921d68ae2b6d6
  (mfojtik@redhat.com)
- UPSTREAM: 49114: Move generators to staging/src/k8s.io/kube-gen
  (mfojtik@redhat.com)
- UPSTREAM: containers/image: <carry>: Do not check lifetime for v3 GPG
  signatures (mfojtik@redhat.com)
- only set the build timestamps exactly once (bparees@redhat.com)
- More CLI OWNERS (ffranz@redhat.com)
- Update OWNERS (decarr@redhat.com)
- Correct the url for the memory stats in the router test (bbennett@redhat.com)
- UPSTREAM: 49656: make admission tolerate object without objectmeta for errors
  (deads@redhat.com)
- Cleanup: SDN node plugin (rpenta@redhat.com)
- Add networking team members to networking-related OWNERS files
  (bbennett@redhat.com)
- bump(github.com/openshift/origin-web-console):
  91f326ceb188ee44d1621e597b6df82a1d7a51ed (eparis+openshiftbot@redhat.com)
- update surrounding files to handle the change (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  441f98f86045d84fb6ad3e980e7240b80737cb85 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  19290dea8360bab9480f3ecb0bfee1684a42336f (eparis+openshiftbot@redhat.com)
- bulk move to start oc package (deads@redhat.com)
- SWEET32 mitigation: Disable Triple-DES (cheimes@redhat.com)
- Add back origin-sdn-ovs (andrew@andrewklau.com)
- Add enj to appropriate OWNERS (mkhan@redhat.com)
- UPSTREAM: kubernetes-incubator/cluster-capacity: <drop>: update OWNERS
  (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2aaa1a7a8d2e616e72e2973fc071d24230510619 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ffb9b2dcfc7abc0b48e428bf63f2ee2ef0c01d5c (eparis+openshiftbot@redhat.com)
- remove oc cluster up dependency on oc binary (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  737c0d107a7e8105e67c683732172ff63c91aea5 (eparis+openshiftbot@redhat.com)
- remove deads2k from some of the more specific packages (deads@redhat.com)
- Don't put OWNERS into bindata (skuznets@redhat.com)
- Properly authorize controller API requests (ccoleman@redhat.com)
- Add Eric Paris to the root-level OWNERS (skuznets@redhat.com)
- Bootstrap OWNERS files for the repository (skuznets@redhat.com)
- sync with admin role for all (deads@redhat.com)
- Remove `subjectaccessreviews.v1.` hacks from `test/cmd/authentication.sh`
  (mkhan@redhat.com)
- UPSTREAM: 48224: add reflector metrics (deads@redhat.com)
- Move CapabilityAll from k8s types and rename it to AllowAllCapabilities.
  (vsemushi@redhat.com)
- deploy: use subPath in hooks for volumes when defined (mfojtik@redhat.com)
- UPSTREAM: revert: 1b2aacbd4010757b6978ddc40bfc8d89495c89dd: <carry>: allow to
  use * as a capability in Security Context Constraints. (vsemushi@redhat.com)
- UPSTREAM: 49118:     Allow unmounting bind-mounted directories.
  (jpazdziora@redhat.com)
- UPSTREAM: 48778: check for negative values (jvallejo@redhat.com)
- Use upstream client bootstrap (ccoleman@redhat.com)
- UPSTREAM: 49514: Make client bootstrap public (ccoleman@redhat.com)
- UPSTREAM: 48518: Kubelet client cert initialization (and 48519)
  (ccoleman@redhat.com)
- Set up caBundle for service catalog in cluster up (jliggitt@redhat.com)
- bump(k8s.io/kubernetes): d2e5420de791a3a5234eb1c3d84921d68ae2b6d6
  (deads@redhat.com)
- UPSTREAM: 49444: Do not spin forever if kubectl drain races with other
  removal (jpeeler@redhat.com)
- Remove &lt; from pre-formatted block in README.md (me@gbraad.nl)
- bump(github.com/openshift/origin-web-console):
  6e99d34ea3898a70b04588b747cf0f75e898332b (eparis+openshiftbot@redhat.com)
- UPSTREAM: 49395: rate limiting should not affect system masters
  (deads@redhat.com)
- switch the SA token authenticator to a client (deads@redhat.com)
- registry: reenable logstash formatter (miminar@redhat.com)
- bump(github.com/bshuster-repo/logrus-logstash-hook):
  0e6d502573042a6563419fbcce4f284600bfe929 (miminar@redhat.com)
- Fix RWO warning with percentages (mkargaki@redhat.com)
- Correct TLS cipher suite comments for HTTP/2 (cheimes@redhat.com)
- Builds should be able to reference image stream tags and images
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  93a4318dd8a0ccb50fabf5426425a8b60aee7b3b (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ab5f706082bccf458217dac15e70740e779f3f5e (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e839e15196adadef3f242fdb51fdde59f8bfd2c3 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 49353: Use specified ServerName in aggregator TLS validation
  (jliggitt@redhat.com)
- Add a conformance test for Prometheus (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3355713b6791c0a96b24449f5f518280d6a8af0c (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c607b86859a1f442e5ffb807cbe87ef3fb2d0049 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 49326: add cronjobs to all (deads@redhat.com)
- UPSTREAM: 49379: Pass clientset's Interface to CreateScheduler.
  (avagarwa@redhat.com)
- Create scheduler from SchedulerServer and Configurator. (avagarwa@redhat.com)
- expose the entire API through the aggregator (deads@redhat.com)
- UPSTREAM: 49312: allow the /version endpoint to pass through
  (deads@redhat.com)
- update build proxy url parsing for go 1.8 (jminter@redhat.com)
- bump(github.com/openshift/source-to-image):
  7da3d3e97565e652a59fd81286f3956fd43e85a8 (jminter@redhat.com)
- Use constants for representation of the SCCs restrictiveness.
  (vsemushi@redhat.com)
- Switch haproxy methods to gauge from counter (ccoleman@redhat.com)
- Mutator for build output and better error handling (ccoleman@redhat.com)
- Update prometheus to capture cadvisor metrics (ccoleman@redhat.com)
- Include cAdvisor path for 1.7 in the scrape targets (ccoleman@redhat.com)
- UPSTREAM: 49079: Restore cadvisor prometheus metrics (ccoleman@redhat.com)
- Disable some very slow serial tests (ccoleman@redhat.com)
- Setting origin.spec to 3.7 (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7b414b3ab02aa8d81c04f33532bdd74a138245df (eparis+openshiftbot@redhat.com)
- cluster up: make socat resilient to errors (cewong@redhat.com)
- Add additional conditions to retry docker image push (cdaley@redhat.com)
- Moved locking to protect a read of a map in the router (bbennett@redhat.com)
- Register SDN informers synchronously (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  620d2d40c1a615635260c76d6a499ab35687314a (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  296c13cff12fcecfc70a14d4ef09efb895d63b93 (eparis+openshiftbot@redhat.com)
- react to quota API change (deads@redhat.com)
- UPSTREAM: 49227: tighten quota controller interface (deads@redhat.com)
- remove old controller SA registration (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6faa37faaa06120adfc7f8364677bdd326a509d8 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 49111: Fix findmnt parsing in containerized kubelet
  (jsafrane@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d22548e76861be775f635cf557b4831aec76266d (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3ba38530b8d26db3b0e4a6fe6a9cd5c6e6178d95 (eparis+openshiftbot@redhat.com)
- remove unnecessary duplicate clusterrole (deads@redhat.com)
- always generate a config which contains aggregator keys for cluster up
  (deads@redhat.com)
- e2e: disable running cli commands in container test because of flakes
  (mfojtik@redhat.com)
- node, syscontainer: export CNI plugins to the host when present
  (gscrivan@redhat.com)
- UPSTREAM: 49285: do not mutate statefulset on update (mfojtik@redhat.com)
- initial build analyzer (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c9b410c06e473500434616cf283e68806edd99cb (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5d483015ffc38926df4ed5ddca772d0733272293 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8ad161e198929596ed9b05efd6b0c191df651876 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cfb6f1331b17a77b6d019a9211939cbcc08d08bd (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ecbb648e9be1051e300464a4c36829059df113f6 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 49230: use informers for quota calculation (decarr@redhat.com)
- bump(github.com/openshift/origin-web-console):
  27f6af5f827dde976ee4c4e5acf75e3c2885c331 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0d9243369dbf71106652544a8947164122263f72 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  977c76ba7273a4a77bb343cd6748b5a3f55bec15 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fcc70ebddff5845d0cb4880d074c1d2a414f7bff (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ae0dc4b3e7173547d3515c02558e29739bd68c0e (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bc20227fa24a97c773a8a197f83dd6c09f8c5211 (eparis+openshiftbot@redhat.com)
- use git-show--s instead of git-show--quiet (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  15c6d1b177d071eb66337449fd50fcdc9051dd41 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4209f2aee9002898cc649c883ce5be185aef9b6f (eparis+openshiftbot@redhat.com)
- UPSTREAM: 47788: Get rid of 30s ResyncPeriod in endpoint controller.
  (avagarwa@redhat.com)
- Update quota controller's role for Kube authorizer (mkhan@redhat.com)
- Fix conversion for role binding to cluster role (mkhan@redhat.com)
- UPSTREAM: 47731: Use endpoints informer for the endpoint controller.
  (avagarwa@redhat.com)
- use openshift service catalog in cluster up (bparees@redhat.com)
- Add an ENV to control ipv6 behavior in the router (bbennett@redhat.com)
- move non-admission plugins out of admission packages (deads@redhat.com)
- remove useless indirection from proxy command (deads@redhat.com)
- UPSTREAM: 48960: No warning event for DNSSearchForming (decarr@redhat.com)
- UPSTREAM: 48635: proxy/userspace: suppress "LoadBalancerRR: Removing
  endpoints" message (deads@redhat.com)
- UPSTREAM: 49120: Modify podpreset lister to use correct namespace
  (jpeeler@redhat.com)
- UPSTREAM: <drop>: Adapt etcd testing util to v3.2.1 (jliggitt@redhat.com)
- resolve image push secret in controller, not instantiate (bparees@redhat.com)
- bump(github.com/coreos/etcd):v3.2.1 (ccoleman@redhat.com)
- Add an --ignore-unknown-parameters flag to "oc process"
  (matthias.bertschy@gmail.com)
- Update all release images to latest status and add Go 1.9beta2
  (ccoleman@redhat.com)
- deploy: verify the deployer SA has perms to update tags (mfojtik@redhat.com)
- fail closed on versioned pods (pweil@redhat.com)
- Mention caling daemon-reload in cluster_up_down.md (tom@tomforb.es)
- Embed expiry in session, fix clock-skew issues in IE (jliggitt@redhat.com)
- cluster up: add insecure-registries in daemon.json (firdaus.ramlan@gmail.com)

* Wed Jul 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.154-1
- disable a few failing e2e tests (deads@redhat.com)
- disable projectrequest unit test because the fake client is weird
  (deads@redhat.com)
- defer adding new oc/kubectl commands until after the rebase
  (deads@redhat.com)
- add FSStorageOS to SCC (deads@redhat.com)
- sdn: additional kube-1.7 rebase fixup (dcbw@redhat.com)
- Fix cross-platform compile of pod manager (jliggitt@redhat.com)
- node default config value changes (deads@redhat.com)
- disable registry url validation in prune (deads@redhat.com)
- disable go version checking (deads@redhat.com)
- boring: generated (deads@redhat.com)
- remove dead volume source migrator (deads@redhat.com)
- storage changes in 3.7 (deads@redhat.com)
- remove old hpa serialization tests (deads@redhat.com)
- new openshift permissions (deads@redhat.com)
- react to upstream flag changes: api-version was removed (deads@redhat.com)
- minor API updates to allow generation (deads@redhat.com)
- rewire openshift start (deads@redhat.com)
- Update default enabled ciphers for go1.8 (jliggitt@redhat.com)
- fix pkg/sdn/plugin unit tests for CNI bump (danw@redhat.com)
- Port SDN proxy filter to new EndpointsHandler (danw@redhat.com)
- wire up new kubeproxy (deads@redhat.com)
- sdn: update for CNI changes (dcbw@redhat.com)
- endpointsconfighandler changes, can't remove from history (deads@redhat.com)
- DISABLE UNIDLER (deads@redhat.com)
- adjust to new systemd library (deads@redhat.com)
- router metrics changed/disabled! (deads@redhat.com)
- directly depend on the scc admission plugin (deads@redhat.com)
- proxy command changed a lot.  This may be a wrapper now (deads@redhat.com)
- React to upstream NewBuilder changes (deads@redhat.com)
- react to upstream command changes in wrappers (deads@redhat.com)
- handle printer changes (deads@redhat.com)
- react to client factory changes in oc/kubectl (deads@redhat.com)
- update admission plugins (deads@redhat.com)
- remove NodeLegacyHostIP per
  https://github.com/kubernetes/kubernetes/pull/44830 (deads@redhat.com)
- handle upstream storage controller changes (deads@redhat.com)
- rewire controllers (deads@redhat.com)
- update defaulting to handle new kube (deads@redhat.com)
- update build for 1.8 (deads@redhat.com)
- update generators to work again (deads@redhat.com)
- boring: fake client update (deads@redhat.com)
- boring: simple method moves (deads@redhat.com)
- UPSTREAM: 48884: Do not mutate pods on update (ccoleman@redhat.com)
- UPSTREAM: 48613: proxy/userspace: honor listen IP address as host IP if given
  (dcbw@redhat.com)
- UPSTREAM: 48733: Never prevent deletion of resources as part of namespace
  lifecycle (jliggitt@redhat.com)
- UPSTREAM: 48578: run must output message on container error
  (ffranz@redhat.com)
- UPSTREAM: 48582: Fixes oc delete ignoring --grace-period. (ffranz@redhat.com)
- UPSTREAM: 48481: protect against nil panic in apply (ffranz@redhat.com)
- UPSTREAM: 49139: expose proxy default (deads@redhat.com)
- UPSTREAM: <drop>: restore normal alias test (deads@redhat.com)
- UPSTREAM: <drop>: disable suspicously failing tests (deads@redhat.com)
- UPSTREAM: <carry>: allow separate mapping fo kind (deads@redhat.com)
- UPSTREAM: 49137: proxier is it really nil (deads@redhat.com)
- UPSTREAM: <carry>: squash to SCC (deads@redhat.com)
- UPSTREAM: <carry>: generator updates (deads@redhat.com)
- UPSTREAM: 45294: Fix protobuf generator for aliases to repeated types
  (deads@redhat.com)
- UPSTREAM: <drop>: hack out portworx to avoid double proto registration
  (deads@redhat.com)
- UPSTREAM: 49136: don't mutate printers after creation (deads@redhat.com)
- UPSTREAM: 49134: tolerate missing template (deads@redhat.com)
- UPSTREAM: 49133: update permissions to allow block owner deletion
  (deads@redhat.com)
- UPSTREAM: <carry>: increase wait in kubecontrollers (deads@redhat.com)
- UPSTREAM: <carry>: compensate for poor printer behavior (deads@redhat.com)
- UPSTREAM: <carry>: make wiring in kubeproxy easy until we sort out config
  (deads@redhat.com)
- UPSTREAM: 49132: make a union categoryexpander (deads@redhat.com)
- UPSTREAM: 49131: expose direct from config new scheduler method
  (deads@redhat.com)
- UPSTREAM: 49130: expose RegisterAllAdmissionPlugins (deads@redhat.com)
- UPSTREAM: <drop>: keep old pod available (deads@redhat.com)
- UPSTREAM: <drop>: regenerated openapi because it's not commited
  (deads@redhat.com)
- UPSTREAM: <carry>: allow to use * as a capability in Security Context
  Constraints. (vsemushi@redhat.com)
- UPSTREAM: 45894: Export BaseControllerRefManager (deads@redhat.com)
- UPSTREAM: coreos/etcd: <carry>: vendor grpc v1.0.4 locally
  (agoldste@redhat.com)
- UPSTREAM: docker/engine-api: 26718: Add Logs to ContainerAttachOptions
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: 2140: Add 'ca-central-1' region for registry
  S3 storage driver (mfojtik@redhat.com)
- UPSTREAM: opencontainers/runc: 1216: Fix thread safety of SelinuxEnabled and
  getSelinuxMountPoint (pmorie@redhat.com)
- UPSTREAM: docker/distribution: 2008: Honor X-Forwarded-Port and Forwarded
  headers (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Update dependencies
  (agladkov@redhat.com)
- UPSTREAM: docker/distribution: 1857: Provide stat descriptor for Create
  method during cross-repo mount (jliggitt@redhat.com)
- UPSTREAM: docker/distribution: 1757: Export storage.CreateOptions in top-
  level package (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- BREAK logstash formatting (deads@redhat.com)
- UPSTREAM: <drop>: generated updates (deads@redhat.com)
- UPSTREAM: <drop>: squash to bump, copy (deads@redhat.com)
- bump(k8s.io/kubernetes): 695b5616baa050ac185abdddb3c750205a08a19b
  (deads@redhat.com)
- use revamped source-to-image git url parser (jminter@redhat.com)
- bump(github.com/openshift/source-to-image):
  7da3d3e97565e652a59fd81286f3956fd43e85a8 (jminter@redhat.com)
- update godeps.json to run bump checkers (deads@redhat.com)
- UPSTREAM: <drop>: revert up to 43e60df912 UPSTREAM: 48884: Do not mutate pods
  on update (deads@redhat.com)
- update bootstrappolicy/dead addDeadClusterRole to include systemOnly
  annotation (admin@benjaminapetersen.me)
- UPSTREAM: 48813: maxinflight handle should let panicrecovery handler call
  NewLogged (deads@redhat.com)
- don't add ICT for source images if build is binary (bparees@redhat.com)

* Tue Jul 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.153-1
- Remove test-integration.sh dependance on symbols (jliggitt@redhat.com)
- Fix hack/test-integration.sh on OSX (jliggitt@redhat.com)
- Register aggregator resources into scheme prior to starting any components
  (jliggitt@redhat.com)
- Handle cleanup of individual authz objects (jliggitt@redhat.com)
- fix templateinstance update test logic (jminter@redhat.com)
- Run separate informers for api and controllers (jliggitt@redhat.com)

* Mon Jul 17 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.152-1
- Remove unused flag from latest prometheus example (ccoleman@redhat.com)

* Sun Jul 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.151-1
- 

* Sun Jul 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.150-1
- tail build logs into build object (bparees@redhat.com)
- UPSTREAM: 48884: Do not mutate pods on update (ccoleman@redhat.com)
- Make the master endpoint lease ttl configurable (ccoleman@redhat.com)

* Sat Jul 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.149-1
- Remove use of policy API from CLI (mkhan@redhat.com)
- Set mutation limit proportional to read limit by default
  (ccoleman@redhat.com)

* Fri Jul 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.148-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 118403a
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  adf6034c1033297756e0b04417de3cc849d4532b (eparis+openshiftbot@redhat.com)
- Remove download of goversioninfo (jupierce@redhat.com)

* Fri Jul 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.147-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console d51bb45
  (smunilla@redhat.com)
- goversioninfo for windows resource file edits (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  860e8a9b09fe11778ff069babc5d3974ad7567d3 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  450419593645ed81215ed13c0c67696eef3cf976 (eparis+openshiftbot@redhat.com)
- Unconditionally remove proxy headers to prevent httpoxy (simo@redhat.com)

* Fri Jul 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.146-1
- Adding git to support got get (jupierce@redhat.com)

* Fri Jul 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.145-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console e9fb159
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5e4a98b87b2a66f3ac96c11691480c10858f4ec4 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  23ed4cc6cc7c8ea0cdceeb8c0ab043fd03e3f5dd (eparis+openshiftbot@redhat.com)
- Update release scripts (ccoleman@redhat.com)
- Windows compile time info (ffranz@redhat.com)
- UPSTREAM: 48613: proxy/userspace: honor listen IP address as host IP if given
  (dcbw@redhat.com)

* Thu Jul 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.144-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 22172b6
  (smunilla@redhat.com)
- Update image-promotion.md (p3tecracknell@gmail.com)
- Ensure CNI dir exists before writing openshift CNI configuration under CNI
  dir (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e3e7ba71d505d1c65b890cc39169131584a46a1d (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8f900905ebebc9bdb3c0e24c90b5a85c9a7d8343 (eparis+openshiftbot@redhat.com)
- Update broken jUnit reporter tests (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  649b78ed8a4a6296e738f5f691078da4f34998cb (eparis+openshiftbot@redhat.com)
- Add an option to refresh to newest hack/env image (skuznets@redhat.com)
- add PR testing hook for openshift-login plugin (gmontero@redhat.com)
- UPSTREAM: 48635: proxy/userspace: suppress "LoadBalancerRR: Removing
  endpoints" message (dcbw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b9034e8109e833d961d771a4ecbedbfeb4422919 (eparis+openshiftbot@redhat.com)
- re-enable newapp conformance test (bparees@redhat.com)
- Add integration test for preferred GVs in discovery
  (stefan.schimanski@gmail.com)
- swagger.json should be accessible to anonymous users (ccoleman@redhat.com)
- Allow installation of missing Golang dependencies at run-time
  (skuznets@redhat.com)
- Update generated completions (ffranz@redhat.com)
- UPSTREAM: 44746: support for PodPreset in get command (ffranz@redhat.com)
- Tolerate NotFound on delete, remove roles on policybinding deletion
  (jliggitt@redhat.com)
- make GC mutation check ignore selfLink (deads@redhat.com)
- Bump OVS version requirement to 2.6.1 (sdodson@redhat.com)

* Wed Jul 12 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.143-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 2ff23c3
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  569f20cc5d988c63d84a2aefd989957795ab35b5 (eparis+openshiftbot@redhat.com)

* Wed Jul 12 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.142-1
- Increase default maxInFlightRequests to 1200 (ccoleman@redhat.com)
- UPSTREAM: <carry>: Lengthen too short timeouts on startup
  (ccoleman@redhat.com)
- Live client check only if scopes were added (mkhan@redhat.com)
- Update lastTriggeredImage if not set when instantiating DCs
  (mkargaki@redhat.com)
- set broker catalog poll interval to 5minutes (bparees@redhat.com)
- UPSTREAM: 48733: Never prevent deletion of resources as part of namespace
  lifecycle (jliggitt@redhat.com)
- UPSTREAM: 48624: kube-proxy logs abridged (decarr@redhat.com)
- UPSTREAM: 48085: Move iptables logging in kubeproxy (decarr@redhat.com)
- Adding meta tag so the login screen renders correctly when IE is in intranet
  mode (rhamilto@redhat.com)
- Get encryption configuration from a config and apply resource transformers.
  (vsemushi@redhat.com)
- Support valueFrom in build environment variables #2 (cdaley@redhat.com)
- origin, syscontainer: add bind mount for /etc/pki (gscrivan@redhat.com)
- node, syscontainer: add bind mount for /etc/pki (gscrivan@redhat.com)
- Typo fix of 'execure' on oc cluster up --help (jwmatthews@gmail.com)
- update upstream kubernetes link for cni (rchopra@redhat.com)

* Tue Jul 11 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.141-1
- Tolerate deletion of resources during migration (mkhan@redhat.com)
- Do not resolve images on job/build/statefulset updates (ccoleman@redhat.com)
- dump jenkins logs on env var ext test mismatch (gmontero@redhat.com)

* Mon Jul 10 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.140-1
- Add a helper for finding dependency chains (ccoleman@redhat.com)
- ImageStreamTag update should ignore missing labels (ccoleman@redhat.com)

* Sat Jul 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.139-1
- 

* Sat Jul 08 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.138-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console e52273d
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ed474b193b37dc5a206368a9843275a7fe276786 (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  09e388accea5f3412c2f39581f23e779c72b5214 (eparis+openshiftbot@redhat.com)
- UPSTREAM: 48578: run must output message on container error
  (ffranz@redhat.com)
- Deleted image streams are never removed from controller queue
  (ccoleman@redhat.com)
- UPSTREAM: 48582: Fixes oc delete ignoring --grace-period. (ffranz@redhat.com)
- cluster up: fix host volume share creation (cewong@redhat.com)
- Updates to the cleanup DC test (mkargaki@redhat.com)
- Clarify what needs to be said in cherry-pick commit comments
  (bbennett@redhat.com)
- Separate serviceaccount and secret storage config. (vsemushi@redhat.com)
- UPSTREAM: 47822: Separate serviceaccount and secret storage config
  (vsemushi@redhat.com)

* Fri Jul 07 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.137-1
- Use admin commands on origin container to install router and registry
  (cewong@redhat.com)
- UPSTREAM: <drop>: Increase SAControllerClientBuilder timeout
  (ccoleman@redhat.com)
- UPSTREAM: 48481: protect against nil panic in apply (ffranz@redhat.com)
- Add static library to release image (ccoleman@redhat.com)
- bump(github.com/openshift/source-to-image):
  7f58756254b0a65bf59fa87a8ecedad01ce6a85b (bparees@redhat.com)
- disable autoscaling v2alpha1 by default (deads@redhat.com)
- have TemplateInstance objects appear in swagger spec (jminter@redhat.com)

* Thu Jul 06 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.136-1
- When CRI runtime is not docker, don't init docker socket
  (ccoleman@redhat.com)
- Reverse the order of migrate output to match desired visual outcome
  (ccoleman@redhat.com)
- Get more error info if OVS ofport allocation fails (danw@redhat.com)
- Deployer pod may be unable to observe started pod from API
  (ccoleman@redhat.com)
- cluster up: Fix the regular expression used to parse openshift version
  (cewong@redhat.com)
- Add disabled DNS test back into conformance (ccoleman@redhat.com)
- Throw error using --context-dir with a template (cdaley@redhat.com)
- AggregatedLogging diagnostic: handle optional components better
  (lmeyer@redhat.com)
- AggregatedLogging diagnostic: there is no deployer (lmeyer@redhat.com)
- UPSTREAM: <carry>: increase job re-list time in cronjob controller
  (maszulik@redhat.com)
- UPSTREAM: 46121: Fix kuberuntime GetPods (sjenning@redhat.com)

* Wed Jul 05 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.135-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 04a56c8
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5fdc2282f59424d6e3aee74fa347cb0c5cae918b (eparis+openshiftbot@redhat.com)
- bump(k8s.io/kubernetes): fff65cf41bdeeaff9964af98450b254f3f2da553
  (deads@redhat.com)
- fix tsb error message (jminter@redhat.com)
- use apiserver args for enabling and disabling alpha versions
  (deads@redhat.com)
- Add an option to skip all cleanup with $SKIP_CLEANUP (skuznets@redhat.com)
- Break up os::cleanup::all (skuznets@redhat.com)
- Node DNS should answer PTR records for stateful sets (ccoleman@redhat.com)

* Wed Jul 05 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.134-1
- Update to use new published images (ccoleman@redhat.com)
- Updated Prometheus template to account for new config parameters
  (andy.block@gmail.com)
- ResolverConfig should be left on so search path is inherited
  (ccoleman@redhat.com)
- sdn: add better logging of ofport request failure (dcbw@redhat.com)
- UPSTREAM: 47347: actually check for a live discovery endpoint before
  aggregating (deads@redhat.com)
- make GC skipping resources avoid gc finalizers (deads@redhat.com)
- UPSTREAM: 48354: allow a deletestrategy to opt-out of GC (deads@redhat.com)
- Test DC's AvailableReplicas reporting when MinReadySeconds set
  (tnozicka@gmail.com)
- Fix avaibility reporting for DC with MinReadySeconds (tnozicka@gmail.com)

* Tue Jul 04 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.133-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 599adf3
  (smunilla@redhat.com)
- UPSTREAM: 48394: Verify no-op updates against etcd always
  (ccoleman@redhat.com)
- add audit logging to apiserver starts (deads@redhat.com)
- Image trigger controller should handle extensions v1beta1
  (ccoleman@redhat.com)
- move the panic handler first (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1c22134e39384afb563dba02546031a4676b55ae (eparis+openshiftbot@redhat.com)
- cli: recognize persistent volume claim in status (mfojtik@redhat.com)
- generated: OpenAPI spec (ccoleman@redhat.com)
- UPSTREAM: 44784: Handle vendored names in OpenAPI gen (ccoleman@redhat.com)
- Pass correct namer to OpenAPI spec (ccoleman@redhat.com)
- UPSTREAM: 47078: HPA: only send updates when the status has changed
  (sross@redhat.com)

* Mon Jul 03 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.132-1
- oadm migrate was double printing early exit error (ccoleman@redhat.com)
- Restore custom HPA controller setup (sross@redhat.com)
- bump(github.com/openshift/source-to-image):
  b4097ed9cdefeb304d684c5ff34a764a45244c7c (bparees@redhat.com)
- oc new-app --build-env doesn't work on templates (cdaley@redhat.com)

* Sun Jul 02 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.131-1
- Revert "Windows compile time info" (ccoleman@redhat.com)
- Bug 1453190 - Fix pod update operation (rpenta@redhat.com)
- ipfailover - control preempt strategy (pcameron@redhat.com)
- set triggers is fetching resources twice (ccoleman@redhat.com)
- bump(github.com/opencontainers/runc):
  d223e2adae83f62d58448a799a5da05730228089 (sjenning@redhat.com)
- explicitly enable alpha apis when SC is on (bparees@redhat.com)
- UPSTREAM: 48343: don't accept delete tokens taht are waiting to be reaped
  (deads@redhat.com)
- don't accept deleted tokens (deads@redhat.com)
- loosen deploymentconfig version checking in extended tests
  (bparees@redhat.com)
- bug fixes to extended test deployment config logging (jminter@redhat.com)
- Router bug fix: allow access with mixedcase/uppercase hostnames
  (yhlou@travelsky.com)
- UPSTREAM: 47367: add client side event spam filtering (decarr@redhat.com)
- bump(github.com/juju/ratelimit): 5b9ff866471762aa2ab2dced63c9fb6f53921342
  (decarr@redhat.com)

* Sat Jul 01 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.130-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 8aad754
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2b43d484d29691540e1c1643764448e70991072c (eparis+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f46dc298e9e08b17aadf5e63331303b1be718a05 (eparis+openshiftbot@redhat.com)
- One more fix to SDN controller perms (danw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b60dacf4138d002b92b0a0d0c136c1f9ad7d42ef (eparis+openshiftbot@redhat.com)
- Disable ThirdPartyController (mkhan@redhat.com)
- Handle major, minor, and patch in Windows versioninfo (ffranz@redhat.com)
- adding versioninfo data to Windows binaries (shiywang@redhat.com)
- Remove unused encodeVserverId function (rajatchopra@gmail.com)
- Update completions (ffranz@redhat.com)
- UPSTREAM: <drop>: deprecate --api-version in oc config set-cluster
  (ffranz@redhat.com)
- UPSTREAM: 47919: Cherry: Use %%q formatter for error messages from the AWS
  SDK (jpeeler@redhat.com)
- fix version checking for sc enablement (bparees@redhat.com)
- Make private tags not match when running make build. (jpazdziora@redhat.com)
- add the nodes local IP address to OVS rules (jtanenba@redhat.com)
- fix for bz1465304 - use regular escaping of the resource, vserver is not
  special (rajatchopra@gmail.com)
- Autogenerated Swagger Spec (simo@redhat.com)
- Add securityDefinitions to the generated OpenAPI spec (simo@redhat.com)
- Move some oauth helpers in a new util package (simo@redhat.com)
- Use function to define default scopes (simo@redhat.com)
- Fix #14832 Mention anything as password during cluster up
  (budhram.gurung01@gmail.com)
- fix for bug1431655: delete reencrypt routes (rajatchopra@gmail.com)
- registry: allow to override the DOCKER_REGISTRY_URL and default to in-cluster
  address (mfojtik@redhat.com)
- UPSTREAM: 47347: actually check for a live discovery endpoint before
  aggregating (part 2) (deads@redhat.com)

* Fri Jun 30 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.129-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 8e1222f
  (smunilla@redhat.com)
- temporarily remove the new-app conformance test (bparees@redhat.com)
- Deployer controller emits event when failed to create deploy pod
  (decarr@redhat.com)
- SDN controller requires access to watch resources (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  da50b0868c606de8b4ecb4640bafa65fdef0569c (dmcphers+openshiftbot@redhat.com)
- don't use admin client, it loses namespace info (bparees@redhat.com)
- Remove the unnecessary golang duplicity. (#14946) (jpazdziora@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c9b5dba5598acd17d18a8cef1bd0715dda19c2cc (dmcphers+openshiftbot@redhat.com)
- extended: add deployment config failure trap to new-app (#14952) (mi@mifo.sk)
- more env, server side debug for pipeline ext tests (gmontero@redhat.com)
- UPSTREAM: 48261: fix removing finalizer for gc (deads@redhat.com)
- make service controller failure non-fatal again (sjenning@redhat.com)
- update bulk to have a serialization priority (deads@redhat.com)
- remove unused privileged clients from master config (mfojtik@redhat.com)
- Additional debugging information for WaitForOpenShiftNamespaceImageStreams
  (cdaley@redhat.com)
- make templateinstance secret optional (bparees@redhat.com)
- Allow project admins to create/edit/delete NetworkPolicies (danw@redhat.com)
- minor fixes made while debugging image_ecosystem extended test failure
  (jminter@redhat.com)

* Thu Jun 29 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.128-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console c982d21
  (smunilla@redhat.com)
- UPSTREAM: 44962: Remove misleading error from CronJob controller when it
  can't find parent (mfojtik@redhat.com)
- Updated bootstrappolicy (ppospisi@redhat.com)
- UPSTREAM: 46771: Allow persistent-volume-binder to List Nodes/Zones Available
  in the Cluster (ppospisi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  648bef724e728a1c16129997aba33e17ce065081 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2fb62238b922bebd311ccb43bc00c39500452828 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a16c85b780e7e12846b53e2694a56b360f1c539a (dmcphers+openshiftbot@redhat.com)
- Fix matchpathcon to only print the label (eparis@redhat.com)
- deploy: retry instantiate on conflicts (mfojtik@redhat.com)
- Bumping version ahead of stg for new versioning scheme (jupierce@redhat.com)
- Clean up and modernize CONTRIBUTING.md (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  69499249c93896c6f67c58fe2b0e8fe6be822c84 (dmcphers+openshiftbot@redhat.com)
- bump(k8s.io/kubernetes): 314edd5dc58f36ee1238dd2127a61feae14b9a4a
  (deads@redhat.com)
- set default build prune limits for group api (bparees@redhat.com)
- Fix go vet in TestRunAsAnyGenerate (mkhan@redhat.com)
- Refactor authorization policy REST storage code (mkhan@redhat.com)
- Use live lookups to resolve uncached role refs (mkhan@redhat.com)
- Update documentation for DC conditions (mkargaki@redhat.com)
- UPSTREAM: kubernetes-incubator/cluster-capacity: 81: Fix cluster-capacity to
  avoid pending pods not created by it. Fixes
  https://bugzilla.redhat.com/show_bug.cgi?id=1449891 (avagarwa@redhat.com)
- allow templateinstance updates for GC (deads@redhat.com)
- add indexer before the informers are started until the generator is rewritten
  (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  76b9495ac9392d18ebd8eb5de8b83ce19b6f682a (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 47904: prioritize messages for long steps (deads@redhat.com)
- UPSTREAM: <drop>: Use Clone for CloneTLSConfig for golang 1.8
  (jdetiber@redhat.com)
- UPSTREAM: 44058: Make background garbage collection cascading
  (maszulik@redhat.com)
- Partial revert 13326de3e8d86386abc4e89ada132d24d6490be3 (maszulik@redhat.com)
- restore etcd main file (deads@redhat.com)
- refactor openshift start to separate controllers and apiserver
  (deads@redhat.com)
- Add extended tests for DC ControllerRef (tnozicka@gmail.com)
- Report multiple build causes for image change triggers (ccoleman@redhat.com)
- Preserve backwards compatibilty for old routes for destination CA
  (ccoleman@redhat.com)
- pass an internal pod object to SCC admission control so it works
  (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c5950988b45d881560f1a14cd68b08c87be28933 (dmcphers+openshiftbot@redhat.com)
- Image change trigger must be able to create all build types
  (ccoleman@redhat.com)
- Add a diagnostic that runs extended validation on routes
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  26d468aab574f66a99496c35c21edc6e45d55dba (dmcphers+openshiftbot@redhat.com)
- Allow objects to request resolution, not just images (ccoleman@redhat.com)
- Move the router to use generated clientsets (ccoleman@redhat.com)
- generated: completions (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b14785bb207e11689c58a6a23dc7cca229d67732 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 45637: --api-version on explain is not deprecated
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7bc6cbc81bc0e5a9a9a7a732dc860adfa2bd4367 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ba1ba6dc29593658b0af1b29b0cc506142c4dbaa (dmcphers+openshiftbot@redhat.com)
- Update output logging in hack/verify-gofmt.sh (skuznets@redhat.com)
- Router metrics tests should use the configured port (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2a223555a6247f530789cfc2d043b44245a8ce84 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 47823: don't pass CRI error through to waiting state reason
  (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  733e9f20be535f157fd313b204f1636b6e36cd99 (dmcphers+openshiftbot@redhat.com)
- Fix for go vet warning on ldaputil/client.go (bruno@abstractj.org)
- Add better logging to hack/push-release.sh (skuznets@redhat.com)
- sdn: kill containers that fail to update on node restart (dcbw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d0e24f52afc2b6cab5f22df1c59cdcad2d37d572 (dmcphers+openshiftbot@redhat.com)
- Updating GITHUB_WEBHOOK_SECRET description Updating external examples (that
  have already had the GITHUB_WEBHOOK_SECRET description updated)
  (cdaley@redhat.com)
- move API groups to canonical locations (deads@redhat.com)
- UPSTREAM: <carry>: update group references (deads@redhat.com)
- UPSTREAM: coreos/etcd: <drop>: Backwards compatibility to Go 1.7.5
  (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):v3.2.1 (ccoleman@redhat.com)
- stop deleting lister expansions (deads@redhat.com)
- script to manage the bulk of the moves (deads@redhat.com)
- UPSTREAM: <carry>: update client namer rules for amguity (deads@redhat.com)
- manual (deads@redhat.com)
- generated (deads@redhat.com)
- pick 14625: Allow setting volumes:[none] to disallow all volume types in SCC
  (deads@redhat.com)
- move SCC types to openshift (deads@redhat.com)
- write manual, backwards compatible client (deads@redhat.com)
- update generation scripts (deads@redhat.com)
- UPSTREAM: <drop>: generated SCC changes (deads@redhat.com)
- UPSTREAM: <carry>: drop SCC manual changes (deads@redhat.com)
- UPSTREAM: <carry>: add patch to allow shimming SCC (deads@redhat.com)
- Allow websocket authentication via protocol header (jliggitt@redhat.com)
- UPSTREAM: 47740: Use websocket protocol authenticator in apiserver
  (jliggitt@redhat.com)
- UPSTREAM: 47740: Add websocket protocol authentication method
  (jliggitt@redhat.com)
- add ext tests for cfgmap/is -> jenkins slave pod template
  (gmontero@redhat.com)
- Generate: oadm migrate volumesource (mkhan@redhat.com)
- Add oadm migrate volumesource (mkhan@redhat.com)
- Move NotChanged reporter to allow reuse (mkhan@redhat.com)
- UPSTREAM: 48017: Plumb preferred version to nested object encoder
  (jliggitt@redhat.com)
- UPSTREAM: 47973: include object fieldpath in event key (sjenning@redhat.com)
- make sure that GC can delete privileged pods (deads@redhat.com)
- UPSTREAM: 47975: make proto time precision match json (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  066938a2329cb306fe293287d718b51b75159eae (dmcphers+openshiftbot@redhat.com)
- Fix ClusterNetwork REST GetAttrs (mkhan@redhat.com)
- Update default allowed registries (maszulik@redhat.com)
- allow templateinstance controller to instantiate non-v1 objects
  (jminter@redhat.com)
- Errors must be always shown in oc status (ffranz@redhat.com)
- UPSTREAM: 46440: fix api server handler routing (move CRD behind TPR)
  (jdetiber@redhat.com)
- UPSTREAM: 41758: Updated key.pm and cert.pm to remove error in setting up
  localhostCert pool (jdetiber@redhat.com)
- UPSTREAM: 44583: bump bazel build to go1.8.1 and remove invalid unit tests
  (jdetiber@redhat.com)
- UPSTREAM: 44579: make certs used in roundtripper_test same as those used in
  proxy_test (jdetiber@redhat.com)
- move SC and logging templates to a system namespace (bparees@redhat.com)
- UPSTREAM: 45049: Log an EBS vol's instance when attaching fails because
  VolumeInUse (mawong@redhat.com)
- Added IPv6 support for the ipfailover keepalived image.  - Added IPv6 Address
  support  - Added IPv6 Address Range Support  - Added IPv6 Address Validation
  - Added IPv4 Address Validation  - Added relevant test cases
  (rmedina@netlabs.com.uy)
- Build and ship ginkgo binary with extended tests (skuznets@redhat.com)
- assorted improvements to template service broker - include name of object
  that fails SAR where appropriate, to improve error   message - prevent bind
  from working when a given key in credentials is assigned more   than once -
  in catalog, set description to "No description provided" when no description
  is provided - prevent a possible panic in the API controller if a malformed
  template is   passed in - add a test to ensure templateinstances can be
  created and deleted with   templates with objects in multiple namespaces
  (jminter@redhat.com)
- Build junitreport with hack/build-go.sh, based on
  tools/junitreport/README.md. (jpazdziora@redhat.com)
- add bash retry around all sh->oc invocations; rework openshift-jee-sample-
  docker start/verify build; add master stack dump util switch to native mem
  tracking bump pipeline tests jenkins container mem based on latest native mem
  analysis (gmontero@redhat.com)
- node, syscontainer: add bind mount for /var/lib/dockershim
  (gscrivan@redhat.com)
- node, syscontainer: add bind mount for /etc/cni/net.d (gscrivan@redhat.com)
- openvswitch, syscontainer: add missing substitution (gscrivan@redhat.com)
- Add unit test for second run of DC trigger (tnozicka@gmail.com)
- don't prevent updates that only touch ownerrefs (deads@redhat.com)
- Fix godep-save by ignoring cmd/cluster-capacity and cmd/service-catalog dirs.
  (avagarwa@redhat.com)
- enable podpresets with service catalog (bparees@redhat.com)
- Treat connection refused as "not available" in bind test
  (ccoleman@redhat.com)
- Stop setting port 1935 in the router configuration (ccoleman@redhat.com)
- Disable stats port when new router metrics are on (ccoleman@redhat.com)
- When stats port is -1, completely disable stats port (ccoleman@redhat.com)
- Format CONTRIBUTING.adoc consistently (skuznets@redhat.com)
- Remove local Vagrant workflow from CONTRIBUTING.adoc (skuznets@redhat.com)
- Fix chained DC IST trigger (tnozicka@gmail.com)
- Do not force any selinux context on volumeDir (eparis@redhat.com)
- The --ports flag does not modify dc env variables (pcameron@redhat.com)
- hack: drop obsolete build env from build-images.sh (mkargaki@redhat.com)

* Fri Jun 23 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.123-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 1922a40
  (smunilla@redhat.com)
- bump(k8s.io/kubernetes): e14cf3cef86ecaf0ec2a2b685a8113c0b258eacb
  (deads@redhat.com)
- don't use hyphens in template.openshift.io/(base64-)?expose- annotation key
  (jminter@redhat.com)
- Update generated completions. (vsemushi@redhat.com)
- proxy: honor BindAddress for the iptables proxy (dcbw@redhat.com)
- Handle system logging cleanup when no logfile exists (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  93f3bfb7d598276309585d25eb9e158fb7597fd3 (dmcphers+openshiftbot@redhat.com)
- Replace fsouza go-docker client with engine-api client in cluster up
  (cewong@redhat.com)
- Bump default namespace controller workers to 10 (ccoleman@redhat.com)
- UPSTREAM: 46796: Bump namespace controller to 10 workers
  (ccoleman@redhat.com)
- The hack/common.sh is no more. (jpazdziora@redhat.com)
- add a display name to the jenkins pipeline example (bparees@redhat.com)
- UPSTREAM: 46460: Add configuration for encryption providers
  (vsemushi@redhat.com)
- fix policies for unidling controller (mfojtik@redhat.com)
- make sdn retry on auth failure (mfojtik@redhat.com)
- refactor rest of the origin controllers and kube controllers initialization
  (mfojtik@redhat.com)
- UPSTREAM: 46034: Event aggregation: include latest event message in aggregate
  event (sjenning@redhat.com)
- UPSTREAM: 47792: Fix rawextension decoding in update (jliggitt@redhat.com)
- bump(github.com/elazarl/goproxy): c4fc26588b6ef8af07a191fcb6476387bdd46711
  (jdetiber@redhat.com)
- UPSTREAM: 44113: vendor: Update elazarl/goproxy to fix e2e test with go1.8
  (jdetiber@redhat.com)
- Update oc run help (maszulik@redhat.com)
- deploy: remove leading and trailing spaces from images (mfojtik@redhat.com)

* Wed Jun 21 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.122-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console a4b7297
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b2af7e03656601ffbddee340279068a5899ffa30 (dmcphers+openshiftbot@redhat.com)
- enforce server version requirement for service catalog (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8166d90d6d05ede9da33463f89751ad3586ceb30 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5912a3ac9352cf8aa8b4d570a70dc91a96cac082 (dmcphers+openshiftbot@redhat.com)
- give the service catalog controller event CRUD (bparees@redhat.com)
- doc cluster up --service-catalog flag (bparees@redhat.com)

* Tue Jun 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.121-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 1a8ad18
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1e799562a0bb10ff9da976f484ba4c2e5f5ed0b2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fdd786b139287f4779b1baa78e4deb2f130e8ce9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  aca7203cc7612e877cd9c7625dfee00c986be5b9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  95a85d302c46bb217370ec0b8a71b3fc0c773ace (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e68d9d8beb14ff2096adebd022a3efc9008844d7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a94c2f2924b06462509e30a9dda0141c51e40e4d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  54c8cb20b3176f46ee7f87e69ed60a4be87e3b55 (dmcphers+openshiftbot@redhat.com)
- TestScopedProjectAccess should handle other modifications
  (ccoleman@redhat.com)
- Update cluster quota mapping to efficiently handle internal and external
  (ccoleman@redhat.com)
- Namespace security allocator should use versioned informers
  (ccoleman@redhat.com)
- Project finalizer should use versioned informer (ccoleman@redhat.com)

* Tue Jun 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.120-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console c5d3f3e
  (smunilla@redhat.com)
- Revert "make prometheus example work on older servers" (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eaa5d820b92a0e01cd530e3a00900e06845b538a (dmcphers+openshiftbot@redhat.com)
- update bindata for prometheus (mfojtik@redhat.com)
- make prometheus example work on older servers (mfojtik@redhat.com)
- retry ls-remote in new-app (bparees@redhat.com)
- improve error message when processing non-template resources
  (bparees@redhat.com)
- When sorting SCCs by restrictions don't add a score if SCC allows volumes of
  projected type. (vsemushi@redhat.com)

* Tue Jun 20 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.119-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 11d8da8
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ca13e6d720a4a2826a3fb744b5b33b3b3bbad6e3 (dmcphers+openshiftbot@redhat.com)
- Move deployments to use versioned Kubernetes informers (ccoleman@redhat.com)
- Build controller should reference versioned pods and secrets
  (ccoleman@redhat.com)
- Remove last internal use of service from controllers (ccoleman@redhat.com)
- Bias to using internal versions of some informers for GC
  (ccoleman@redhat.com)
- Migrate controllers to external secrets, services, and service accounts
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0bdb9296607544b3d0cc376d781a932fa7daf48e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bfd1f3a4cc8320376594ec9304e7419697a2dc80 (dmcphers+openshiftbot@redhat.com)
- Make clients require bash-completion (sdodson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  421609382be5f72a911a6390b4a4ef3fb196af38 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 47537: Fix typo in secretbox transformer prefix
  (vsemushi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  11dc8ce124266e481e12b0aecca96da7d4a5b1d4 (dmcphers+openshiftbot@redhat.com)
- Ensure OpenShift resources have a stable protobuf serialization
  (ccoleman@redhat.com)
- UPSTREAM: 47701: generated changes and client-go bump (ccoleman@redhat.com)
- UPSTREAM: 47701: Force protobuf to be stable in output (ccoleman@redhat.com)
- UPSTREAM: 44115: scheduler should not log an error when no fit
  (decarr@redhat.com)
- Fix the copy src behavior to copy the directory contents
  (bbennett@redhat.com)
- UPSTREAM: 47274: Don't provision for PVCs with AccessModes unsupported by
  plugin (mawong@redhat.com)
- Change the MAC addresses to be generated based on IP (dcbw@redhat.com)
- UPSTREAM: 47462: strip container id from events (decarr@redhat.com)
- UPSTREAM: 47270: kubectl drain errors if pod is already deleted
  (decarr@redhat.com)
- UPSTREAM: 45349: Fix daemonsets to have correct tolerations for
  TaintNodeNotReady and TaintNodeUnreachable. (avagarwa@redhat.com)

* Mon Jun 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.118-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 652afe2
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  59e53bf06a17fea16bf3aa11e7aa457e5e6e8ce6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  66e12d3ac7e52e8ffe2cf63d6b6d478fc9d0ceac (dmcphers+openshiftbot@redhat.com)

* Mon Jun 19 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.117-1
- UPSTREAM: 47605: Change Container permissions to Private for provisioned
  Azure Volumes (hchen@redhat.com)
- Add build pruning test for BuildPhaseError (cdaley@redhat.com)
- `oadm migrate storage` counts extra when filtering (ccoleman@redhat.com)
- oc new-app display correct error on missing context directory
  (cdaley@redhat.com)
- Require proxy-mode=iptables for NetworkPolicy plugin (for now)
  (danw@redhat.com)
- Fix Services-vs-NetworkPolicy (danw@redhat.com)
- Grab a newer OVS binary for the dind image (danw@redhat.com)

* Sun Jun 18 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.116-1
- Aggregator uses internal versions until 3.7 <DROP> (ccoleman@redhat.com)
- UPSTREAM: <drop>: Use internal service/endpoints informers in aggregator
  (ccoleman@redhat.com)
- make cgroup imports linux-only (jliggitt@redhat.com)
- Ensure the build controller uses generated clients (ccoleman@redhat.com)
- Remove the legacy shared informer factory (ccoleman@redhat.com)
- Log error verifying state (jliggitt@redhat.com)
- UPSTREAM: 47491: image name must not have leading trailing whitespace
  (decarr@redhat.com)
- UPSTREAM: 47450: Ignore 404s on evict (decarr@redhat.com)
- NetworkCheck diagnostic: create projects with empty nodeselector
  (lmeyer@redhat.com)
- Use the original response message, avoid hiding the original cause.
  (jpazdziora@redhat.com)
- UPSTREAM: 47281: Update devicepath with filepath.Glob result
  (mitsuhiro.tanino@hds.com)

* Sat Jun 17 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.115-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console c747505
  (smunilla@redhat.com)
- (WIP) Support valueFrom syntax for build env vars (#14143)
  (cdaley@redhat.com)
- install service catalog w/ oc cluster up (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a5253ad733227bb825a62bf4f170cdc29c40099c (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  178424f9fcf67f3d524e75992db11159e9814b81 (dmcphers+openshiftbot@redhat.com)
- set cgroup parent on build child containers (bparees@redhat.com)
- Autoscaler should GC via autoscaling v1 (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fe95b42008af2d72302d032dc7962803193e8bfd (dmcphers+openshiftbot@redhat.com)
- Add hack/verify-generated-json-codecs.sh to find unexpected codecs
  (stefan.schimanski@gmail.com)
- UPSTREAM: <carry>: remove JSON codec (jliggitt@redhat.com)
- UPSTREAM: k8s.io/metrics: <carry>: remove JSON codec (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  370db11a5c6ecb89513d8ddacaf77d2ef69da6e0 (dmcphers+openshiftbot@redhat.com)
- Remove generated_certs.d from vendored docker in service-catalog, as it
  breaks checkout on windows. (avagarwa@redhat.com)
- Add template.openshift.io/expose annotation for use with tsb bind
  (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6afcb56ae11d5d8e40a7d9118c117cf1bedfa3df (dmcphers+openshiftbot@redhat.com)
- add prioritization for aggregation (deads@redhat.com)
- bump(github.com/openshift/source-to-image):
  41976c1106ca25fa7b51632b440838e8bf6d25ed (bparees@redhat.com)
- Update openapi, scc sorting (jliggitt@redhat.com)
- UPSTREAM: <carry>: SCC FSType none (jliggitt@redhat.com)
- Add description parsing to allow subtree merges (jpeeler@redhat.com)
- Remove generated_certs.d from vendored docker in cluster-capacity
  (avagarwa@redhat.com)
- UPSTREAM: 47003: Fix sorting of aggregate errors for golang 1.7.
  (avagarwa@redhat.com)
- UPSTREAM: 46800: generated (deads@redhat.com)
- UPSTREAM: 46800: separate group and version priority (deads@redhat.com)
- UPSTREAM: 45085: kube-apiserver: check upgrade header to detect upgrade
  connections (deads@redhat.com)
- UPSTREAM: 45247: generated: Promote apiregistration from v1alpha1 to v1beta1
  (deads@redhat.com)
- UPSTREAM: 45247: Promote apiregistration from v1alpha1 to v1beta1
  (deads@redhat.com)
- update manage-node to support multiple output formats (jvallejo@redhat.com)
- service-catalog: do not build user-broker (jpeeler@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' changes from c91fecb..568a7b9 (jpeeler@redhat.com)
- Add DefaultIOAccounting to all openshift services (ccoleman@redhat.com)
- UPSTREAM: 47516: Fix getInstancesByNodeNames for AWS (hekumar@redhat.com)
- UPSTREAM: gophercloud/gophercloud: 383: support HTTP Multiple Choices in
  pagination (hchen@redhat.com)
- UPSTREAM: 47003: Remove duplicate errors from an aggregate error input. Helps
  Helps with some scheduler errors that fill the log enormously.
  (avagarwa@redhat.com)
- issue #495. Use openshift-ansible for logging and metrics in 'oc cluster up'
  (jcantril@redhat.com)

* Fri Jun 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.114-1
- Donot serve certificate content for Non-SSL routes (pcameron@redhat.com)
- give template instance controller admin permissions (jminter@redhat.com)

* Fri Jun 16 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.113-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console df92c71
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ed51404a3febc8c3b6bce616933cd6003fb0ea8d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4bd3b4af68c492988ce1575df8e3a2478ef020dd (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c0d9ac2675038cb8bd877a52791f46859108c5c9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fa0779958c86359d42dea484be5f2b7d96eac3ac (dmcphers+openshiftbot@redhat.com)
- Prevent duplicate deployment informers (ccoleman@redhat.com)
- add httpd imagestreams and quickstart (bparees@redhat.com)
- UPSTREAM: <drop>: fix unit test (jliggitt@redhat.com)
- Run all k8s unit tests (jliggitt@redhat.com)
- UPSTREAM: kubernetes-incubator/cluster-capacity: 78: Remove resources that
  are not needed to run cluster  capacity analysis. (avagarwa@redhat.com)
- UPSTREAM: kubernetes-incubator/cluster-capacity: 77: cluster-capacity should
  only list resources needed by  scheduler and use a fake informer for replica
  sets. (avagarwa@redhat.com)
- acceptschema2: true (aweiteka@redhat.com)
- enable registry middleware acceptschema2 (aweiteka@redhat.com)

* Thu Jun 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.112-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 782afed
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  039d7654c398ab0e741b1bd8a8116af8dde0ab3f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1a990338d4e4cee7b0aadeda3346b9144fc6d2fb (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 43982: Fix deletion of Gluster, Ceph and Quobyte volumes
  (jsafrane@redhat.com)
- handle nil URLs in safeforlogging (bparees@redhat.com)
- Route security management by end user (pcameron@redhat.com)

* Thu Jun 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.111-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console e10e9a8
  (smunilla@redhat.com)
- Add keepalived-ipfailover image support (#14620)
  (knobunc@users.noreply.github.com)
- bump(github.com/openshift/origin-web-console):
  6e95ce2a77f3248acf13caff408e858a149b0fe3 (dmcphers+openshiftbot@redhat.com)
- Fix broken test case (ccoleman@redhat.com)
- bump(k8s.io/kubernetes): 3a5b73339d2adbb002a9820e125a5529f3a749fd
  (deads@redhat.com)
- fix buildcontroller integration test to use generated informers
  (mfojtik@redhat.com)
- refactor initialization for image, service serving certs and trigger
  controllers (mfojtik@redhat.com)
- simplify controller wiring (deads@redhat.com)
- Add `oadm migrate etcd-ttl` which encodes upstream TTL migration
  (ccoleman@redhat.com)
- Report the volume of etcd writes via a diagnostic (ccoleman@redhat.com)
- Use generated informer with cluster resource quota (ccoleman@redhat.com)
- Remove ImageStreamReferenceIndex from BuildInformer (cewong@redhat.com)
- Refactor BuildConfig controller to use Informers (cewong@redhat.com)
- don't start template informer unless templateservicebroker is configured
  (jminter@redhat.com)
- UPSTREAM: 47347: actually check for a live discovery endpoint before
  aggregating (deads@redhat.com)
- admin and editor should cover image builder (deads@redhat.com)
- Refactor Build Controller to use Informers (cewong@redhat.com)
- Eliminate nil/empty distinction for new v1 field (ironcladlou@gmail.com)
- UPSTREAM: 46852: Lookup no --no-headers flag safely in PrinterForCommand
  function (tnozicka@gmail.com)

* Thu Jun 15 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.110-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 9ccc920
  (smunilla@redhat.com)
- Fix leader election logging (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f55bd6ba4c42e84b062ced14986624156ca72f87 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3424e5cbd818a0bb4f4eaf3adc11c32eb7f6c1de (dmcphers+openshiftbot@redhat.com)
- Use GC rather than refcounting for VNID policy rules (danw@redhat.com)
- Add prometheus examples (ccoleman@redhat.com)
- bump(k8s.io/kubernetes): 2d104177c1bb3d16a21244b899f649016d63d684
  (deads@redhat.com)
- update describer test to create event directly (jvallejo@redhat.com)
- Document in a more correct manner imagebuilder dep (juanlu@redhat.com)
- Remove stale SDN code: Writing cluster network CIDR to config.env
  (rpenta@redhat.com)
- Bring image building logic into the Bash library (skuznets@redhat.com)
- Add instructions for `oc cluster up` on EC2 (skuznets@redhat.com)
- Document dependencies for build-base-images.sh (juanlu@redhat.com)
- Fix permissions on event harvest (mkargaki@redhat.com)
- UPSTREAM: 44898: while calculating pod's cpu limits, need to count in init-
  container (sjenning@redhat.com)
- add more packages to deep copy (deads@redhat.com)
- UPSTREAM: <drop>: regenerate code (deads@redhat.com)
- Allow users to create bindings to roles (mkhan@redhat.com)
- Make the default quorum reads (ccoleman@redhat.com)
- UPSTREAM: 43878: Delete EmptyDir volume directly instead of renaming the
  directory (hchen@redhat.com)
- UPSTREAM: 46974: Avoid * in filenames (jliggitt@redhat.com)
- Show SCC provider in error message. (vsemushi@redhat.com)

* Wed Jun 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.109-1
- 

* Wed Jun 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.108-1
- fix build to buildconfig ownerref (bparees@redhat.com)
- Use the generated informers for authorization (ccoleman@redhat.com)
- UPSTREAM: 46112: apimachinery: move unversioned registration to metav1
  (stefan.schimanski@gmail.com)

* Wed Jun 14 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.107-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 38b587f
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a0ff0091977bc1f6c0a74e447916fc1910e87593 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  55cff30264f68c32ccb39331ebbd802ea96fb403 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a56bdfd5c946d7db271635e5f494859f2de9a473 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eae97da11583fba53ec33f3c389bf9042d26d5b5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  871e99ab720cab4536b49ec0d1419510180d3bbe (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  96fab4dda4b1deae9decc6fc08aded3b19981ab4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a1c9e73d57951364f8d635cf0e09d96082fbb05f (dmcphers+openshiftbot@redhat.com)
- Tag local images with full name, not nick-names (#14624)
  (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  acdadd0b04277c352a4c67e4c995c0e91bf78ee7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  50bfbcca851359dbeebd2b2b1db2bd2c995c05cf (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  54262a23e1ca715abfaa85ca6d93d4f2555b0bc2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cf4b1228bcb640844a1f37467df1cd59e2e6fd22 (dmcphers+openshiftbot@redhat.com)
- deflake orphaning GC (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9c3ecd12ff769b28bd3412adb2e727c50b261fb2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c641cd8375f1a13d38f1d0d03ba86d5780f985c2 (dmcphers+openshiftbot@redhat.com)
- remove template.openshift.io/namespace parameter from template service broker
  and use context object instead (jminter@redhat.com)
- UPSTREAM: docker/distribution: 2299: Fix signalling Wait in regulator.enter
  (obulatov@redhat.com)
- Disable local repo by default (mkargaki@redhat.com)
- Add tests for the patternMatch template function (bbennett@redhat.com)
- Cleaned up the matchPattern regex code (bbennett@redhat.com)
- UPSTREAM: 47060: Fix etcd storage location for CRs (deads@redhat.com)

* Tue Jun 13 2017 Jenkins CD Merge Bot <smunilla@redhat.com> 3.6.106-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 99e9f72
  (smunilla@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5cb91fbd7cb9c94b9ec6a1505cc4ad2eef8ae909 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1cf3870091ecd65fe3780424958b2c577044d3e8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  44a4862dcbe7794f888e00a24030ba9fa6dca70d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a80d0d94219eeda41dace824fa9eb1a9cdefd961 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  07c5b8c09831ce6fa74493e6b251031d21c249e1 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  39472b9499cfa0ebc881ba4765376db431b3d629 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  513970c16696797a73c0eac1794e600a4a2b97c8 (dmcphers+openshiftbot@redhat.com)
- Improve the UX of the local image build script (#14600) (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4938784b11dc8d66194ec934294948c82d5fe36e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1f2c340a7e7f9d5b73b581a48f0855e1d6ea0512 (dmcphers+openshiftbot@redhat.com)
- Copy directory trees, and add support for haproxy images (#14602)
  (knobunc@users.noreply.github.com)
- Update image local build script (#14599) (skuznets@redhat.com)
- Add a missing : to if line (bbennett@redhat.com)
- Use Python for the local building script (skuznets@redhat.com)
- reset admission plugins for the openshift APIs (deads@redhat.com)
- UPSTREAM: <drop>: drop post 3.7 rebase.  allows disabled aggregator
  (deads@redhat.com)
- wire up openshift to be slight more 'normal' (deads@redhat.com)
- react to fixed http handlers (deads@redhat.com)
- UPSTREAM: 46440: fix api server handler routing (move CRD behind TPR)
  (deads@redhat.com)
- UPSTREAM: 44408: aggregator controller changes only (deads@redhat.com)
- UPSTREAM: 45432: use apiservice.status to break apart controller and handling
  concerns (deads@redhat.com)
- react to refactor names for the apiserver handling chain (deads@redhat.com)
- UPSTREAM: 45370: refactor names for the apiserver handling chain
  (deads@redhat.com)
- react to updating discovery wiring (deads@redhat.com)
- UPSTREAM: 00000: disambiguate operation names for legacy discovery
  (deads@redhat.com)
- UPSTREAM: 43003: separate discovery from the apiserver (deads@redhat.com)
- react to moving insecure options (deads@redhat.com)
- UPSTREAM: 42835: remove legacy insecure port options from genericapiserver
  (deads@redhat.com)
- react to requiring a codec (deads@redhat.com)
- UPSTREAM: 42896: require codecfactory (deads@redhat.com)
- UPSTREAM: 44466: use our own serve mux that directs how we want
  (deads@redhat.com)
- UPSTREAM: 44399: add deregistration for paths (deads@redhat.com)
- deploy: set ownerRef from RC to grouped API version (mfojtik@redhat.com)
- UPSTREAM: coreos/etcd: <drop>: Backwards compatibility to Go 1.7.5
  (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):v3.2.0 (ccoleman@redhat.com)
- Collapse code between authorizationsync and migrate (mkhan@redhat.com)
- Generate: oadm migrate authorization (mkhan@redhat.com)
- Add parity check for Openshift authz and Kube RBAC (mkhan@redhat.com)
- Remove deployment legacy informers in favor of generated
  (ccoleman@redhat.com)
- Replace legacy IS informer with generated informers everywhere
  (ccoleman@redhat.com)
- Update NetworkPolicy code for v1 semantics (danw@redhat.com)
- Allow specifying haproxy SSL Cipher list (pcameron@redhat.com)
- check status of router, registry, metrics, logging, imagestreams in oc
  cluster status (jminter@redhat.com)
- Update router cipher suites (pcameron@redhat.com)

* Mon Jun 12 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.105-1
- Reuse the authorization and template shared informers for GC
  (ccoleman@redhat.com)
- Migrate functions from and remove hack/common.sh (skuznets@redhat.com)
- Migrate final function from and remove hack/util.sh (skuznets@redhat.com)
- UPSTREAM: 45661: orphan when kubectl delete --cascade=false
  (deads@redhat.com)
- soften pipeline log missing annotation msg (gmontero@redhat.com)
- make template service broker forbidden error message friendlier
  (jminter@redhat.com)
- UPSTREAM: 46916: Add AES-CBC and Secretbox encryption (ccoleman@redhat.com)
- bump(golang.org/x/crypto): add nacl, poly1305, and salsa20
  (ccoleman@redhat.com)
- Update the bootstrap/policy convertClusterRoles function to annotate
  systemOnly roles (admin@benjaminapetersen.me)

* Sun Jun 11 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.104-1
- bump(github.com/boltdb/bolt): v1.3.0 (ccoleman@redhat.com)
- Segregate OpenShift's iptables rules (danw@redhat.com)
- Minor iptables rule cleanups (danw@redhat.com)
- Align DIND container image build with Orgin scripts (skuznets@redhat.com)
- UPSTREAM: 46510: remove duplicate, flaky tests (deads@redhat.com)
- UPSTREAM: 45864: Fix unit tests for autoregister_controller.go reliable
  (deads@redhat.com)
- Fixed a missing escape that caused the usage to fail when run
  (bbennett@redhat.com)
- Fix signature workflow extended test (miminar@redhat.com)
- use cp; rm instead of mv to transfer the tito output to _output.
  (jminter@redhat.com)
- Add NormalizePolicyRules to authorizationsync (mkhan@redhat.com)
- sdn: be a normal CNI plugin (dcbw@redhat.com)
- Fix concurrency error in registry's pull-through (miminar@redhat.com)
- sdn: remove IPAM garbage collection (dcbw@redhat.com)

* Sat Jun 10 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.103-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console ab8a3e0
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cdf2198ce38f9c78ec67ffa8746d215c0761617e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9e2599c098405cb59f9e3d4b26793c1d9c03ce0e (dmcphers+openshiftbot@redhat.com)
- script for building local images (bparees@redhat.com)
- service-catalog: add dockerfile and add to build (jpeeler@redhat.com)
- service-catalog: add sc files to origin RPM (jpeeler@redhat.com)
- service-catalog: ignore needed directories (jpeeler@redhat.com)
- Squashed 'cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-
  catalog/' content from commit c91fecb (jpeeler@redhat.com)
- Enable pod preset admission, but default to off (decarr@redhat.com)
- make templateinstance immutability message less unfriendly
  (jminter@redhat.com)
- template service broker catalog: only list as required those template
  parameters which are marked required and which cannot be generated
  automatically (jminter@redhat.com)
- sdn: don't require netns on Update action (dcbw@redhat.com)
- sdn: use OVS flow parsing utils for AlreadySetUp() (dcbw@redhat.com)
- ovs: split out flow parsing code, expose API, and parse actions
  (dcbw@redhat.com)
- Reinstate "e2e test: remove PodCheckDns flake" (lmeyer@redhat.com)
- sdn: don't allow pod bandwidth/QoS changes after pod start (dcbw@redhat.com)
- Add an HTTP proxy mode to egress router (danw@redhat.com)
- Revert "Add proper check for default certificate" (ichavero@redhat.com)
- Add proper check for default certificate (ichavero@redhat.com)
- Support IPv6 terminated at the router with internal IPv4
  (ichavero@redhat.com)

* Fri Jun 09 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.102-1
- bump(github.com/openshift/origin-web-console):
  513fa446b1d6ed3dc02b2e746ec28fedda035790 (dmcphers+openshiftbot@redhat.com)
- base-base-images: Use https for yum repo (chlunde@ifi.uio.no)
- oc cluster up: fix check for available ports when docker is running in user
  namespace mode. (vsemushi@redhat.com)
- bump(github.com/fsouza/go-dockerclient):
  3f9370a4738ba8d0ed6eea63582ca6db5eb48032 (vsemushi@redhat.com)

* Thu Jun 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.101-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console aad455c
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  adb6f072ddaadbac34e1f131d5b973b0112a7de7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e18553f36eaaea92f0a62e60cbb6679f98bf5116 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9fd7f5bb6099e9515ff1f7712154b45772961623 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  23aa410b0eaa66bd53bc15d4a6ed5d686aad55b7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7a49dade19e2b4e4c7af381c085c434709606840 (dmcphers+openshiftbot@redhat.com)
- Correctly place script output at _output/scripts (skuznets@redhat.com)
- UPSTREAM: 46968: bkpPortal should be initialized beforehand
  (mitsuhiro.tanino@hds.com)
- UPSTREAM: 46036: retry clientCA post start hook on transient failures
  (deads@redhat.com)
- permit OS_GIT_VERSION to have a git hash longer than 7 characters - this
  occurs when 7 characters are not enough to uniquely describe a given commit
  (jminter@redhat.com)
- Prevent POODLE vulnerability in HAProxy router (elyscape@gmail.com)

* Thu Jun 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.100-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console f83022a
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f70768f21d2470f62c8fdd5d0b6fdafe2cf73e7a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  774891b62dc176a0c17f9e7674cda69d8c010ea2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a18e11e64b454a3eea8dbda6dcec6c56fb3310bd (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ad1da6b7a0346ee473d8e19c8c9bbb8d6addf145 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f868176cbd4e5db14b98410b46e1b66e13198c90 (dmcphers+openshiftbot@redhat.com)
- bump(k8s.io/kubernetes): 010d313ca749b4005269f1cd1b9cd17168635490
  (deads@redhat.com)
- UPSTREAM: 43922: prevent corrupted spdy stream after hijacking connection
  (maszulik@redhat.com)
- Remove -p from oc exec in end-to-end, since it's deprecated
  (maszulik@redhat.com)
- Re-enable e2e tests that were failing due to #12558 (maszulik@redhat.com)
- Turn off journald rate limiter when running test-end-to-end-docker.sh
  (maszulik@redhat.com)
- Push origin-cluster-capacity image to docker hub. (avagarwa@redhat.com)
- Bug 1450291 - Improve logs in image pruning (maszulik@redhat.com)
- Remove duplicate migrate code (mkhan@redhat.com)
- UPSTREAM: <drop>: Backwards compatibility with etcd 3.2 and Go 1.7.5
  (ccoleman@redhat.com)
- UPSTREAM: coreos/etcd: <drop>: Backwards compatibility to Go 1.7.5
  (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):v3.2.0-rc.1 (ccoleman@redhat.com)
- do ugly things to wire a cloudconfig file (deads@redhat.com)
- Move compress AWK script into hack/lib (skuznets@redhat.com)
- Add OPENSHIFT_CONFIG_BASE environment, and doc the env (bbennett@redhat.com)
- Source bash completions from the dind cluster rc script (bbennett@redhat.com)
- Skip sudo if there is a writable docker socket (bbennett@redhat.com)
- Add new commands to hack/dind-cluster.sh (bbennett@redhat.com)
- Do some cleanups to the shell script in preparation for adding features
  (bbennett@redhat.com)
- Revert "disable image-references" (maszulik@redhat.com)

* Wed Jun 07 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.99-1
- 

* Tue Jun 06 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.98-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console be07324
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7eed6c131dbb2250082d076d16f97dff2c7aa3fb (dmcphers+openshiftbot@redhat.com)
- Add default directories to the $OS_BUILD_ENV_PRESERVE path
  (skuznets@redhat.com)
- Add $OS_OUTPUT_SCRIPTPATH and rename release/rpm path vars
  (skuznets@redhat.com)

* Tue Jun 06 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.97-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 8286ecd
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  52788fbc8480a5fe4f130e53077d64a70a4e361e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  09106a3e4136c360ee21e4f5a0888d138bc77c99 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9ca02da728bbad0aa6bd057e3f8cb545f53c72c1 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c8e8103398755f057175423f8d3a9828f7098a38 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5ecda44cacac97c291add305e4aa826789f9b648 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5068c0779128e0565da1bf8cd3de6f478e76048a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0985844fe88b231f80531a2bb5413b2a5a43b395 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f398ed206b697b9e66a0f87f8171f20d8441e6f8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eef932fe99d5e901f32d6a6f38c8c4b27ad9f19a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ae9077e84d54e64ea041556bb5845d1fa90c23af (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d21047eec24b5b45fdd8e662f6fa11e01e35d913 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ecca4ea57ef7807be9a6bc15a81fad0e07bba721 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  380e226f167ebb3c8ef657a68659e325667247ec (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b70413b3570ab5a6e76809c235b50cfe6316f993 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  78bd823d79705b6da452788c19f90d9100fed885 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6fce309ab345f01c60ed1385f7ff6ea15d0b1110 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a7a4737616f2d3cc1cca3cfa7cd87673bfabea6b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cbe550fb94d7b25ebfae81375599067fb0af0696 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1547af23ae25248fb8ff715e59aa7f56cbcbf73e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8ea0abdffbb46a1a0d2655c68b46ac52d3c06cac (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a8da383db5d71f2e5d810e143fb0ad64369e87dc (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  30c6d4138a03897da25dd6b9be7349f3f99066ad (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 46771: Allow pv-binder-controller to List Nodes/Zones Available in
  the Cluster (ppospisi@redhat.com)
- UPSTREAM: 46239: Log out from multiple target portals when using iscsi
  storage plugin (hchen@redhat.com)
- Split resource and non-resource rules during conversion (mkhan@redhat.com)
- correct serialisation of tsb json schemas (jminter@redhat.com)
- Fixes oc status for external name svc (ffranz@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fca8df36f90520ee4b02e45131c7f050396c6d73 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 46614: Add `auto_unmount` mount option for glusterfs fuse mount.
  (hchiramm@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1e873baee3d136bf0ab64ad2e9249a58925b9a50 (dmcphers+openshiftbot@redhat.com)
- Move docker specific scripts back into image directories (sdodson@redhat.com)
- UPSTREAM: 46751: Pre-generate SNI test certs (andy.goldstein@gmail.com)
- UPSTREAM: 45933: Use informers in scheduler / token controller (part 2,
  fixing tests) (andy.goldstein@gmail.com)
- Regenerate clientsets (andy.goldstein@gmail.com)
- Fix verifying generated clientsets (andy.goldstein@gmail.com)
- Fix unit testing kubernetes staging dir symlinks (andy.goldstein@gmail.com)
- bump(github.com/openshift/origin-web-console):
  b9918c6e98a5b13adc6679d3e46f39cffe522cfc (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 46640: Improve validation of active deadline seconds
  (decarr@redhat.com)
- “oc expose svc” should use service targetPort number
  (pcameron@redhat.com)
- UPSTREAM: 43945: Remove 'beta' from default storage class annotation
  (jsafrane@redhat.com)
- UPSTREAM: 46463: AWS: consider instances of all states in DisksAreAttached,
  not just "running" (mawong@redhat.com)
- Use base64url to decode id_token (jliggitt@redhat.com)
- migrate serviceaccount and rest of build controllers to new controller
  initialization (mfojtik@redhat.com)
- Auto generated: bash completions for network diagnostics (rpenta@redhat.com)
- Provide better error message when network diags test pod fails
  (rpenta@redhat.com)
- UPSTREAM: 46042: ResourceQuota admission control injects registry
  (federation) (decarr@redhat.com)
- OpenShift uses ResourceQuota admission plugin (decarr@redhat.com)
- UPSTREAM: 46042: ResourceQuota admission control injects registry
  (decarr@redhat.com)
- Bug 1417641 - Make network diagnostic test pod image/protocol/port
  configurable (rpenta@redhat.com)
- Commit otherwise ignored files to be in sync with cluster capacity repo.
  (avagarwa@redhat.com)
- Vendor cluster-capacity in origin. (avagarwa@redhat.com)
- Update build-images.sh to build cluster-capacity image. (avagarwa@redhat.com)
- Exclude cluster-capacity from test-go.sh (avagarwa@redhat.com)
- Remove cmd/cluster-capacity from go vet list. (avagarwa@redhat.com)
- Build cluster capacity from origin-cluster-capacity rpm.
  (avagarwa@redhat.com)
- Update origin.spec to build cluster capacity as a sub rpm.
  (avagarwa@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b1c04e8ef00995c6a0d22f318110e1a1302cec73 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  35fcf4cc6dd36061bbee649729913c316484de4a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c31218b92a2de7193172b93858633f057c4df118 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  029053efca17a857b4e5c8883acf1419dee8a868 (dmcphers+openshiftbot@redhat.com)
- Bug 1439142 - Use openshift/origin-deployer image instead of openshift/hello-
  openshift as network diagnostic test pod. (rpenta@redhat.com)
- UPSTREAM: 46628: cleanup kubelet new node status test (decarr@redhat.com)
- UPSTREAM: 46516: kubelet was sending negative allocatable values
  (decarr@redhat.com)
- Shuffle endpoints function for the router template : bz1447115
  (rchopra@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d3aed86aa0c4d2aea355b777f8df6f880815157f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  09b0e004e368f38f99d949f8078cb60c00f80885 (dmcphers+openshiftbot@redhat.com)
- updating to include BuildPhaseError and BuildPhaseCancelled
  (cdaley@redhat.com)
- Add DC controllerRef to RC (tnozicka@gmail.com)
- Fix deployment related SA's permissions (tnozicka@gmail.com)
- UPSTREAM: 46500: Fix standardFinalizers - add missing
  metav1.FinalizerDeleteDependents (Note: it is in different files from
  upstream because they moved helpers.go into helper/helpers.go)
  (tnozicka@gmail.com)
- UPSTREAM: 45894: Export BaseControllerRefManager (tnozicka@gmail.com)
- deploy: check the dc conditions instead of relying on deployer logs
  (mfojtik@redhat.com)
- UPSTREAM: 46608: fixes kubectl cached discovery on Windows
  (ffranz@redhat.com)
- pkg/cmd/server/api/validation/master.go: fix typo in warning message.
  (vsemushi@redhat.com)
- Detect cohabitating resources in etcd storage test (mkhan@redhat.com)
- UPSTREAM: revert: 54d84e6a8db4c07f78fb2823508fed7751ebf1bd: 24153: make
  optional generic etcd fields optional (mkhan@redhat.com)
- Set DeleteStrategy for all Openshift resources (mkhan@redhat.com)
- UPSTREAM: 46390: Require DeleteStrategy for all registry.Store
  (mkhan@redhat.com)
- UPSTREAM: 44068: Use Docker API Version instead of docker version (fixup)
  (dcbw@redhat.com)
- bump external examples (bparees@redhat.com)
- Add w.close for watch (song.ruixia@zte.com.cn)
- Allow basic users to 'get' StorageClasses (mawong@redhat.com)
- UPSTREAM: 44295: Azure disk: dealing with missing disk probe
  (hchen@redhat.com)
- sdn traffic leaking out of the cluster (pcameron@redhat.com)

* Tue May 30 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.86-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 481a31a
  (tdawson@redhat.com)
- Fix service ingress ip controller test flake (ccoleman@redhat.com)
- Wait for controller startup in test (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  468dcef3f11479886354b89813e7c077092db1e4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eef308f74c0d8c13102005257ba656328e05d88b (dmcphers+openshiftbot@redhat.com)
- generated (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ee1e8c341434674a65ec17ef3e7c131a49e18fe2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c7f6b00279648c8b0ed9e1253ccd181b29404246 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 44406: CRI: Stop following container log when container exited.
  (andy.goldstein@gmail.com)
- Don't ignore stderr when running integration tests with jUnit
  (skuznets@redhat.com)
- add artifacts for aggregation testing (deads@redhat.com)
- UPSTREAM: 44837: Fix Content-Type error of apis (deads@redhat.com)
- wire in aggregator (deads@redhat.com)
- add aggregator config (deads@redhat.com)
- UPSTREAM: <drop>: regenerate proto (deads@redhat.com)
- UPSTREAM: 43301: add APIService conditions (deads@redhat.com)
- Bump largest tolerable log size to 200M (skuznets@redhat.com)
- Match upstream changes (andy.goldstein@gmail.com)
- Updated generated completions (miminar@redhat.com)
- Template service broker documentation and logging updates
  (jminter@redhat.com)
- DiagnosticPod: double timeout for pod start (lmeyer@redhat.com)
- Prevent admin templates from instantiating on older versions
  (cewong@redhat.com)
- separate quota evaluation for admission versus reconciliation
  (deads@redhat.com)
- UPSTREAM: <drop>: Set the log level for iptables rule dump to 5
  (bbennett@redhat.com)
- UPSTREAM: 45427: 45897: GC controller improvements (andy.goldstein@gmail.com)
- UPSTREAM: revert: 62d77ec: UPSTREAM: <carry>: add OpenShift resources to
  garbage collector ignore list (andy.goldstein@gmail.com)
- Match ns controller concurrent syncs default change
  (andy.goldstein@gmail.com)
- UPSTREAM: 46437: Up namespace controller workers to 5
  (andy.goldstein@gmail.com)
- add best practice try/catch, timeout, specifically with extended tests in
  mind (gmontero@redhat.com)
- Add logging to imagechange build trigger (ccoleman@redhat.com)
- Bug 1454535 - Use created project name over namespace name in project
  template (mfojtik@redhat.com)
- UPSTREAM: 46373: don't queue namespaces for deletion if the namespace isn't
  deleted (deads@redhat.com)
- Initial support for nested gotests (jliggitt@redhat.com)
- Further constrain test/cmd/* project creation with prefixes
  (ccoleman@redhat.com)
- Don't log profile info on startup (ccoleman@redhat.com)
- Use shared informers in project auth cache (ccoleman@redhat.com)
- Rename origin namespace finalizer controller to be more precise
  (ccoleman@redhat.com)
- Use shared informers in origin namespace finalizer controller
  (ccoleman@redhat.com)
- Rename security allocation controller to be more precise
  (ccoleman@redhat.com)
- Use shared informers in security allocator controller (ccoleman@redhat.com)
- Use shared informers in project cache (ccoleman@redhat.com)
- Used shared informer for registry secret controllers (ccoleman@redhat.com)
- Use shared informers for serving cert controller (ccoleman@redhat.com)
- Use shared informers for ingress ip controller (ccoleman@redhat.com)
- Add node config option for a resolv.conf to read (ccoleman@redhat.com)
- UPSTREAM: 46371: reset resultRun on pod restart (sjenning@redhat.com)
- UPSTREAM: 46305: clear init container status annotations when cleared in
  status (sjenning@redhat.com)
- remove obsolete build ict ctrl; transfer legacy error unit tests to new ict
  ctrl (gmontero@redhat.com)
- Enable preliminary support for origin federation (marun@redhat.com)
- UPSTREAM: 46315: Fix provisioned GCE PD not being reused if already exists
  (mawong@redhat.com)
- Prefer secure connection during image pruning (miminar@redhat.com)
- UPSTREAM: 46323: Use beta annotation for fed etcd pvc storage class
  (marun@redhat.com)
- UPSTREAM: 46247: Enable customization of federation etcd image
  (marun@redhat.com)
- Add `request-timeout` val to `oc login` restclient (jvallejo@redhat.com)
- UPSTREAM: 46020: Enable customization of federation image (marun@redhat.com)
- UPSTREAM: 45496: fix pleg relist time (sjenning@redhat.com)
- admission config: support legacy admission configs without kind
  (stefan.schimanski@gmail.com)
- admission config: add failing unit tests (stefan.schimanski@gmail.com)

* Thu May 25 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.85-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 026b56c
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bc8c8494baa3fc6c16a8158dacf6a5103ece122d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7b5be5192ac6cb18265b1009a9b5091bbd160122 (dmcphers+openshiftbot@redhat.com)
- Use shared informers in image controllers (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  674bfa26638c617eadb4faea84a98ccb5a987102 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  49f82e8dce65fd143055c66b7efba2e8dcb5113a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  913018dbb47766f2612da63c2455938de0ed87a4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e3bb0a00c91f4c2f06aba841b442d17b4a34f2ba (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  955f83cd3b71d6cc619e74b5913745d54c62d66b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1fdfcb2d3eec1cb38b0a6baa12415de84b4aeda2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5755f73a66a63969b00e69cf9fe18f8bd65fff8a (dmcphers+openshiftbot@redhat.com)
- Give docker builders access to optimized image builds (ccoleman@redhat.com)
- UPSTREAM: containers/image: <carry>: Disable gpgme on windows/mac
  (ccoleman@redhat.com)
- React to changes to cache.Indexer (ccoleman@redhat.com)
- Use shared informers in the scheduler (ccoleman@redhat.com)
- Use informers for token controller (ccoleman@redhat.com)
- UPSTREAM: 45933: Use informers in scheduler / token controller
  (ccoleman@redhat.com)
- Cherry-pick should support branch argument on NO_REBASE (ccoleman@redhat.com)
- fix templateinstance SAR check (deads@redhat.com)
- make route strategy respect the full user (deads@redhat.com)
- switch to system:masters user for etcd test (deads@redhat.com)
- Disable RBAC bootstrap-roles post-start hook (stefan.schimanski@gmail.com)
- UPSTREAM: 45977: kuberuntime: report StartedAt regardless of container states
  (sjenning@redhat.com)
- junitreport integration test compatability with older versions of diff-utils
  (for Mac OSX) (bornemannjs@gmail.com)
- Don't allow deleted routes in resync list (pcameron@redhat.com)
- Add then resync then Delete cause Pop() panic (pcameron@redhat.com)
- UPSTREAM: 46299: Fix in-cluster kubectl --namespace override
  (andy.goldstein@gmail.com)
- Add an OPENSHIFT-ADMIN-OUTPUT-RULES chain for admins to use (danw@redhat.com)
- Set layer size whether it found in cache or not (obulatov@redhat.com)

* Wed May 24 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.84-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 1e51f34
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  da9b22af7370bdc2778f3a4057e4042ee1c6d893 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  be03a8d99aa5117b6dcf1188fc88ff9d5a727ad2 (dmcphers+openshiftbot@redhat.com)
- combine router template sections for edge and reencrypt routes
  (jtanenba@redhat.com)

* Wed May 24 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.83-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console e6acfff
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1801153c4a7406622f15938b62e59e2299b885b2 (dmcphers+openshiftbot@redhat.com)
- Make HAProxy's log format configurable (yhlou@travelsky.com)

* Wed May 24 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.82-1
- Enable leader election on endpoints for controllers (ccoleman@redhat.com)
- Add defaults and env control of the fin timeouts in the router
  (bbennett@redhat.com)

* Wed May 24 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.81-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 2171013
  (tdawson@redhat.com)
- bindata generation (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fb7e6b6f3c69f2f183547e6ba9dabb473a906199 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5c8746079bfae25c4e745508eb965c5761030e64 (dmcphers+openshiftbot@redhat.com)
- Populate user in subject access review correctly (jliggitt@redhat.com)
- Emit XFS volume statistics on failure (skuznets@redhat.com)
- change token scopes constant (deads@redhat.com)
- template broker should use SAR, not impersonation (jminter@redhat.com)

* Tue May 23 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.80-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console bf427d3
  (tdawson@redhat.com)
- fix parameter description (jfchevrette@gmail.com)
- UPSTREAM: 46246: Fix kubelet event recording (decarr@redhat.com)
- Unblacklist StateFul e2e tests that should work with correct watch cache
  sizes (stefan.schimanski@gmail.com)
- bump(github.com/openshift/origin-web-console):
  03832b75dbc09e0c67627d2cfafd706512330b99 (dmcphers+openshiftbot@redhat.com)
- Drop obsolete pod permissions from the dc controller (mkargaki@redhat.com)
- bump(github.com/openshift/origin-web-console):
  063afb062d8051a579534419d148c8febba4d6d5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  743688a1719db5cebf2a66d3a98e6bf0517f26c9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  79a3a8de72051cb13f6f81fd1dcce6d05b7be398 (dmcphers+openshiftbot@redhat.com)
- Use cluster IP for nip.io prefixing in extended tests (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  39a73ad163614be079e7ef7f519bf8aa4e11dd3e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  606cab0851f3f154656d33ef3ac1469ed8c98d4c (dmcphers+openshiftbot@redhat.com)
- Correctly remove containers and volumes on cleanup (skuznets@redhat.com)
- UPSTREAM: 46037: NS controller: don't stop deleting GVRs on error
  (andy.goldstein@gmail.com)
- UPSTREAM: 45304: increase the QPS for namespace controller (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5e47d864a44ea2d23cd19f3ace35638e2df6d2fa (dmcphers+openshiftbot@redhat.com)
- Fix minor typo (jminter@redhat.com)
- deploy: rewire deployment controllers initialization to use a controller init
  func (mfojtik@redhat.com)
- auth: add rbac roles for deployments (mfojtik@redhat.com)
- deploy: use correct client for deployer controller (mfojtik@redhat.com)
- deploy: rename deployment controller to deployer controller
  (mfojtik@redhat.com)
- deploy: automatically set ownerRef for hook pods when rollout fail
  (mfojtik@redhat.com)
- deploy: set background propagation policy for old deployment cleanup
  (mfojtik@redhat.com)
- deploy: fix the owner reference kind to be rc (mfojtik@redhat.com)
- rewire build controller initialization to use a controller init func
  (deads@redhat.com)
- move openshift controller roles to system:openshift:controller:*
  (deads@redhat.com)
- Make the Prometheus example a fully automated secure deployment
  (ccoleman@redhat.com)
- Annotate the router by default with prometheus scraping (ccoleman@redhat.com)
- Extended test for local name resolution (ccoleman@redhat.com)
- add nodejs6 centos imagestreamtag (bparees@redhat.com)
- dump git server logs on test failure (bparees@redhat.com)
- UPSTREAM: 46127: Return MethodNotSupported when accessing unwatcheable
  resource with ?watch=true (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c03a72c239ccd000b6eca9f9107301e79ddda66d (dmcphers+openshiftbot@redhat.com)
- openshift-sdn: ensure multicast rules are cleaned up when net namespace is
  deleted (dcbw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e80630f386de5dda387c500f14f7f18e03a8ff19 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  17fc52f48249d703a06be0111c47c204012b9426 (dmcphers+openshiftbot@redhat.com)
- Add proxy protocol status to reload script output (bbennett@redhat.com)
- UPSTREAM: 45741: Fix discovery version for autoscaling to be v1
  (sross@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7276266854df2223de5806d437262deeb21173b5 (dmcphers+openshiftbot@redhat.com)
- mark build->buildconfig ownerref as a controller (bparees@redhat.com)
- wait for builder service accounts before running build tests
  (bparees@redhat.com)
- Fix deployclient imports (andy.goldstein@gmail.com)
- delete refs to https://github/gabemontero (gmontero@redhat.com)
- Regenerate clientsets (andy.goldstein@gmail.com)
- UPSTREAM: 45835: client-gen: honor groupName overrides in customArgs
  (andy.goldstein@gmail.com)
- If the stream tag is not found, replace with a local tag reference
  (ccoleman@redhat.com)
- generated: completions (ccoleman@redhat.com)
- Add --dry-run to import-image and cleanup describe output
  (ccoleman@redhat.com)
- Add `oc set image-lookup` command (ccoleman@redhat.com)
- Allow pods and other Kube objects to easily reference imagestreams
  (ccoleman@redhat.com)
- Resolve ImageStreamTag reference properly (ccoleman@redhat.com)
- UPSTREAM: 41939: Add an AEAD encrypting transformer for storing secrets
  encrypted at rest. (vsemushi@redhat.com)
- Add lookupPolicy.local to ImageStream and ImageStreamTag
  (ccoleman@redhat.com)
- Fix prioritizing of semver equal tags (obulatov@redhat.com)
- UPSTREAM: 44068: Use Docker API Version instead of docker version
  (maszulik@redhat.com)
- UPSTREAM: google/cadvisor: 1639: Reduce cAdvisor log spam with multiple
  devices (decarr@redhat.com)
- bump(github.com/google/cadvisor): 2ddeb5f60e22d86c8d1eeb654dfb8bfadf93374c
  (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  33acb3cbfeb89857abaf3074b6104bbd62f78085 (dmcphers+openshiftbot@redhat.com)
- Fix extended networking service tests to retry updates on conflict
  (danw@redhat.com)
- Reorganize ClusterNetwork creating/updating/validating (danw@redhat.com)
- Abstract out ClusterNetwork-vs-cluster-objects test (danw@redhat.com)
- Abstract out ClusterNetwork-vs-local-networks test (danw@redhat.com)
- Move a stray bit of ClusterNetwork validation to right place
  (danw@redhat.com)
- UPSTREAM: 45940: apiserver: no Status in body for http 204
  (stefan.schimanski@gmail.com)
- Debugging help for annotation triggers (ccoleman@redhat.com)
- Enable the image trigger controller with policy (ccoleman@redhat.com)
- Init containers should be targets for triggers (ccoleman@redhat.com)
- Support annotation triggers in `oc set trigger` (ccoleman@redhat.com)
- Add a new generic image trigger controller (ccoleman@redhat.com)
- DeploymentTrigger test does not reset timer (ccoleman@redhat.com)
- Allow deployment instantiate to skip images (ccoleman@redhat.com)
- api: Allow instantiate to exclude some triggers (ccoleman@redhat.com)
- Remove debugging logs from scheduler component, not needed anymore
  (ccoleman@redhat.com)
- makefile: Generate OpenAPI in the right spot (ccoleman@redhat.com)
- Dump contents of etcd v3 using the etcdhelper tool (skuznets@redhat.com)
- Add dump action to etcd helper tool (mkhan@redhat.com)
- Fixes template objects describer (ffranz@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6f33bb421ddb24d37190608d2560169ed6df003c (dmcphers+openshiftbot@redhat.com)
- Change version check on jq in clear route status script (jtanenba@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fc9da4e0926fa04f8e284e8ea1b83023e74c767b (dmcphers+openshiftbot@redhat.com)
- Remove deprecated bash utility (mkargaki@redhat.com)
- Dockerfile updates for RPM installs (mkargaki@redhat.com)
- Reconfigure container manifests to install using RPMs (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d864f4a772de664c3a5bd76bf0d3a971f0a8939a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a8654d4bc227831b66e06deda57bedd4372fb505 (dmcphers+openshiftbot@redhat.com)
- master: override watch cache capacity with origin default
  (stefan.schimanski@gmail.com)
- UPSTREAM: 45403: apiserver: injectable default watch cache size
  (stefan.schimanski@gmail.com)
- Fix negative unavailableReplicas dc field (mkargaki@redhat.com)
- syntax fix in irule TCL code (rchopra@redhat.com)
- UPSTREAM: 45413: Extend timeouts in timed_workers_test (mkhan@redhat.com)
- Use OS_IMAGE_PREFIX over router-specific test env (mkargaki@redhat.com)
- (WIP) Cleanup policy for builds (cdaley@redhat.com)
- switch to policy watch (deads@redhat.com)
- add generated code for policy objects (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e234396ecba3b619dabeb9b92655e80b093c471e (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 45747: OwnerReferencesPermissionEnforcement ignores pods/status
  (decarr@redhat.com)
- UPSTREAM: 45826: prevent pods/status from touching ownerreferences
  (decarr@redhat.com)
- bump(github.com/openshift/origin-web-console):
  93586889805944a2fe0a64dd75a46cf033d1f888 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0abc00f650fbe5929eb3a49554422edd83292a02 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 45623: Don't attempt to make and chmod subPath if it already exists
  (mawong@redhat.com)
- UPSTREAM: <drop>: kube-apiserver must not start aggregator (deads@redhat.com)
- minimal reaction to aggregation refactors (deads@redhat.com)
- UPSTREAM: 43383: proxy to IP instead of name, but still use host verification
  (deads@redhat.com)
- UPSTREAM: 42911: combine kube-apiserver and kube-aggregator
  (deads@redhat.com)
- UPSTREAM: 42900: rewire aggregation handling chain to be normal
  (deads@redhat.com)
- UPSTREAM: 43076: allow combining API servers (deads@redhat.com)
- UPSTREAM: 43149: break kube-apiserver start into stages (deads@redhat.com)
- UPSTREAM: 43141: Create controller to auto register TPRs with the aggregator
  (deads@redhat.com)
- UPSTREAM: 43226: don't start controllers against unhealthy master
  (deads@redhat.com)
- UPSTREAM: 43144: start informers as a post-start-hook (deads@redhat.com)
- UPSTREAM: 42886: allow fallthrough handling from go-restful routes
  (deads@redhat.com)
- UPSTREAM: 42672: use separate scheme to serve the kube-aggregator
  (deads@redhat.com)
- UPSTREAM: 42801: add local option to APIService (deads@redhat.com)
- UPSTREAM: <drop>: missing test file (deads@redhat.com)
- UPSTREAM: 45286: When pods are terminated we should detach the volume
  (hekumar@redhat.com)
- Image signature verification worflow test (mfojtik@redhat.com)
- Make template service broker namespace(s) configurable (jminter@redhat.com)
- bump(github.com/containers/storage): 5cbbc6bafb45bd7ef10486b673deb3b81bb3b787
  (maszulik@redhat.com)
- bump(github.com/flynn/go-shlex): drop (maszulik@redhat.com)
- bump(github.com/mtrmac/gpgme): b2432428689ca58c2b8e8dea9449d3295cf96fc9
  (maszulik@redhat.com)
- bump(github.com/opencontainers/image-spec):
  00850eca2ab993e282a4921f8b7000b2fcbd26fa (maszulik@redhat.com)
- bump(github.com/opencontainers/go-digest):
  a6d0ee40d4207ea02364bd3b9e8e77b9159ba1eb (maszulik@redhat.com)
- bump(github.com/containers/image): f5768e7cbd2d715ea3f0153cd857699157d1f33a
  (maszulik@redhat.com)
- bump(golang.org/x/crypto): d172538b2cfce0c13cee31e647d0367aa8cd2486
  (maszulik@redhat.com)
- UPSTREAM: 45601: util/iptables: fix cross-build failures due to
  syscall.Flock() (dcbw@redhat.com)
- UPSTREAM: 44895: util/iptables: grab iptables locks if iptables-restore
  doesn't support --wait (dcbw@redhat.com)
- UPSTREAM: 43575: util/iptables: check for and use new iptables-restore 'wait'
  argument (dcbw@redhat.com)
- Increase networking test endpoint wait to 3 minutes (skuznets@redhat.com)
- remove references to personal repositories (bparees@redhat.com)
- only log stacks on server errors (deads@redhat.com)
- UPSTREAM: 43377: only log stacks on server errors (deads@redhat.com)
- switch clusterquotamapping to use the normal cache wait (deads@redhat.com)
- disambiguate directory and branch names via -- (gmontero@redhat.com)
- generated file (hchen@redhat.com)
- orphan resources by default for SOME resourced under /oapi (deads@redhat.com)
- Network policy pod watch should ignore pods with HostNetwork set to true
  (rpenta@redhat.com)
- Remove Nodes,Namespaces,Pods and Services resource names from SDN
  RunEventQueue() (rpenta@redhat.com)
- Network policy pod watch changes (rpenta@redhat.com)
- Change SDN watch services on node to reuse existing shared informer
  (rpenta@redhat.com)
- Change network policy watch namespaces to use shared informers
  (rpenta@redhat.com)
- Pass shared informers to node SDN controller (rpenta@redhat.com)
- Change SDN watch namespaces on master to reuse existing shared informer
  (rpenta@redhat.com)
- Change SDN watch nodes to reuse existing shared informer (rpenta@redhat.com)
- Pass shared informers to master SDN controller (rpenta@redhat.com)
- UPSTREAM: 43396: iSCSI CHAP support (hchen@redhat.com)
- add client for buildconfig (deads@redhat.com)
- use upstream initialization for most controllers (deads@redhat.com)
- fix the help text in the clear-route-status script (jtanenba@redhat.com)
- cli: do not require --expected-identity when removing all signatures
  (mfojtik@redhat.com)
- cli: show proper command name in verify-image-signature usage
  (mfojtik@redhat.com)
- Make revisionHistoryLimit test more resistant to failures
  (mkargaki@redhat.com)
- master: only one place for storage GVKs (stefan.schimanski@gmail.com)
- UPSTREAM: 43170: Add ability to customize fed namespace for e2e
  (marun@redhat.com)
- UPSTREAM: 44073: Optionally retrieve fed e2e cluster config from secrets
  (marun@redhat.com)
- UPSTREAM: 44066: Improve federation e2e test setup (marun@redhat.com)
- UPSTREAM: 44072: Cleanup e2e framework for federation (marun@redhat.com)
- Allow multiple destinations in egress-router (danw@redhat.com)
- Add egress-router unit test (danw@redhat.com)
- Reorg egress-router code, add "initContainer mode" (danw@redhat.com)
- Simplify egress-router routing by using a default route (danw@redhat.com)
- Harden egress-router a bit (danw@redhat.com)
- Only "set -x" in egress-router if EGRESS_ROUTER_DEBUG is set
  (danw@redhat.com)
- Fix bash coding style in egress-router.sh. (danw@redhat.com)
- Move egress-router image (danw@redhat.com)
- switch to use upstream remote authentication and authorization
  (deads@redhat.com)
- UPSTREAM: 45238: expose kubelet authentication and authorization builders
  (deads@redhat.com)
- generated: api changes and cli flags (ccoleman@redhat.com)
- Add a test case for reencrypt serving certs (ccoleman@redhat.com)
- Support router reencrypt using the serving cert CA (ccoleman@redhat.com)
- Allow controlling spec.host via a new permission (ccoleman@redhat.com)
- Update generated files (stefan.schimanski@gmail.com)
- Resurrect extensions/v1beta.HPA tests (stefan.schimanski@gmail.com)
- Autoscaling HPAs as primary group, extensions as cohabitation
  (stefan.schimanski@gmail.com)
- UPSTREAM: <drop>: resurrect extensions/v1beta1.HPA
  (stefan.schimanski@gmail.com)
- Image generated clients (maszulik@redhat.com)
- Mark image types that should have generated clients (maszulik@redhat.com)
- Add build step in make update before generating completions
  (maszulik@redhat.com)
- Revert "add: pkg/kubecompat/apis/extensions/v1beta1 to restore HPA in
  extensions/v1beta1" (stefan.schimanski@gmail.com)
- Remove unused client builders (stefan.schimanski@gmail.com)
- bump test coverage for generate/app (pweil@redhat.com)
- the arg 'id' isn't used in the func 'schema0ToImage', I think we should be
  deleted it (miao.yanqiang@zte.com.cn)

* Sat May 13 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.75-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 272d713
  (tdawson@redhat.com)
- Fix extended networking test that could potentially wait forever
  (danw@redhat.com)
- dockerhelper.Helper.DockerRoot(): don't swallow error. (vsemushi@redhat.com)
- synchronize origin authorization resources to rbac ones (deads@redhat.com)
- UPSTREAM: 44798: Cinder: Automatically Generate Zone if Availability in
  Storage Class is not Configured (ppospisi@redhat.com)
- bump(github.com/fatih/structs):v1.0 (ccoleman@redhat.com)
- UPSTREAM: 44760: Fix issue #44757: Flaky Test_AttachDetachControllerRecovery
  (mawong@redhat.com)
- UPSTREAM: 43289: Attach/detach controller: fix potential race in constructor
  (mawong@redhat.com)
- controller manager options not passed to quota (decarr@redhat.com)
- UPSTREAM: 45685: fix quota resync (decarr@redhat.com)
- Add projected volume plugin into correct SCCs (pmorie@redhat.com)
- adding X-Forwarded-For header to reencrypt route (jtanenba@redhat.com)
- UPSTREAM: 39732: Fix issue #34242: Attach/detach should recover from a crash
  (mawong@redhat.com)
- UPSTREAM: 44452: Implement LRU for AWS device allocator (mawong@redhat.com)
- UPSTREAM: 42033: fix TODO: find and add active pods for dswp
  (mawong@redhat.com)
- UPSTREAM: 44566: WaitForCacheSync before running attachdetach controller
  (mawong@redhat.com)
- UPSTREAM: 44781: Ensure desired state of world populator runs before volume
  reconstructor (mawong@redhat.com)
- DRY out script cleanup code (skuznets@redhat.com)
- Automatically determine which type of jUnit report to generate
  (skuznets@redhat.com)
- Disable swap in Go build (skuznets@redhat.com)
- ensure build start time is always set (bparees@redhat.com)
- cluster up: set docker cgroup driver on kubelet config (cewong@redhat.com)
- UPSTREAM: 45515: Ignore openrc group (cewong@redhat.com)
- add clients for roles and rolebindings (deads@redhat.com)
- Sanitize certificates from routes in the router (ccoleman@redhat.com)
- Hold startup until etcd has stabilized cluster version (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  01350df90ab02d53767fbcbf10a85027f5e7d079 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  862164ee901780063c9e5de26ab37c9d5af071ec (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  787e8dd90e35aa30639e60ac295dd05032caf380 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e7a16eb9d963ef00c3fba7d9807fef43b691167f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a90c5c804db9a36245bf973e5300bc5f86342d20 (dmcphers+openshiftbot@redhat.com)
- strip proxy credentials when logging proxy env variables (bparees@redhat.com)
- switch the RC controller to the upstream launch mechanism (deads@redhat.com)
- fix local resource output oc set (jvallejo@redhat.com)
- bump(github.com/gophercloud): b06120d13e262ceaf890ef38ee30898813696af0
  (hchen@redhat.com)
- UPSTREAM: 41498: cinder: Add support for the KVM virtio-scsi driver
  (hchen@redhat.com)
- UPSTREAM: 44082: use AvailabilityZone instead of Availability
  (hchen@redhat.com)
- bump(github.com/openshift/source-to-image):
  5d863bfc266dcae304ebd527b420c00cc9b08511 (bparees@redhat.com)
- Include DefaultTolerationSeconds admission plugin but off by default.
  (avagarwa@redhat.com)
- retry build instantiation and clone on conflict (jminter@redhat.com)
- UPSTREAM: 44639: Set fed apiserver to bind to 8443 instead of 443
  (marun@redhat.com)
- UPSTREAM: 45505: expose the controller initializers (deads@redhat.com)
- UPSTREAM: 44625: Retry secret reference addition on conflict
  (jliggitt@redhat.com)
- UPSTREAM: google/cadvisor: 1642: cAdvisor fs metrics should appear in
  /metrics (ccoleman@redhat.com)
- UPSTREAM: 40423: Support for v1/v2/autoprobe openstack cinder blockstorage
  (hchen@redhat.com)
- Force to specify not empty secret for metrics endpoint (agladkov@redhat.com)

* Tue May 09 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.73-1
- master: zero kubelet readonly port to match node
  (stefan.schimanski@gmail.com)
- Add test to notice upstream changes in group perferred versions
  (stefan.schimanski@gmail.com)
- Fix the OVS flow "note" to match 1.5 and earlier (danw@redhat.com)
- Dump events from e2e tests (mkargaki@redhat.com)

* Tue May 09 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.72-1
- 

* Mon May 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.71-1
- improve output of --list-pods (jvallejo@redhat.com)

* Mon May 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.70-1
- bump(github.com/openshift/origin-web-console):
  86ccb7ba30bee182b18de74f53f9ab2e8b129c98 (dmcphers+openshiftbot@redhat.com)

* Mon May 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.69-1
- start bootstrapping cluster roles from kube (deads@redhat.com)

* Mon May 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.68-1
- openshift-build: enable pr fetch from oc new-app, start-build, source lookup
  (gmontero@redhat.com)

* Sun May 07 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.67-1
- UPSTREAM: 44939: don't HandleError on container start failure
  (sjenning@redhat.com)
- switch to upstream x509 request header authenticator (deads@redhat.com)
- switch to upstream authentication/authorization types (deads@redhat.com)
- UPSTREAM: 45235: remove bearer token from headers after we consume it
  (deads@redhat.com)
- Use generic start utilities in test-cmd (skuznets@redhat.com)
- Stop using loopback address for API_HOST (skuznets@redhat.com)
- Start registry using administrative credentials (skuznets@redhat.com)
- Dry out start code to use `$ADMIN_KUBECONFIG` (skuznets@redhat.com)
- Allow for configuration of profiling with `$OPENSHIFT_PROFILE`
  (skuznets@redhat.com)
- Allow for using network plugins with `$NETWORK_PLUGIN` (skuznets@redhat.com)
- Reformatted Bash and cleaned up cruft in scripts (skuznets@redhat.com)

* Sat May 06 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.66-1
- Make integration test able to produce junitreport format (jhadvig@redhat.com)
- Wait for SA creation in etcd test (mkhan@redhat.com)
- Fix dumping registry disk usage in tests (andy.goldstein@gmail.com)
- display jenkins url for pipeline build (gmontero@redhat.com)
- <drop>: Allow basic fallback without hairpin (mkhan@redhat.com)
- GSSAPI test: wait longer for config change (mkhan@redhat.com)
- extended-tests: update rebase flake patterns (stefan.schimanski@gmail.com)
- fix build default import to v1 api (bparees@redhat.com)
- Add ability to provide additional rpmbuild-options to tito
  (alivigni@redhat.com)

* Fri May 05 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.65-1
- bump(github.com/openshift/origin-web-console):
  fd1527a4b40ad3bfda9643c1408f05052c7758e1 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8c82ca97414df7afd184cc385f5f5deda0ff7fd0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d4cf894195a22088ebf1206638ed055bba630cc5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7330c8e302ba171ed11e2849508f9186af9e582e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b9e9fde662aa602c1c85b6e14bab0e70fe9a51e0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  078c7308f5b838d16629ea37debcc6d8a8891f84 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  99ace08b7027802c038611b6aa82fc479347c776 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9138b0532860eab255d7edaf1a55691fc3f1ffa9 (dmcphers+openshiftbot@redhat.com)
- Don't show policy rules with attribute restrictions
  (andy.goldstein@gmail.com)
- deploy: suggest to cancel dc instead of rc in oc deploy (mfojtik@redhat.com)
- update generated docs and completion (mfojtik@redhat.com)
- image: add verify-image-signature command for image admins
  (mfojtik@redhat.com)
- Always build with -tags=containers_image_openpgp (mitr@redhat.com)
- bump(github.com/opencontainers/image-spec):
  00850eca2ab993e282a4921f8b7000b2fcbd26fa (mfojtik@redhat.com)
- bump(github.com/opencontainers/go-digest):
  a6d0ee40d4207ea02364bd3b9e8e77b9159ba1eb (mfojtik@redhat.com)
- bump(github.com/mtrmac/gpgme): b2432428689ca58c2b8e8dea9449d3295cf96fc9
  (mfojtik@redhat.com)
- bump(github.com/containers/storage): 5cbbc6bafb45bd7ef10486b673deb3b81bb3b787
  (mfojtik@redhat.com)
- bump(github.com/containers/image): c07f8fdceeda1517556602778a61ba94894e7c02
  (mfojtik@redhat.com)
- bump(golang.org/x/crypto): 1f22c0103821b9390939b6776727195525381532
  (mfojtik@redhat.com)
- Prune external images by default (miminar@redhat.com)
- allow SA tokens via websockets (deads@redhat.com)
- support for reencrypt routes in the same vserver (rajatchopra@gmail.com)

* Thu May 04 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.64-1
- bump(github.com/openshift/origin-web-console):
  9f15b0a9c9254fa81e9da7e1f8b517b6a3001496 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  513de5aaa7315ba8b21935fe90e4aae82bd1e773 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  51f517abc35cdf2280ecba4675143c78f5301e34 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9b9e9f8777c7b3e9da2ab58035dd667e93ca8c90 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: <carry>: Fix to avoid REST API calls at log level 2.
  (avagarwa@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b23d5ff49ffa5a29fce3865e1b1f287ac9aa15ba (dmcphers+openshiftbot@redhat.com)
- rely on the upstream namespace cleanup controller (deads@redhat.com)
- resolve merge conflict (li.guangxu@zte.com.cn)
- use the upstream system:masters authorizer (deads@redhat.com)
- UPSTREAM: <drop>: remove hacks for delaying post start hooks
  (deads@redhat.com)
- UPSTREAM: 44462: 44489: fix selfLink for cluster-scoped resources
  (andy.goldstein@gmail.com)
- Switch back to kapi instead of v1 for RCs in deployment describer
  (andy.goldstein@gmail.com)
- Bug 1445694 - Fix locking in syncEgressDNSPolicyRules() (rpenta@redhat.com)

* Wed May 03 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.63-1
- Update Makefile update/verify (andy.goldstein@gmail.com)
- Update clientset imports (andy.goldstein@gmail.com)
- Generate shared informers (andy.goldstein@gmail.com)
- Regenerate clientsets (andy.goldstein@gmail.com)
- Regenerate listers (andy.goldstein@gmail.com)
- Add geninformers (andy.goldstein@gmail.com)
- Tweak clientsets/listers packages, add lister verification
  (andy.goldstein@gmail.com)
- Don't use version name in external clientset (andy.goldstein@gmail.com)
- Remove OutputPackagePath default from genlisters (andy.goldstein@gmail.com)
- UPSTREAM: 45171: Use groupName comment for listers/informers
  (andy.goldstein@gmail.com)
- Rename all controller.go to be easily distinguishable in logs
  (maszulik@redhat.com)

* Tue May 02 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.62-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 09e89a7
  (tdawson@redhat.com)
- Ignore generated data during origin->ose merge (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  09e89a72b9f5c560494e27b9815958e5b729c27b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6bc131aac557f5f2afa66ea09173b26d0d348093 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  900019bc0d2f665d664c1c74e49053fd3d33fdef (dmcphers+openshiftbot@redhat.com)
- Remove clarified todos (stefan.schimanski@gmail.com)
- bump(github.com/openshift/origin-web-console):
  15d3a3d764397e6bfd41d184d5ccfc21ce7fe3c0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  66df3b8534cf2869f65a0e94792688c6a690fea2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  200c8cd0b9194e53ab237fcc988fd422398b5f63 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  36195366413134ad5defd1d7bcc0737c5f310397 (dmcphers+openshiftbot@redhat.com)
- Narrow Travis config to only verify commits (skuznets@redhat.com)
- Use docker image reference from ImageStream (obulatov@redhat.com)
- Remove GetFake*Handler (obulatov@redhat.com)
- Replace io.ReadSeeker by simple []byte buffer (obulatov@redhat.com)
- deploy: allow to trigger deployment when ICT is updated (mfojtik@redhat.com)

* Mon May 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.61-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 3619536
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  be96ed8488654b2e90eb9f5da540cfdfb9f73524 (dmcphers+openshiftbot@redhat.com)
- proxy/hybrid: add locking around userspace map (dcbw@redhat.com)

* Mon May 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.60-1
- 

* Mon May 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.59-1
- 

* Mon May 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.58-1
- bump(github.com/openshift/origin-web-console):
  03c7a392be32e5b2fa064dc4d247256b5af35d56 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: docker/docker: <carry>: WORD/DWORD changed (deads@redhat.com)
- UPSTREAM: 44970: CRI: Fix StopContainer timeout (stefan.schimanski@gmail.com)

* Mon May 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.57-1
- Add support for Node.js 6 (official) (sspeiche@redhat.com)

* Sun Apr 30 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.56-1
- Drop "dnf clean all" from dind image builds to try to fix a flake
  (danw@redhat.com)
- Fix etcdhelper imports (andy.goldstein@gmail.com)
- Remove Docker journal gathering steps (skuznets@redhat.com)
- Handle "origin" container like a k8s container on cleanup
  (skuznets@redhat.com)
- Set version tags for client-go (jliggitt@redhat.com)
- Fix TestConcurrentBuildPodController integration test (cewong@redhat.com)
- UPSTREAM: 44439: controller: fix saturation check in Deployments
  (mkargaki@redhat.com)

* Sat Apr 29 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.55-1
- bump(github.com/openshift/origin-web-console):
  53807d04bc6cc26a815bcbf44b39a69a6d064a53 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0fd4423a0211eeedb946b482ed1948ac7fa73610 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 45105: taint-controller-tests: double 'a bit of time' to avoid
  flakes (stefan.schimanski@gmail.com)
- UPSTREAM: 45100: node-controller: deflake TestUpdateNodeWithMultiplePods
  (stefan.schimanski@gmail.com)
- UPSTREAM: 41634: Handle error event type (yashulyak@gmail.com)
- Add synthetic Ginkgo foci for parallel and serial conformance suites
  (skuznets@redhat.com)
- make build event checking tolerant of latency (bparees@redhat.com)
- Resolve build hang when docker daemon under load (jminter@redhat.com)
- bump(github.com/openshift/source-to-image):
  191ae3b6e99e84bc76419e18d3086dc3a3d2a49d (jminter@redhat.com)
- Wait for service account token (jliggitt@redhat.com)
- Minor improvements. (vsemushi@redhat.com)
- debug logging for WaitForABuild failures (bparees@redhat.com)
- fix git server capacity to 1gi (bparees@redhat.com)

* Fri Apr 28 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.54-1
- syscontainers: add mount for /var/log to origin (gscrivan@redhat.com)
- enable builds with PR references (gmontero@redhat.com)
- UPSTREAM: 43375: Set permission for volume subPaths (pmorie@redhat.com)
- Fix NATting of external traffic with ovs-networkpolicy (danw@redhat.com)
- apply build resource defaults directly to the build pod (bparees@redhat.com)

* Thu Apr 27 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.53-1
- disable image-references (deads@redhat.com)
- Ignore two upstream e2e flakes for now (stefan.schimanski@gmail.com)
- Fix TestBootstrapClusterRoles unit test (stefan.schimanski@gmail.com)
- Run make-update (stefan.schimanski@gmail.com)
- UPSTREAM: 44730: Check for terminating Pod prior to launching successor in
  StatefulSet (stefan.schimanski@gmail.com)
- squash: pods PATCH permission for rc controller (stefan.schimanski@gmail.com)
- Disable cgroups-per-qos in dind (danw@redhat.com)
- fix: build type changes (deads@redhat.com)
- fix-test: hpa.{extensions -> autoscaling} in policy cmd test
  (stefan.schimanski@gmail.com)
- fix: clientconfig again, match master (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  521e60a07d38fb8421b6ee00104747e5a91c6701 (dmcphers+openshiftbot@redhat.com)
- hack/godep-save.sh: add kube version check and fail early
  (stefan.schimanski@gmail.com)
- UPSTREAM: <drop>: disable apiserver loopback loop in generic context
  (stefan.schimanski@gmail.com)
- sqash: fix sdn after rebase (stefan.schimanski@gmail.com)
- TO REVERT: fix SAR serialization (deads@redhat.com)
- skip RBAC resources for oadm migrate because of permission problems
  (deads@redhat.com)
- test-fix: run staging tests through vendor (stefan.schimanski@gmail.com)
- fix: switch scc review to external objects (deads@redhat.com)
- fix: decoder for set env (deads@redhat.com)
- Allow a large burst for discovery to reduce oc latency (deads@redhat.com)
- fix: deployment generator for external version (deads@redhat.com)
- UPSTREAM: opencontainers/runc: 1124: Ignore error when starting transient
  unit that already exists (decarr@redhat.com)
- fix-test: GC user in TestOldLocalResourceAccessReviewEndpoint
  (stefan.schimanski@gmail.com)
- fix-test: add GC user to globalDeploymentConfigGetterUsers
  (stefan.schimanski@gmail.com)
- fix test: allow daemon set, stateful set e2e tests to schedule to all nodes
  for gce testing (andy.goldstein@gmail.com)
- fix: appliedclusterresourcequota to be a getter again (deads@redhat.com)
- fix-test: test-cmd.sh (deads@redhat.com)
- fix-test: 'oc get builds' in extended build test (andy.goldstein@gmail.com)
- fix-test: extended deploymentconfig test issues (andy.goldstein@gmail.com)
- fix-test: Wait for openshift/ruby:latest ImageStreamTag before running build
  forcePull test (andy.goldstein@gmail.com)
- Enable garbage collector controller (andy.goldstein@gmail.com)
- fix-test: correct type cast in PodSucceeded for dns extended test
  (stefan.schimanski@gmail.com)
- fix-test: replace cakephp-mysql-{example -> persistent} in
  templateservicebroker tests (stefan.schimanski@gmail.com)
- fix: be fatal on DNS informer error and fix init order
  (stefan.schimanski@gmail.com)
- Fix ObjectRefernce type in unidler proxy signaler (sross@redhat.com)
- Remove "vendored" userspace proxy (sross@redhat.com)
- fix: oc idle: marshal versioned objects (sross@redhat.com)
- Default CNI conf/bin dirs correctly (andy.goldstein@gmail.com)
- fix: same eof newline on Mac and Linux (stefan.schimanski@gmail.com)
- fix: do not build before update in Makefile (stefan.schimanski@gmail.com)
- fix-test: use semantic equality in roundtrip tests like upstream
  (stefan.schimanski@gmail.com)
- rewrites: compile fixes for unit tests (stefan.schimanski@gmail.com)
- fix: buildpod controller (stefan.schimanski@gmail.com)
- adapt: use external kube client for events (agoldste@redhat.com)
- fix: workaround kube issue 44448 for buildconfiginstantiate
  (andy.goldstein@gmail.com)
- fix-test: wait until server is healthy (and therefore rbac post start hook is
  finished) (stefan.schimanski@gmail.com)
- fix: return Build, not Status in build webhook New()
  (stefan.schimanski@gmail.com)
- fix-test: node_auth_test needs to include ?output=1 for exec and attach now
  (andy.goldstein@gmail.com)
- fix: DefaultClientConfig: use kube cluster defaults, which fallback to
  localhost:8080 (andy.goldstein@gmail.com)
- fix-test: update extended test exclude list (stefan.schimanski@gmail.com)
- fix-test: adapt to Jobs removed from extensions (andy.goldstein@gmail.com)
- fix: versioned objects in 'oc set env' by default (andy.goldstein@gmail.com)
- fix: grammar in error string (stefan.schimanski@gmail.com)
- fix-test: ignore terminating registry pods in test-cmd.sh
  (agoldste@redhat.com)
- fix-test: avoid 'tput: invalid terminal' in core.sh in test-cmd.sh
  (stefan.schimanski@gmail.com)
- hack/build-{go,images}: include gcs+oss build tags as in build-cross.sh
  (stefan.schimanski@gmail.com)
- fix-test: add DockerShim root dir to NodeConfig to run int tests as non-root
  (andy.goldstein@gmail.com)
- fix-test: add DockerShimSocket to NodeConfig to run int tests as non-root
  (agoldste@redhat.com)
- adapt: match upstream pkg/proxy refactors (agoldste@redhat.com)
- fix: openapi generation script with staging dirs (agoldste@redhat.com)
- add: teach deployment HPA describer about new HPA (sross@redhat.com)
- fix: wait logic in WaitForServiceAccounts (agoldste@redhat.com)
- fix-test: switch storage extensions/v1beta1 migration test from jobs to HPA
  (agoldste@redhat.com)
- fix-test: add watch request param support to router http mock
  (agoldste@redhat.com)
- fix: panic on nil field/label selectors in router ListWatchers
  (agoldste@redhat.com)
- REVIEW: update: etcd_storage_path_test.go, with unfinished etcd3 support
  (agoldste@redhat.com)
- add: disabled etcd3 testing code and liveness check (agoldste@redhat.com)
- fix: install all kube apis and kubecompat (agoldste@redhat.com)
- add: pkg/kubecompat/apis/extensions/v1beta1 to restore HPA in
  extensions/v1beta1 (agoldste@redhat.com)
- REVIEW: fix: write versioned AdmissionConfiguration for admission plugins
  (stefan.schimanski@gmail.com)
- add: test cases in TestClusterResourceOverridePluginWithNoLimits
  (agoldste@redhat.com)
- fix-test: non-existing extensions/v1beta1 in
  TestExtensionsAPIDisabledAutoscaleBatchEnabled (stefan.schimanski@gmail.com)
- fix-test: update expected index in TestRootRedirect
  (stefan.schimanski@gmail.com)
- fix: do not install Docker image types, only register
  (stefan.schimanski@gmail.com)
- update: bootstrap policy (agoldste@redhat.com)
- Enforce etcd2 storage backend (stefan.schimanski@gmail.com)
- fix-test: update list of admission controllers that are off by default
  (agoldste@redhat.com)
- REVIEW: adapt: convert proxy & dns to shared informers (agoldste@redhat.com)
- add: new deployment cohabitation (stefan.schimanski@gmail.com)
- REVIEW: refactoring: merge storage factory code paths
  (stefan.schimanski@gmail.com)
- fix: start controllers in background and informers after controller init
  (agoldste@redhat.com)
- adapt: internal+external clients in controllers (stefan.schimanski@gmail.com)
- REVIEW: initial 1.6 port of master setup (stefan.schimanski@gmail.com)
- move: split pkg/cmd/server/kubernetes into master and node
  (stefan.schimanski@gmail.com)
- REVIEW: update: known kube apigroup versions in pkg/cmd/server/api
  (stefan.schimanski@gmail.com)
- fix: add 'oc auth', fix apply args (agoldste@redhat.com)
- REVIEW: adapt: pkg/cmd/cli/cmd/set printer changes (agoldste@redhat.com)
- adapt: pkg/cmd/cli/cmd/rollout signature fixes (agoldste@redhat.com)
- pkg/sdn/plugin: use upstream ConstructPodPortMapping
  (andy.goldstein@gmail.com)
- adapt: match pkg/sdn upstream refactors (agoldste@redhat.com)
- adapt: match pkg/quota to upstream refactors (agoldste@redhat.com)
- adapt: match pkg/quota/image upstream refactors (agoldste@redhat.com)
- adapt: pkg/cmd/util new upstream batch and hpa apigroups
  (stefan.schimanski@gmail.com)
- update: pkg/cmd/util/clientcmd generator list (stefan.schimanski@gmail.com)
- update: pkg/cmd/util/clientcmd security related restclient.Config fields
  (stefan.schimanski@gmail.com)
- fix-test: add SchedulerName and TerminationMessage* to container fixtures
  (stefan.schimanski@gmail.com)
- fix-test: upstream roundtrip framework in pkg/api/serialization_test
  (stefan.schimanski@gmail.com)
- fix-test: fuzzer panic in pkg/api/serialization_test
  (stefan.schimanski@gmail.com)
- add-test: legacy apigroup case for hpa test (stefan.schimanski@gmail.com)
- fix-test: update HPA test example versions after extensions/v1beta1 is gone
  (stefan.schimanski@gmail.com)
- fix-test: misc unit tests (stefan.schimanski@gmail.com)
- adapt: use SCC lister from kube in pkg/security (agoldste@redhat.com)
- adapt: pkg/dns: replace kendpoints.PodHostnamesAnnotation with proper field
  (agoldste@redhat.com)
- REVIEW: adapt: wrap controller manager AddFlags with our controller list
  (agoldste@redhat.com)
- adapt: pkg/deploy informers (stefan.schimanski@gmail.com)
- adapt: stop using a ShortcutExpander in OverwriteBootstrapPolicy
  (agoldste@redhat.com)
- REVIEW: adapt: remove OutputVersion helper calls in pkg/cmd/cli/cmd
  (agoldste@redhat.com)
- REVIEW: use version that handles internal api objects to determine effective
  scc (agoldste@redhat.com)
- adapt: f.PrinterForCommand instead of versioned printer (agoldste@redhat.com)
- adapt: request timeout and image pull progress deadline for
  docker.GetKubeClient (agoldste@redhat.com)
- add: internal kube informers in origin shared informer factory (needed for
  admission) (agoldste@redhat.com)
- remove: old api/protobuf-spec files (stefan.schimanski@gmail.com)
- remove: old clientsets (stefan.schimanski@gmail.com)
- remove: legacy kube informers (agoldste@redhat.com)
- adapt: upstream informers/listers for clusterquotamapping controller
  (agoldste@redhat.com)
- adapt: add GetOptions to non-generated client (stefan.schimanski@gmail.com)
- adapt: shims around auth registries for pointer List/GetOptions
  (stefan.schimanski@gmail.com)
- adapt: v1 secrets (stefan.schimanski@gmail.com)
- adapt: use InternalListWatch shim to translate ListOptions
  (stefan.schimanski@gmail.com)
- adapt: pass client to admission plugins via WantsInternalKubeClientSet
  (stefan.schimanski@gmail.com)
- REVIEW: adapt: pkg/cmd/util/clientcmd (agoldste@redhat.com)
- REVIEW TODOs: adapt: pkg/cmd/cli/describe printer updates
  (agoldste@redhat.com)
- adapt: pkg/api/kubegraph/analysis.FindHPASpecsMissingCPUTargets
  (stefan.schimanski@gmail.com)
- adapt: match upstream discovery type changes (stefan.schimanski@gmail.com)
- adapt: upstream store.CompleteWithOptions instead of ApplyOptions
  (agoldste@redhat.com)
- apigroups: remove double registration of meta types for internal versions
  (stefan.schimanski@gmail.com)
- apigroups: adapt to new 1.6 style (stefan.schimanski@gmail.com)
- apigroups: remove meta kinds (stefan.schimanski@gmail.com)
- adapt: event broadcasters (stefan.schimanski@gmail.com)
- Rewrites (agoldste@redhat.com)
- Fix hack scripts on Mac (stefan.schimanski@gmail.com)
- UPSTREAM: 00000: make AsVersionedObjects default cleanly (deads@redhat.com)
- UPSTREAM: <carry>: openapi test, patch in updated package name
  (deads@redhat.com)
- UPSTREAM: 44221: validateClusterInfo: use clientcmdapi.NewCluster()
  (deads@redhat.com)
- UPSTREAM: 44570: Explicit namespace from kubeconfig should override in-
  cluster config (deads@redhat.com)
- UPSTREAM: <carry>: add OpenShift resources to garbage collector ignore list
  (andy.goldstein@gmail.com)
- UPSTREAM: 44859: e2e: handle nil ReplicaSet in checkDeploymentRevision
  (stefan.schimanski@gmail.com)
- UPSTREAM: 44861: NotRegisteredErr for known kinds not registered in target GV
  (stefan.schimanski@gmail.com)
- UPSTREAM: <drop>: Run make-update – vendor/ changes
  (stefan.schimanski@gmail.com)
- UPSTREAM: google/cadvisor: 1639: Reduce cAdvisor log spam with multiple
  devices (decarr@redhat.com)
- UPSTREAM: docker/engine-api: 26718: Add Logs to ContainerAttachOptions
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: 2140: Add 'ca-central-1' region for registry
  S3 storage driver (mfojtik@redhat.com)
- UPSTREAM: opencontainers/runc: 1216: Fix thread safety of SelinuxEnabled and
  getSelinuxMountPoint (pmorie@redhat.com)
- UPSTREAM: coreos/go-systemd: 190: util: conditionally build CGO functions
  (agoldste@redhat.com)
- UPSTREAM: revert: bedff43594597764076a13c17b30a5fa28c4ea76: docker/docker:
  <drop>: revert: 734a79b: docker/docker: <carry>: WORD/DWORD changed
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: 2008: Honor X-Forwarded-Port and Forwarded
  headers (miminar@redhat.com)
- UPSTREAM: docker/distribution: 1857: Provide stat descriptor for Create
  method during cross-repo mount (jliggitt@redhat.com)
- UPSTREAM: docker/distribution: 1757: Export storage.CreateOptions in top-
  level package (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Update dependencies
  (agladkov@redhat.com)
- UPSTREAM: coreos/etcd: <carry>: vendor grpc v1.0.4 locally
  (agoldste@redhat.com)
- bump(*): Kubernetes 1.6.1 (stefan.schimanski@gmail.com)
- hack/copy-kube-artifacts.sh: add staging/*** for proto files
  (stefan.schimanski@gmail.com)
- hack/godep-save.sh: run godep-save twice to install test dependencies
  (stefan.schimanski@gmail.com)
- hack/godep-*.sh: workaround godep dependency issues
  (stefan.schimanski@gmail.com)
- hack/godep-*.sh: symlink staging dirs (stefan.schimanski@gmail.com)
- hack/godep-*.sh: bump godep to v79 (stefan.schimanski@gmail.com)
- Add backward compatibility for the old EgressNetworkPolicy "0.0.0.0/32" bug
  (danw@redhat.com)

* Thu Apr 27 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.52-1
- add build substatus (cdaley@redhat.com)

* Thu Apr 27 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.51-1
- bump(github.com/openshift/origin-web-console):
  7a3a32bd990d2f8f42b868d7b99a1186570c0260 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  373860fcf1dcea41d8036a79d7989023956b7362 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  90b035d6dc4ebb39f79e4b48476af45a058622a5 (dmcphers+openshiftbot@redhat.com)
- Match subpaths correctly when path contains trailing slash
  (jliggitt@redhat.com)
- Remove experimental `oc import docker-compose` (ccoleman@redhat.com)
- Add proper default certificate check (ichavero@redhat.com)
- Avoid printing cert message for already loaded route (ichavero@redhat.com)
- indicate uselessness of metrics container for haproxy (rchopra@redhat.com)
- Replace using of cat | grep by a single grep invocation.
  (vsemushi@redhat.com)
- cli help changes to indicate options that are not supported for F5 (yet)
  (rchopra@redhat.com)
- sdn: fix initialization order to prevent crash on node startup
  (dcbw@redhat.com)

* Wed Apr 26 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.50-1
- Upgrade golang-1.8 to 1.8.1 (ffranz@redhat.com)

* Tue Apr 25 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.49-1
- Perform network diagnostic checks if we are able to launch at least 50%% of
  test pods. (rpenta@redhat.com)
- Bug 1421643 - Fix network diagnostics timeouts (rpenta@redhat.com)

* Tue Apr 25 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.48-1
- bump(github.com/openshift/origin-web-console):
  dd6bb1c5e3bdcb8710abcb22c81e036d0826dba7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  29e01c2fa776510aa9088e44866cd2b3bfe0b9dc (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  caf3dcb7d3f73a67a8d88c70eb8f1d6f1e2beebf (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5968a9200ba59d70f8f453adb4ed2eb57fbe601f (dmcphers+openshiftbot@redhat.com)
- deploy: move reasons from util to types (mfojtik@redhat.com)
- Loop until backend metrics reaches threshold (ccoleman@redhat.com)
- Moved test output from /tmp to under _output (skuznets@redhat.com)
- deploy: use CancelledRolloutReason in failed progress condition when
  cancelled (mfojtik@redhat.com)
- handle setting triggers on BCs with no default ICT (bparees@redhat.com)
- Upgrade golang-1.7 to 1.7.5 (ffranz@redhat.com)
- allow GIT_SSL_NO_VERIFY to be set on build pods via build defaulter
  (bparees@redhat.com)
- Check multiple GVKs in AddObjectsToTemplate (andy.goldstein@gmail.com)
- Prefer legacy kinds (andy.goldstein@gmail.com)
- Hide storage-admin role by default (jliggitt@redhat.com)
- Add prometheus metrics for dockerregistry (agladkov@redhat.com)

* Mon Apr 24 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.47-1
- Update logging in our deployment controllers (mkargaki@redhat.com)

* Sun Apr 23 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.46-1
- 

* Sun Apr 23 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.45-1
- 

* Sat Apr 22 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.44-1
- 

* Sat Apr 22 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.43-1
- Work around an OSX xcode ld error temporarily (ccoleman@redhat.com)

* Fri Apr 21 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.42-1
- 

* Thu Apr 20 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.41-1
- bump(github.com/openshift/origin-web-console):
  328fd64e83762af68d13474799ba8a21949e24a8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  36649013813e88394c41a2644ceb1770a2d26cd1 (dmcphers+openshiftbot@redhat.com)

* Thu Apr 20 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.40-1
- bump(github.com/openshift/origin-web-console):
  6d7f002e1ed71be92dbde134be31dbea8188e15e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5479ebe6f24563359b13ff4ca3585ae27d022ca3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a09129460462348d2156a28a04f803bbc55846ab (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  805c67b6aff92f28eaf3878b9e426f5d038844c6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  75d1c5600fa3eb96d885699f49d9af304f99a3d3 (dmcphers+openshiftbot@redhat.com)
- Specify the default registry for the OSE images (skuznets@redhat.com)

* Wed Apr 19 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.39-1
- bump(github.com/openshift/origin-web-console):
  69c571a6844b2f11fcc749d75e7a5e0a1ceac6e7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  64bc893727bb44ba1fe44e6901abea9eae2be2a5 (dmcphers+openshiftbot@redhat.com)
- Remove shadowed variable checks from vetting script (skuznets@redhat.com)
- Increase max request size for HAProxy to be comparable to cloud LBs
  (ccoleman@redhat.com)

* Tue Apr 18 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.38-1
- bump(github.com/openshift/origin-web-console):
  e9be0a3d3fde2d1a2fa8c7125f313d4e3e5b435c (dmcphers+openshiftbot@redhat.com)

* Tue Apr 18 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.37-1
- bump(github.com/openshift/origin-web-console):
  a6c277c2e75903d0a91a0e76497c4b9fdb45e6ca (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3e2bab4cfe4cdb94c88bc072a7db0216b35077a0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6c0257867c68da506b49a3e42fd6476e035c2863 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  82ed8832b64650779ef76d5a7c4b32f63a86a7b9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b5de937c73d44a623cc5d5f37da1872dbc76ac4e (dmcphers+openshiftbot@redhat.com)

* Tue Apr 18 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.36-1
- bump(github.com/openshift/origin-web-console):
  470afd2c653015e37174234c426a1a32218ced3c (dmcphers+openshiftbot@redhat.com)
- use k8s helper to retrieve tls client config (bparees@redhat.com)
- Auto generated swagger spec/docs/protobuf/conversions (rpenta@redhat.com)
- Add bind-utils package to node image and origin spec. (rpenta@redhat.com)
- Drop all traffic and firewall rules if DNS resolver is not found for
  resolving a domain in egress network policy (rpenta@redhat.com)
- Update firewall rules for egress dns entries (rpenta@redhat.com)
- Fix existing egress bug: Synchronize operations on egress policies
  (rpenta@redhat.com)
- Update ovs rules for egress dns entries (rpenta@redhat.com)
- DNS handler for egress network policy (rpenta@redhat.com)
- Test cases for generic DNS helper (rpenta@redhat.com)
- Generic DNS helper that holds map of dns names and associated information.
  (rpenta@redhat.com)
- Added 'DNSname' field to EgressNetworkPolicyPeer object (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9143df076669593d524c97ac6fcc65eb4f545506 (dmcphers+openshiftbot@redhat.com)
- Use the correct warning log function in cleanup scripts (skuznets@redhat.com)

* Mon Apr 17 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.35-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console 9143df0
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6f1fe0a8dcaf18774515e841abb1db2775820cf6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1a6a6ef6bf49ad7a58c989af69d0182d4c479899 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  490e5c84dadc69bb4a4747c7abd2de333338d67d (dmcphers+openshiftbot@redhat.com)
- explain dir copy behavior for image sourcepath (bparees@redhat.com)

* Mon Apr 17 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.34-1
- bump(github.com/openshift/origin-web-console):
  115c0752655a3ba8bd30e391f01452bf64b0dccd (dmcphers+openshiftbot@redhat.com)
- Add debugging to the stacktrace dump (skuznets@redhat.com)

* Mon Apr 17 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.33-1
- 

* Sun Apr 16 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.32-1
- Fix service IP validation to handle "ClusterIP: None" (danw@redhat.com)

* Sat Apr 15 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.31-1
- Prevent the router from deadlocking itself when calling Commit()
  (bbennett@redhat.com)

* Fri Apr 14 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.30-1
- bump(github.com/openshift/origin-web-console):
  dc92a52a0a1e672266a755b39606a8c2695bd1d5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8f396125d34834f9537bd3247f599b901b9ece2b (dmcphers+openshiftbot@redhat.com)
- don't read buildenv from stdin twice (bparees@redhat.com)
- Ignore SELinux attributes when creating release tarball (skuznets@redhat.com)
- Changelog generation not handling versions correctly (ccoleman@redhat.com)
- ignore namespace when processing templates (bparees@redhat.com)

* Thu Apr 13 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.29-1
- bump(github.com/openshift/origin-web-console):
  dd72cfb1df52ff444913da6a2fe21f4db25fc317 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  aa4a228dfad2ce4af2f13a4eb0a8e5e495ad3c80 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1cf9f825df8ba6f06a45ce9530d9b43092a9dd8a (dmcphers+openshiftbot@redhat.com)
- Clean up a misleading comment in the ratelimiter code (take 2)
  (bbennett@redhat.com)
- UPSTREAM: google/cadvisor: 1639: Reduce cAdvisor log spam with multiple
  devices (decarr@redhat.com)
- Drop e2e OVS test since it's now redundant (danw@redhat.com)
- Move pod-related flow logic into ovsController (danw@redhat.com)
- Move multicast flow logic into ovsController, and fix a bug (danw@redhat.com)
- Move EgressNetworkPolicy flow logic into ovsController (danw@redhat.com)
- Split out a unit-testable ovsController type in pkg/sdn/plugin
  (danw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a230883459f8e19525d96f1105e11dc222c17a5f (dmcphers+openshiftbot@redhat.com)
- Add a mock implementation of ovs.Interface for testing (danw@redhat.com)
- Change ovs.Interface, ovs.Transaction from structs to interfaces
  (danw@redhat.com)
- Clean up a misleading comment in the ratelimiter code (bbennett@redhat.com)
- Prevent new project creation with openshift/kubernetes/kube prefixes
  (jliggitt@redhat.com)

* Wed Apr 12 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.28-1
- SelfSubjectAccessReview does not authorize with api groups
  (ccoleman@redhat.com)
- Move audit middleware to separate module (agladkov@redhat.com)
- generate build state events (bparees@redhat.com)
- Use WaitAndGetVNID() instead of GetVNID() when initializing network policy
  plugin on the node (rpenta@redhat.com)
- add reference to build as a label on built images (bparees@redhat.com)
- fix broken make-run (xiaoping378@163.com)
- Remove unnecessary "!=" comparison (supermouselyh@hotmail.com)
- update load-etcd for new client (deads@redhat.com)

* Tue Apr 11 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.27-1
- bump(github.com/openshift/origin-web-console):
  a172029b6d375f52775f7b3d5b064f9a94c31fe4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  428eb00293c69f662936dee9a865fb455019213e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  448a8b3205c5aa83a6510e109f8c50097898059e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1cfff7844eecd016adaf0b3e255dbf147076180c (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5ec55aba0928e896f611b2a8afe29c370afe11dd (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5c07e9658690473f04528919d0ea537a25d2a121 (dmcphers+openshiftbot@redhat.com)
- oadm policy reconcile-sccs: update comments and help text.
  (vsemushi@redhat.com)
- Expand configuration options of the dind tool (jtanenba@redhat.com)
- Fix for bz1438402 (rchopra@redhat.com)
- BZ1429398 removed the call to index from jq (jtanenba@redhat.com)
- Bug 1433244 - allow imageimports to be long runing requests
  (maszulik@redhat.com)

* Mon Apr 10 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.26-1
- 

* Mon Apr 10 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.25-1
- added debug for image_ecosystem tests (gmontero@redhat.com)
- Updated `find` invocation not to use OSX-specific flags (skuznets@redhat.com)
- Migrate to use openshift-origin36 repo from openshift-future
  (skuznets@redhat.com)

* Sun Apr 09 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.24-1
- ensure next build is kicked off when a build completes (bparees@redhat.com)
- Enumerated the possible build phase states in the comment
  (skuznets@redhat.com)
- Stop forcing a re-build when updating generated completions
  (skuznets@redhat.com)
- RestrictUsersAdmission: allow SA with implicit NS (miciah.masters@gmail.com)
- openvswitch: update system container to oci 1.0.0-rc5 (gscrivan@redhat.com)
- origin: update system container to oci 1.0.0-rc5 (gscrivan@redhat.com)
- node: update system container to oci 1.0.0-rc5 (gscrivan@redhat.com)

* Sat Apr 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.23-1
- Restrict packages from CentOS to OVS only (skuznets@redhat.com)

* Fri Apr 07 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.22-1
- bump(github.com/openshift/origin-web-console):
  5456efafdb0b7e77c1d8b57c9a167cd58aa91f4b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3483dc3fbfc031b0e7e9c40919af382d1efb3810 (dmcphers+openshiftbot@redhat.com)
- Removing i386 build (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5911991e62d7fadb42ae975a7101083ac06d7a86 (dmcphers+openshiftbot@redhat.com)
- Install OpenVSwitch from the CentOS PaaS SIG Repos (skuznets@redhat.com)
- Streamline cluster up output (cewong@redhat.com)

* Fri Apr 07 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.21-1
- bump(github.com/openshift/origin-web-console):
  837875079457b94b47222641ae6a5f215326e0a5 (dmcphers+openshiftbot@redhat.com)
- Fix image pruning with both strong & weak refs (agoldste@redhat.com)
- Validate that SDN API object CIDRs are in canonical form (danw@redhat.com)
- Warn at master startup if cluster/service CIDR is mis-specified
  (danw@redhat.com)

* Fri Apr 07 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.20-1
- Stop depending on `$BASETMPDIR` for `os::cmd` tempfiles (skuznets@redhat.com)
- Build pod controller integration test - add debug (cewong@redhat.com)
- Handle RPM version calculations correctly when we're on a tag
  (skuznets@redhat.com)
- Update style of temp dir cleanup function (skuznets@redhat.com)

* Fri Apr 07 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.19-1
- Revert "Increase journald rate limiter when running test-end-to-end-
  docker.sh" (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1268da031a08bd67c377cb25bbd315339007f15c (dmcphers+openshiftbot@redhat.com)
- Add gpgme and libassuan to release dockerfile (mfojtik@redhat.com)
- deploy: add missing failure trap (mfojtik@redhat.com)
- Fix tags test to ignore pointers to list (mfojtik@redhat.com)
- Update swagger (mfojtik@redhat.com)
- Add AllowedRegistriesForImport to allow whitelisting registries allowed for
  import (mfojtik@redhat.com)
- Re-enable e2e tests that were failing due to #12558 (maszulik@redhat.com)
- Increase journald rate limiter when running test-end-to-end-docker.sh
  (maszulik@redhat.com)
- Cleanup impersonating code (mkhan@redhat.com)
- deployment: carry over the securityContext from the deployment config to
  lifecycle hook (mfojtik@redhat.com)
- deploy: add owner reference to rc from the deployer (mfojtik@redhat.com)
- Fix the typo of FatalErr (yu.peng36@zte.com.cn)

* Thu Apr 06 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.18-1
- cluster up: use routing suffix for router certificate hostnames
  (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  722fec3e194df2a9aa507a0adb605abc4530c3a2 (dmcphers+openshiftbot@redhat.com)
- Bug 1435588 - Forbid creating aliases across different Image Streams.
  (maszulik@redhat.com)
- Fix govet (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image):
  0765440c43e9622d42cf17f452c751a416e84fc4 (bparees@redhat.com)
- better error on invalid types (bparees@redhat.com)
- registry: add --fs-group and --supplementary-groups to oc adm registry
  (mfojtik@redhat.com)
- deploy: use patch for pausing and resuming deployment config
  (mfojtik@redhat.com)
- Update instructions to explicitly tag an image from docker repository
  (maszulik@redhat.com)

* Wed Apr 05 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.17-1
- Extended tests for router metrics should skip when not configured
  (ccoleman@redhat.com)
- Add pruning example using the CronJob (mfojtik@redhat.com)
- Fix go tests; policy requests now include the partition paths
  (rchopra@redhat.com)
- Used shared informer in BuildPodController and BuildPodDeleteController
  (cewong@redhat.com)
- Update OAuth grant flow tests (jliggitt@redhat.com)
- Redirect to relative subpath for approval, relative parent path on success
  (jliggitt@redhat.com)
- policy urls should support partitions (rchopra@redhat.com)
- F5 router partition path changes:   o Use fully qualified names so that there
  is no defaulting to /Common     and need to have all the referenced objects
  in the same partition,     otherwise F5 has reference errors across
  partitions.   o Fix policy partition path + rework and ensure we check the
  vserver     which is inside the partition we are configured in.   o Comment
  re: delete errors.   o Bug fixes.   o F5 unit test changes for supporting
  partition paths. (smitram@gmail.com)

* Tue Apr 04 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.16-1
- bump(github.com/openshift/origin-web-console):
  94c8d8999c237310857d7d9b8400b777a561152e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e92546776a3efb4f026403173f6ea036fbca412d (dmcphers+openshiftbot@redhat.com)
- support for gitlab and bitbucket webhooks (gmontero@redhat.com)

* Mon Apr 03 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.15-1
- mark jenkins v1 image deprecated (bparees@redhat.com)

* Mon Apr 03 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.14-1
- 

* Mon Apr 03 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.13-1
- Update retries to a saner count (mkargaki@redhat.com)

* Sun Apr 02 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.12-1
- Don't eat the newline at the end of the config (ccoleman@redhat.com)
- Print version of tests in extended (ccoleman@redhat.com)
- generated: completions (ccoleman@redhat.com)
- Add a test suite that verifies the router metrics (ccoleman@redhat.com)
- UPSTREAM: 42959: Delete host exec pods faster (ccoleman@redhat.com)
- Track reload and config write times in the router (ccoleman@redhat.com)
- Add metric labels that map to the API for haproxy servers
  (ccoleman@redhat.com)
- Use ':' as a name separate in the router (ccoleman@redhat.com)
- Expose prometheus metrics for the router by default (ccoleman@redhat.com)
- cluster up: set DNS bind and IP address for newer server versions
  (cewong@redhat.com)

* Sat Apr 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.11-1
- UPSTREAM: 43762: refactor getPidsForProcess and change error handling
  (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b3f64f85afaed6cb8b09fbea69a0ab37889c31ad (dmcphers+openshiftbot@redhat.com)
- Adding generic build failed reason (cdaley@redhat.com)

* Fri Mar 31 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.10-1
- Ratchet AttributeRestrictions validation (mkhan@redhat.com)
- Added tests for preventing pulling image 'scratch' (rymurphy@redhat.com)
- Add architecture type to the os::build::rpm::format_nvr function
  (jhadvig@redhat.com)
- revert SCMAuth: use local proxy when password length exceeds 255 chars
  (jminter@redhat.com)
- Update namespace finalizer to delete RoleBindingRestrictions
  (mkhan@redhat.com)
- deploy: retry scaling when the admission caches are not fully synced
  (mfojtik@redhat.com)

* Thu Mar 30 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.9-1
- ext tests: do not modify client namespace when dumping registry logs
  (cewong@redhat.com)
- updates, tests, for new jenkins log annotations (gmontero@redhat.com)

* Wed Mar 29 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.8-1
- bump(github.com/openshift/origin-web-console):
  c402c32530f2c5a6087be32838ed778c968434e2 (dmcphers+openshiftbot@redhat.com)
- extended: dump registry logs (miminar@redhat.com)
- Retry pending deployments longer before failing them (mkargaki@redhat.com)
- Use correct PEM header (jliggitt@redhat.com)

* Tue Mar 28 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.7-1
- fix extra lines in new-app output (gmontero@redhat.com)
- fix unbound variable error in build-images.sh (bparees@redhat.com)
- address redundant line if new-app error output (gmontero@redhat.com)
- Port openshift-sdn-ovs script to go (danw@redhat.com)
- Add ovsdb-manipulating methods to pkg/ovs (danw@redhat.com)

* Mon Mar 27 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.6-1
- UPSTREAM: 37380: Improve error reporting in Ceph RBD provisioner
  (jsafrane@redhat.com)

* Sun Mar 26 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.5-1
- 

* Sat Mar 25 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.4-1
- bump(github.com/openshift/origin-web-console):
  dce22ee6252dea8426fd4eedee06c71600971818 (dmcphers+openshiftbot@redhat.com)
- template service broker: use cakephp-mysql-example, not ruby-helloworld-
  sample, for tests (jminter@redhat.com)
- rename requestor to requester (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8f4cda159d1382b7e5e2d4c3223b4ac248be6e1f (dmcphers+openshiftbot@redhat.com)
- Improvements to templateservicebroker security: return forbidden clearly and
  try to avoid creating objects if a forbidden error is likely
  (jminter@redhat.com)
- change the router eventqueue key function (jtanenba@redhat.com)
- add extended test for jenkins bc with env vars (gmontero@redhat.com)
- add json schema parameters to service catalog (jminter@redhat.com)
- bump(github.com/lestrrat/go-jsschema):
  a6a42341b50d8d7e2a733db922eefaa756321021 (jminter@redhat.com)
- use console.openshift.io/iconClass (jminter@redhat.com)
- use kapiv1 in pkg/template/api/v1/register.go, enables correct serialisation
  of (for example) DeleteOptions (jminter@redhat.com)
- remove accidentally committed file (jminter@redhat.com)
- Add tests for ManifestService (obulatov@redhat.com)
- Add stateful reactors for fake client (obulatov@redhat.com)
- Add function newTestRepository (obulatov@redhat.com)
- Simplify code (obulatov@redhat.com)
- React to ginkgo changes (ccoleman@redhat.com)
- bump(github.com/onsi/ginkgo):v1.2.0-95-g67b9df7 (ccoleman@redhat.com)
- Extract function getLimitRangeList (obulatov@redhat.com)

* Fri Mar 24 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.3-1
- bump(github.com/openshift/origin-web-console):
  3e1266172305c090cab971dd17868c2d5a6ec8d1 (dmcphers+openshiftbot@redhat.com)
- hack/verify-upstream-commits.sh: take into account pkg/build/vendor
  directory. (vsemushi@redhat.com)
- use secret refs for redis password value so it is not exposed on the console
  (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4d46b4723de1668d533e037ded8f24660448b6dd (dmcphers+openshiftbot@redhat.com)
- Let the name of the RPM package being built vary (skuznets@redhat.com)
- mark Image type +nonNamespaced=true (jminter@redhat.com)
- UPSTREAM: <carry>: add SeccompProfiles to
  SecurityContextConstraintsDescriber. (vsemushi@redhat.com)
- fix wrong comment for check target (li.guangxu@zte.com.cn)
- make test names static (bparees@redhat.com)
- Revert "Handle the edge cases where an eventqueue method panics "
  (knobunc@users.noreply.github.com)
- Remove MessageContext in favor of authorizer.Attributes (mfojtik@redhat.com)
- auth: move apiGroup to be a suffix in error messages (mfojtik@redhat.com)

* Thu Mar 23 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.2-1
- Break out RPM version logic from RPM build script (skuznets@redhat.com)
- Template service broker (jminter@redhat.com)
- Template service broker API (jminter@redhat.com)
- generated clients (deads@redhat.com)
- UPSTREAM: <carry>: update clientset generator for openshift groups
  (deads@redhat.com)
- enable generation of normal clientsets (deads@redhat.com)
- node: system container mounts /rootfs rslave (gscrivan@redhat.com)
- Add tests for optimized builds that check permissions and behavior
  (ccoleman@redhat.com)
- Create a new synthetic ACL for docker optimization (ccoleman@redhat.com)
- Allow skipping image builds when the binaries are already created
  (ccoleman@redhat.com)
- Be more restrictive when copying secrets into Docker builds
  (ccoleman@redhat.com)
- Use 0755 in image source nested directory permissions (ccoleman@redhat.com)
- UPSTREAM: openshift/source-to-image: 711: Incorrect image user order
  (ccoleman@redhat.com)
- Addition of a pointer to string breaks tags_test (ccoleman@redhat.com)
- Suppress excessive logging from environment (ccoleman@redhat.com)
- Use imagebuilder to satisfy the imageOptimizationPolicy field
  (ccoleman@redhat.com)
- Add image optimization policy API to builds (ccoleman@redhat.com)
- Copy pkg/util/dockerfile into build tree (ccoleman@redhat.com)
- bump(github.com/openshift/imagebuilder):1c70938feddeb3ef9368091726e3c8a662dd7
  ac5 (ccoleman@redhat.com)
- Drop `oc ex dockerbuild` (ccoleman@redhat.com)
- SDN egress policy should not firewall endpoints from global namespaces
  (rpenta@redhat.com)
- Egress Network policy fixes (rpenta@redhat.com)

* Wed Mar 22 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.6.1-1
- Merge remote-tracking branch enterprise-3.6, bump origin-web-console ce66b15
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ce66b159b7c61a7ac05854401fda697d0210bc58 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8131638c4fa7d689002628b074dcbf1dfff0f740 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ed420f1f69211fd919b0da9ad6f01ecd9c7cbfff (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  697e8e8a216349296dbf8f2ad469e79345eb829c (dmcphers+openshiftbot@redhat.com)
- Lock down the legacy Origin v1 API (mfojtik@redhat.com)
- Set Git configuration for all users in release containers
  (skuznets@redhat.com)
- migrate.sh can flake if a namespace is still being deleted
  (ccoleman@redhat.com)
- Fix oc get rolebindingrestrictions formatting (miciah.masters@gmail.com)
- bump(gihtub.com/fatih/structs):v1.0 (ccoleman@redhat.com)
- Allow platforms to be sub-selected (ccoleman@redhat.com)
- UPSTREAM: docker/docker: <carry>: WORD/DWORD changed (ccoleman@redhat.com)
- Make image prefix configurable in e2e tests (mkargaki@redhat.com)
- Disallow AttributeRestrictions in PolicyRules (mkhan@redhat.com)
- fix mount propagation on rootfs for containerized node (sjenning@redhat.com)
- In the edge cases where an eventqueue method panics, don't leave the router
  running with a thread that will never update. Kill the router instead and let
  it get restarted by the kubelet. (smitram@gmail.com)

* Wed Mar 22 2017 Troy Dawson <tdawson@redhat.com> 3.6.0-1
- enable release repo (jhadvig@redhat.com)
- Dump container logs on `$TEST_ONLY` extended runs (skuznets@redhat.com)
- Rename router_stress_test.go -> router_without_haproxy_test.go
  (marun@redhat.com)
- router: Add integration test for namespace sync (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7f8960af97f07793c657f9f64b1b3d6fc1c5e988 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7805cf7ca4f735908fcb886c2b9617d55c308c22 (dmcphers+openshiftbot@redhat.com)
- cli: fix bulk generator to prefer legacy group (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f6b426173dac1d2eff0221aee724e2f504b5f161 (dmcphers+openshiftbot@redhat.com)
- Make NetworkPolicy tests use IPs rather than DNS names (danw@redhat.com)
- Port NetworkPolicy test to unversioned APIs. (danw@redhat.com)
- Wrap whole networkpolicy test in InNetworkPolicyContext() (danw@redhat.com)
- Import NetworkPolicy test (unchanged) from upstream PR (danw@redhat.com)
- Add infrastructure for NetworkPolicy support to extended networking tests
  (danw@redhat.com)
- Make cni_vendor_test.sh set NETWORKING_E2E_ISOLATION to false by default
  (danw@redhat.com)
- Make cni_vendor_test.sh not use NETWORKING_E2E_MINIMAL (danw@redhat.com)
- Make the networking e2e env var for isolation match the other vars
  (danw@redhat.com)
- Handle the case where no `$PATH` prefix is necessary (skuznets@redhat.com)
- simply use go tempdir function instead create BASETMPDIR
  (li.guangxu@zte.com.cn)
- Use bindata instead of files for fixtures in extended test suites
  (ccoleman@redhat.com)
- Generalize bindata to tests and bootstrapping (ccoleman@redhat.com)
- Tolerate getting content from fixture paths via permissions or init
  (ccoleman@redhat.com)
- Don't expect `chcon` on a Mac (ccoleman@redhat.com)
- Remove remaining references to EXTENDED_TEST_PATH (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d87ad15c8a453bdd118d83b24b2babae27f44e45 (dmcphers+openshiftbot@redhat.com)
- Removed unnecessary variables for common and PEP8 validations
  (ravisantoshgudimetla@gmail.com)
- Removed unnecessary variables for common (ravisantoshgudimetla@gmail.com)
- Fix eventqueue to return watch.Modified for resynced events
  (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ea891ffd5825ad13cd1cefeebf6fccc82bc6ff45 (dmcphers+openshiftbot@redhat.com)
- openshift binary not required for TEST_ONLY extended (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1615199ed8ddad0032416c209c9d87fc8868d297 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cf387f591228e6e490b5b440d72e5ea38ef941cb (dmcphers+openshiftbot@redhat.com)
- Add conversions from RBAC resources to origin resources (mkhan@redhat.com)
- Update Covers to handle AttributeRestrictions (mkhan@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b3337f4c4cc1c7f81851b22427151a9e06d671bf (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ffa2a4b0141a4d81ccb6c21f8ee5aa864891f403 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  242737ae65ac2d0195a134626b4791a33f851c6d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  693bb77a50f8cbf07f0c0a0f1d11b5a5a2eb5a85 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ffa3e9b39d34ddaad512eae269badd357501132f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ce35daaa1c546b74161e15fc4a19c028fbb8d631 (dmcphers+openshiftbot@redhat.com)
- Update Covers to handle AttributeRestrictions (mkhan@redhat.com)
- Dump Jenkins build log for pipeline builds (cewong@redhat.com)
- add retry action of pulling image (li.guangxu@zte.com.cn)
- image: mutate group admission attributes to ensure grouped resources are
  captured (mfojtik@redhat.com)
- Add tito releaser conf for 3.6 (smunilla@redhat.com)
- api groups boring changes (mfojtik@redhat.com)
- api groups interesting changes (mfojtik@redhat.com)
- fix test/cmd tests to support both legacy and api groups (mfojtik@redhat.com)
- fix integration tests to support both legacy and api groups
  (mfojtik@redhat.com)
- fix unit tests to support both legacy and api groups (mfojtik@redhat.com)
- api: update swagger and protobuf to support api groups (mfojtik@redhat.com)
- api: register and enable Origin API groups (mfojtik@redhat.com)
- UPSTREAM: <drop>: add appliedclusterresourcequotas to
  ignoredGroupVersionResources in namespace controller (mfojtik@redhat.com)
- test Base Dir should be create if not exist (li.guangxu@zte.com.cn)
- bump(github.com/openshift/origin-web-console):
  264b265131a4ee4f4565d37ee22ff86b646dd880 (dmcphers+openshiftbot@redhat.com)
- Add list events permission to pv-provisioner clusterrole (mawong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  de1b4592ea8f145186189923706c4df04c7dfe72 (dmcphers+openshiftbot@redhat.com)
- Fix race between ovsdb-server.service and node service (sdodson@redhat.com)
- Made build webhooks return new Build name. (rymurphy@redhat.com)
- docs: bump Go requirement from 1.6.x to 1.7.x in contributing
  (mfojtik@redhat.com)
- LOG_DIR may not exist as a dir (ccoleman@redhat.com)
- Only provide a fake HOME dir in a few tests (ccoleman@redhat.com)
- Switch all tests to use their default artifact dir (ccoleman@redhat.com)
- Use tmpdirs based only on script name (ccoleman@redhat.com)
- Separate tmpdir_vars from server setup, reorganize extended (again)
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  136be5582ecd2b4185ba94348be9623035962572 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4bf39056e8469e8b40b01b8a4338f238dfd4e685 (dmcphers+openshiftbot@redhat.com)
- Remove annoying diff with ose (mkargaki@redhat.com)
- Correct anyuid/restricted SCCs descriptions. (vsemushi@redhat.com)
- Fixed data race in registry unit tests (miminar@redhat.com)
- Fixed blobdescriptorservice unit test flake (miminar@redhat.com)
- handle unlimited import rate settings (pweil@redhat.com)
- origin: provide tmpfiles.template for system container (gscrivan@redhat.com)
- openvswitch: make syscontainer CONFIG_DIR configurable (gscrivan@redhat.com)
- node: make syscontainer CONFIG_DIR and DATA_DIR configurable
  (gscrivan@redhat.com)
- origin: make syscontainer CONFIG_DIR and DATA_DIR configurable
  (gscrivan@redhat.com)
- openvswitch: make system container docker dependency configurable
  (gscrivan@redhat.com)
- node: make docker and openvswitch dependencies configurable
  (gscrivan@redhat.com)
- origin: system container dependency to etcd and start before node
  (gscrivan@redhat.com)
- openvswitch: provide tmpfiles.template file (gscrivan@redhat.com)
- openvswitch: syscontainers: use systemd-notify for system container
  (gscrivan@redhat.com)
- Remove global DefaultRegistryClient (obulatov@redhat.com)
- node: allow to customize master service name (gscrivan@redhat.com)
- Exclude list cleans up properly (#1430929) (tdawson@redhat.com)
- The router images used in e2e tests should come from the build setup
  (ccoleman@redhat.com)
- Remove the excluders origin-excluder (sdodson@redhat.com)
- Generated changes for image import reference-policy (maszulik@redhat.com)
- Bug 1420976 - Support passing reference-policy in import-image command
  (maszulik@redhat.com)
- bump(github.com/fsouza/go-dockerclient):
  a45f2c476754603041aeb54751207d5e5ec30b77 (bparees@redhat.com)
- UPSTREAM: 41436: Fix bug in status manager TerminatePod (decarr@redhat.com)
- cluster up: Use loopback interface for nodename and default server IP
  (cewong@redhat.com)
- Don't try to use `go env` if `go` is not installed (skuznets@redhat.com)
- Gather all rpms for the repo creation (sdodson@redhat.com)
- UPSTREAM: 42973: Fix selinux support in vsphere (gethemant@gmail.com)
- Remove a pointless cast (danw@redhat.com)
- Pullthrough broken registry server (agladkov@redhat.com)
- add env flag to  new-build example (li.guangxu@zte.com.cn)
- Make router whitespace great again (ccoleman@redhat.com)
- fix kubernetes scheduler long description (li.guangxu@zte.com.cn)
- prevent rolling back to the same dc version (matthias.bertschy@gmail.com)
- Disable CLI tests in the e2e suite (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ad6c8070fd680f68c35222694d9f2cdcabc0c9e6 (dmcphers+openshiftbot@redhat.com)
- Add imagebuilder do origin-release Dockerfiles (jhadvig@redhat.com)
- Remove need for docker in build-images, use multi-tag (ccoleman@redhat.com)
- Make the openvswitch image descend from node (ccoleman@redhat.com)
- Stop chowning /var in haproxy image (ccoleman@redhat.com)
- Fix permissions on scripts so chmod is not necessary (ccoleman@redhat.com)
- Stop building the deployment example (ccoleman@redhat.com)
- Remove haproxy-base image, no longer used (ccoleman@redhat.com)
- Fail if an image fails the build (ccoleman@redhat.com)
- Fix govet errors (maszulik@redhat.com)
- Remove setting up env, it hides errors from go vet (maszulik@redhat.com)
- don't use baseCmdName when printing client version (jvallejo@redhat.com)
- Use kube auth interfaces for union and group (mkhan@redhat.com)
- Updated “Insecure registries” location for newer versions of Docker for
  Mac (hello@marcotroisi.com)
- Revert "Fix of BUG 1405440" (pcameron@redhat.com)
- Make image prefix configurable in setup_image_vars (mkargaki@redhat.com)
- Restrict gssapi flag to only host platform (mkumatag@in.ibm.com)
- Add bootstrap cluster role for external pv provisioners (mawong@redhat.com)
- Fix TestNewAppSourceAuthRequired integration test flake (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  dfb604a40382b584f96f6aff81ba7b9a969c7b9c (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4bc00c8e177d847b30af7cb7806ebaed2843100f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  48d5dfdd6a235ea6fd0f71f72dd58a5ede25f930 (dmcphers+openshiftbot@redhat.com)
- split openshift authorizer interface like kubernetes (deads@redhat.com)
- Disable IngressConfiguredRouter test (skuznets@redhat.com)
- switch to kube authorization attributes (deads@redhat.com)
- Accept the `$JUNIT_REPORT` flag in networking tests (skuznets@redhat.com)
- switch meaning to openshift.GetResource to match upstream (deads@redhat.com)
- Use a very high patch number on RPM release builds (skuznets@redhat.com)
- switch to using interface for authorization matches (deads@redhat.com)
- authorize personalSAR based on selfsubjectaccessreviews.authorization.k8s.io
  (deads@redhat.com)
- UPSTREAM: 41226: Fix for detach volume when node is not present/ powered off
  (eboyd@redhat.com)
- UPSTREAM: 41217: Fix wrong VM name is retrieved by the vSphere Cloud Provider
  (eboyd@redhat.com)
- UPSTREAM: 40693: fix for vSphere DeleteVolume (eboyd@redhat.com)
- UPSTREAM: 39757: Fix space in volumePath in vSphere (eboyd@redhat.com)
- UPSTREAM: 39754: Fix fsGroup to vSphere (eboyd@redhat.com)
- UPSTREAM: 39752: Fix panic in vSphere cloud provider (eboyd@redhat.com)
- bump(github.com/openshift/origin-web-console):
  295a6c8a2bd083a39a4c7616b45bf641353206c9 (dmcphers+openshiftbot@redhat.com)
- Generate jUnit XML reports from hack/test-end-to-end.sh (skuznets@redhat.com)
- Error when user gives build args with non-Docker strat (rymurphy@redhat.com)
- UPSTREAM: 42622: Preserve custom etcd prefix compatibility for etcd3
  (mkhan@redhat.com)
- CGO_ENABLED prevents build cache reuse (ccoleman@redhat.com)
- add pr testing for sync plugin, some refactor of jenkins pluging pr testing
  (gmontero@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f11890ba068af6438d90fab7b02b7e35171585f0 (dmcphers+openshiftbot@redhat.com)
- sdn: handle offset>0 fragments when validating service traffic
  (dcbw@redhat.com)
- Update required docker version to 1.12, including oc cluster up
  (jminter@redhat.com)
- wire in smart group adder (deads@redhat.com)
- Registry still needs to get imagestreamimages (miminar@redhat.com)
- UPSTREAM: 38925: Fix nil pointer issue when making mounts for container
  (sjenning@redhat.com)
- etcd clusters must negotiate to reach v3 mode, wait (ccoleman@redhat.com)
- make new-app error reports clearer (gmontero@redhat.com)
- Allow control over TLS version and ciphers for docker-registry
  (jliggitt@redhat.com)
- use pod network during docker build (bparees@redhat.com)
- UPSTREAM: 42491: make the system:authenticated group adder smarter
  (deads@redhat.com)
- UPSTREAM: revert: 8d20a24: 42421: proxy not providing user info should cause
  error" (deads@redhat.com)
- correct the ways that routes are iterated over to be cleared
  (jtanenba@redhat.com)
- Updating image_ecosystem extended tests (cdaley@redhat.com)
- DRY out `os::cmd` jUnit generation, improve logic (skuznets@redhat.com)
- Move pkg/template/registry to pkg/template/registry/template
  (jminter@redhat.com)
- Fix of BUG 1405440 (yhlou@travelsky.com)
- Fix defaulter gen on Mac and add make update-api (ccoleman@redhat.com)
- Enhance new-app circular test to handle ImageStreamImage refs
  (gmontero@redhat.com)
- node/sdn: make /var/lib/cni persistent to ensure IPAM allocations stick
  around across node restart (dcbw@redhat.com)
- Disable local fsgroup quota test when not using XFS (skuznets@redhat.com)
- Finish deprecation of old `hack/test-go.sh` envars (skuznets@redhat.com)
- Begin deprecation of `os::log::warn` in favor of `os::log::warning`
  (skuznets@redhat.com)
- add client auth configmap (deads@redhat.com)
- UPSTREAM: <drop>: wait for loopback permissions, remove after updating
  loopback authenticator (deads@redhat.com)
- Fix cookies for reencrypt routes with InsecureEdgeTerminationPolicy "Allow"
  (jtanenba@redhat.com)
- bump(github.com/coreos/etcd):v3.1.0 (maszulik@redhat.com)
- Work around docker race condition when running build post commit hooks.
  (jminter@redhat.com)
- Insecure istag allows for insecure transport (miminar@redhat.com)
- Improve error message in image api helper (miminar@redhat.com)
- Do the manifest verification just once (miminar@redhat.com)
- Allow remote blob access checks only for manifest PUT (miminar@redhat.com)
- Cache imagestream and images during a handling of a single request
  (miminar@redhat.com)
- UPSTREAM: 41814: add client-ca to configmap in kube-public (deads@redhat.com)
- Add test for `oc observe --type-env-var` (tkusumi@zlab.co.jp)
- allow build request override of pipeline strategy envs (gmontero@redhat.com)
- use the extraClientCA as it was intended (deads@redhat.com)
- Add integration test for front proxy (mkhan@redhat.com)
- UPSTREAM: 42421: proxy not providing user info should cause error
  (deads@redhat.com)
- UPSTREAM: 36774: allow auth proxy to set groups and extra (deads@redhat.com)
- wire up front proxy authenticator (deads@redhat.com)
- No failure reason displayed when build failed using invalid contextDir
  (cdaley@redhat.com)
- add front proxy as an option for authenticating to the API (deads@redhat.com)
- Minor/boring change: Consistently return user facing errors in SDN
  (rpenta@redhat.com)
- Update Vagrantfile (dmcphers@redhat.com)
- cluster up: warn on error parsing Docker version (cewong@redhat.com)
- make ciphers/tls version configurable (jliggitt@redhat.com)
- UPSTREAM: 42337: Plumb cipher/tls version serving options
  (jliggitt@redhat.com)
- Helper for accessing etcd (maszulik@redhat.com)
- update template to allow 32 vs. 64 bit JVM selection for Jenkins
  (gmontero@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7529b6f5cc000c4163f16913577e4c2f5c38e374 (dmcphers+openshiftbot@redhat.com)
- Reorder extended startup to yield jUnit with TEST_ONLY (skuznets@redhat.com)
- Make `oc observe --type-env-var` pass current env vars (tkusumi@zlab.co.jp)
- Add stateful sets permissions to disruption controller (jliggitt@redhat.com)
- Switch to nip.io from xip.io for default cluster up wildcard DNS
  (jimmidyson@gmail.com)
- bump(github.com/openshift/origin-web-console):
  9d165691ab4ec37fbc44d04976b572957f63265b (dmcphers+openshiftbot@redhat.com)
- Make the default Docker volume size larger (cewong@redhat.com)
- Added types to support Docker build-args (rymurphy@redhat.com)
- tests added for quota scopes while debugging (deads@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d5e1aba11fc08b7cfb8d68b2e22eb460306657fe (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/source-to-image):
  07947a6f4fee815cbe97db93c026d8aa4c9804a9 (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8d4deed08106c03c1fde9ea46e4f64948ecbed39 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 42275: discovery restmapping should always prefer /v1
  (deads@redhat.com)
- UPSTREAM: <drop>: admission namespace isAccessReview, remove post 1.7 rebase
  (mkhan@redhat.com)
- Generated changes (maszulik@redhat.com)
- Clean storage configuration (maszulik@redhat.com)
- UPSTREAM: 40080: Fix resttest Update action when AllowUnconditionalUpdate is
  false (maszulik@redhat.com)
- UPSTREAM: 40903: Set docker opt separator correctly for SELinux options
  (maszulik@redhat.com)
- UPSTREAM: revert: 9f81f6f: <carry>: Change docker security opt separator to
  be compatible with 1.11+ (maszulik@redhat.com)
- Necessary origin updates (maszulik@redhat.com)
- UPSTREAM: 40301: present request header cert CA (maszulik@redhat.com)
- UPSTREAM: revert: 15aaac7f8391a1e0514f80e6450f4bf12c0db191: <drop>: add
  ExtraClientCACerts to SecureServingInfo" (maszulik@redhat.com)
- Enable authorization.k8s.io API and update integration tests
  (mkhan@redhat.com)
- bump(github.com/openshift/origin-web-console):
  15ba89f164629cecc51a48721b805eaf3631695f (dmcphers+openshiftbot@redhat.com)
- networking tests for a pre configured cluster using cni plugin
  (rchopra@redhat.com)
- UPSTREAM: 39751: Changed default scsi controller type (eboyd@redhat.com)
- provider recorder to attach detach controller (hchen@redhat.com)
- Use posttrans for docker-excluder (#1404193) (tdawson@redhat.com)
- Change logging deployer image name from 'logging-deployment' to 'logging-
  deployer' (cewong@redhat.com)
- Do not exclude the excluder for atomic-openshift (tdawson@redhat.com)
- Fixup OCP version (#1413839) (tdawson@redhat.com)
- Use `os::log::debug` to quiet down server starts (skuznets@redhat.com)
- Use `openshift admin ca` instead of deprecated command (skuznets@redhat.com)
- Use correct function to start the node process (skuznets@redhat.com)
- Always run extended tests with `-v -noColor` (skuznets@redhat.com)
- When no jUnit suites are requested, don't merge them (skuznets@redhat.com)
- Handle the case where no k8s containers exist for cleanup
  (skuznets@redhat.com)
- Test goimports binary path diff (mkargaki@redhat.com)
- Added image migration script (miminar@redhat.com)
- UPSTREAM: 41455: Fix AWS device allocator to only use valid device names
  (mfojtik@redhat.com)
- UPSTREAM: 38818: Add sequential allocator for device names in AWS
  (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cfbf483a4d193ae083a9f9ca4b2d720c0b757253 (dmcphers+openshiftbot@redhat.com)
- Add validation to SDN objects with invalid name funcs (mkhan@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a941af697026bc738b4d3d584baf46d3c3298054 (dmcphers+openshiftbot@redhat.com)
- Update autogenerated files. (vsemushi@redhat.com)
- Modify privileged SCC to allow to use all capabilities. (vsemushi@redhat.com)
- UPSTREAM: <carry>: allow to use * as a capability in Security Context
  Constraints. (vsemushi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c38d9de89dac52739d43d77c2625008812e3bb07 (dmcphers+openshiftbot@redhat.com)
- backup and remove keys during migration (sjenning@redhat.com)
- add migration script to fix etcd paths (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a05779d1aad58aef4fc900915f69fec9d10c5130 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cdb8a2459e8f7d67727f64aa0b0cdf35effccd2b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ee3fd10e8db4b01d4c3a3e84fd7b4f0d4d1beefc (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  71bf8613ed7dc55a384b4e77102f9071cfd87cd5 (dmcphers+openshiftbot@redhat.com)
- add env vars to pipeline strategy (bparees@redhat.com)
- Use DefaultImagePrefix instead of hardcoded 'openshift/origin' for network
  diagnostic image. (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7195b40c948520343d1b20263d1de4714a76b430 (dmcphers+openshiftbot@redhat.com)
- Sync etcd endpoints during lease acquistion (agoldste@redhat.com)
- Stop removing empty logs from the artifacts (skuznets@redhat.com)
- tests: increate timeout for pods to be ready for dc test (mfojtik@redhat.com)
- update guest profile with new arp tuning missed in
  https://github.com/openshift/origin/pull/13034 (jeder@redhat.com)
- Verify manifest with remote layers (agladkov@redhat.com)
- use conformance tag properly (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3af61e941288f389b373059110ce2612208c825d (dmcphers+openshiftbot@redhat.com)
- Bug 1422376: Fix resolving ImageStreamImage latest tag (mfojtik@redhat.com)
- build API: mark fields related to extended builds as deprecated
  (cewong@redhat.com)
- Output VXLAN multicast flow in sorted order (danw@redhat.com)
- Retry OVS flow checks a few times if they fail (danw@redhat.com)
- Don't overwrite /usr/local/bin with a file (sdodson@redhat.com)
- UPSTREAM: 40935: Plumb subresource through subjectaccessreview
  (pweil@redhat.com)
- bump(github.com/openshift/origin-web-console):
  105dc93e18970860caa7763f9f7e2a46874b49e7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a1e8edcd20ee3665e57d81fd014703163406cafd (dmcphers+openshiftbot@redhat.com)
- tito: generate man pages (mkargaki@redhat.com)
- Remove redundant docs (mkargaki@redhat.com)
- Check out generated docs (mkargaki@redhat.com)
- Stop checking in generated docs (mkargaki@redhat.com)
- Write `os::log` messages to a file if possible (skuznets@redhat.com)
- Refactor container cleanup to improve logging (skuznets@redhat.com)
- Add additional logging to new_app.go extended test (jminter@redhat.com)
- Bug 1425706 - protect from nil tlsConfig. (maszulik@redhat.com)
- Update README.md (sanjusoftware@gmail.com)
- Fix typos in router code (yhlou@travelsky.com)
- bump(github.com/openshift/origin-web-console):
  188b0c85576fc0de4f0ac85a41633fb1d37ea0e7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  76d914e05af63bd3cebc81c4a742d9cab82c4909 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  42c67c209b85e50609b369ddc2d2f3e761064388 (dmcphers+openshiftbot@redhat.com)
- supplemental_groups.go: minor improvements. (vsemushi@redhat.com)
- stash the build logs before the test fails (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d3237d4839e604be78fa52c8ba2a6e35ac8f388f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  217fa5cb130340f7dc1e6fee80913ddbcc77a948 (dmcphers+openshiftbot@redhat.com)
- Document to describe networking requirements for vendors replacing openshift-
  sdn (rchopra@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a5f801fdbae73728d89ef56c4695dec36f0d72e7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c74ee1a12652bc8003db3588b7de9a09f8f42930 (dmcphers+openshiftbot@redhat.com)
- prevent build updates from reverting the build phase (bparees@redhat.com)
- add timeout http-keep-alive to the router template (jtanenba@redhat.com)
- bump(github.com/openshift/origin-web-console):
  29ee6fc5d9af3d8259554dc663bcade6c11fbe04 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5d8765c344475b7831752f9d156a5056535e4e6f (dmcphers+openshiftbot@redhat.com)
- to fix bugzilla 1424946 (salvatore-dario.minonne@amadeus.com)
- Snip a dependency from the e2e tests (ccoleman@redhat.com)
- Extract duplicated code between new-build and new-app (li.guangxu@zte.com.cn)
- bump(github.com/openshift/origin-web-console):
  36c701fa313582735a70142b2f93f514479a6cbd (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5b16b107dfc4f0c53fcb7bf3b1bc8db799a82a24 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  872b7245f6a7a9c1426c5ae4c2b20b4737cd26ee (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a0d6ce8c4209ebb07649a457040d649503c1a1f0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b3a4d80f16a76cf8db8ced99df14f8c7ce998d12 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  adbb7cec51cb934cffd67df8e188faffdce5ba5b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  711693f8c47150adec04f6bd282231627d1f8284 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  81a20fd396cb477392c4f87157c46cb02a5f80b5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  29a658f3d88371f88070e1e21a2d3c4919b80121 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9b47dfc8bc5827e48a737ffc29f5f3c09e6854e1 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e812e5798650eb74187dac0e55da7c7de07d7233 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8c3bb14f9feaf5454686fbaedbde81727343063e (dmcphers+openshiftbot@redhat.com)
- the router tests demand access to the 1936 port (rchopra@redhat.com)
- Change default arp cache size on nodes (pcameron@redhat.com)
- bump watch timeout for tests to allow for reconciliation recovery
  (bparees@redhat.com)
- SCC review client: generated code (salvatore-dario.minonne@amadeus.com)
- SCC review client: fix bugzilla 1424946 (salvatore-dario.minonne@amadeus.com)
- bump(github.com/openshift/origin-web-console):
  abb2505b2aefddbd57deeb9e57b16bcce4dd9b89 (dmcphers+openshiftbot@redhat.com)
- Remove OSE image build scripts from this repository (skuznets@redhat.com)
- fix bugzilla 1421616 and 1421570 (salvatore-dario.minonne@amadeus.com)
- PSP reviews: client (salvatore-dario.minonne@amadeus.com)
- Update to golang 1.8 ga (ccoleman@redhat.com)
- Generated changes (maszulik@redhat.com)
- jenkins client plugin PR testing (gmontero@redhat.com)
- only report no running pods once (pweil@redhat.com)
- Removed line breaks in glog messages (ffranz@redhat.com)
- Auto generated: docs/bash completions for network diagnostic pod image option
  (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8a5a5f12c3d614d1e8a97e87829c2ca2504939b3 (dmcphers+openshiftbot@redhat.com)
- patch kubeconfig if token cannot be deleted via api (jvallejo@redhat.com)
- Backported redistributable logic to Origin specfile (skuznets@redhat.com)
- Pullthrough typos in oc tag (maszulik@redhat.com)
- Origin image was creating a file at /usr/local/bin with imagebuilder
  (ccoleman@redhat.com)
- Fix force pull behavior in release, correct changelog gen
  (ccoleman@redhat.com)
- Make network diagnostic pod image configurable (rpenta@redhat.com)
- Bug 1421643 - Use existing openshift/origin image instead of new openshift
  /diagnostics-deployer (rpenta@redhat.com)
- Add parent BuildConfig to Build OwnerReferences (rymurphy@redhat.com)
- UPSTREAM: 41196: Fix for Premature iSCSI logout (hchen@redhat.com)
- Change "." to "-" in generated hostnames for routes (jtanenba@redhat.com)
- Add missing newlines in oc tag (maszulik@redhat.com)
- install ceph-common pkg on origin to support rbd provisioning
  (hchen@redhat.com)
- Node should default to controller attach detach (ccoleman@redhat.com)
- update internal error message (jvallejo@redhat.com)
- suggest using default cluster port (jvallejo@redhat.com)
- context.Context should be the first parameter of a function
  (yu.peng36@zte.com.cn)
- The first letter should be small in error of token.go (yu.peng36@zte.com.cn)

* Mon Feb 20 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.32-1
- UPSTREAM: 41658: Fix cronjob controller panic on status update failure
  (maszulik@redhat.com)
- add oc expose -h explanation on generator (jvallejo@redhat.com)
- Have make_redistributable changable via the command line.
  (tdawson@redhat.com)
- improve flag description; add warning msg (jvallejo@redhat.com)
- generated: CLI docs and completions (ccoleman@redhat.com)
- Support local template transformation in process (ccoleman@redhat.com)
- Ensure RPMs are only build from clean git trees (skuznets@redhat.com)
- Add logging to bluegreen-pipeline.yaml to help diagnose flake Rewrite blue-
  green pipeline extended test to avoid hang on error (jminter@redhat.com)
- add service catalog metadata to templates (bparees@redhat.com)
- add closure that guarantees mutex unlock in loop (jvallejo@redhat.com)
- (WIP) Fixing build reason getting wiped out by race condition
  (cdaley@redhat.com)
- allow namespace specification via parameter in templates (bparees@redhat.com)
- UPSTREAM: 39998: Cinder volume attacher: use instanceID instead of NodeID
  when verifying attachment (jsafrane@redhat.com)
- Change all aarch64 references to arm64 (tdawson@redhat.com)
- inform that port is required as part of set-probe error when port missing
  (jvallejo@redhat.com)

* Fri Feb 17 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.31-1
- do not remove non-existent packages (bparees@redhat.com)
- Dump router logs for debugging purposes in the scoped and weighted extended
  tests. (smitram@gmail.com)

* Thu Feb 16 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.30-1
- Fix NetworkPolicies allowing from all to *some* (not all) (danw@redhat.com)

* Thu Feb 16 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.29-1
- 

* Thu Feb 16 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.28-1
- 

* Thu Feb 16 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.27-1
- deployment: add unit test to exercise the watch restart (mfojtik@redhat.com)
- deployment: retry failed watch when observed controller has old resource
  version (mfojtik@redhat.com)
- Add TestEtcdStoragePath (mkhan@redhat.com)
- UPSTREAM: <carry>: Change docker security opt separator to be compatible with
  1.11+ (pmorie@redhat.com)
- Revert "e2e test: remove PodCheckDns flake" (maszulik@redhat.com)
- Add replace patch strategy for DockerImageMetadata and cmd tests for oc edit
  istag (maszulik@redhat.com)
- UPSTREAM: 41043: allow setting replace patchStrategy for structs
  (maszulik@redhat.com)

* Wed Feb 15 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.26-1
- improve output of `oc idle` (jvallejo@redhat.com)

* Wed Feb 15 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.25-1
- 

* Wed Feb 15 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.24-1
- improve scale,process,get help output handle multi-line strings in
  describe/newapp output (jvallejo@redhat.com)

* Wed Feb 15 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.23-1
- 

* Wed Feb 15 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.22-1
- Merge remote-tracking branch enterprise-3.5, bump origin-web-console 04d7652
  (tdawson@redhat.com)
- dind: upgrade to fedora 25 (marun@redhat.com)

* Wed Feb 15 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.21-1
- Fix make_redistributionable logic (tdawson@redhat.com)
- Add new cross.sh command line options (tdawson@redhat.com)
- change the parameter names in the route generation function to match the oc
  expose flag name (jtanenba@redhat.com)
- in clear-route-status.sh alert users if 'jq' tool is not installed
  (jtanenba@redhat.com)
- a split on string can give empty elements - fix bz1421572
  (rchopra@redhat.com)
- UPSTREAM: 41329: stop senseless negotiation (deads@redhat.com)
- Handle RPM output paths better in RPM build (skuznets@redhat.com)
- Provide canonical version and release to `tito` (skuznets@redhat.com)
- router: Fix ingress handling of nil rule value (marun@redhat.com)
- Bumped minimum acceptable tito version (skuznets@redhat.com)
- Refactored custom `tito` tagger and builder (skuznets@redhat.com)
- openvswitch: add wrapper to read configuration from env files
  (gscrivan@redhat.com)
- origin: add wrapper to read configuration from env files
  (gscrivan@redhat.com)
- node: add wrapper to read configuration from env files (gscrivan@redhat.com)
- add better logging for new-app argument processing (bparees@redhat.com)
- add generated docs (jvallejo@redhat.com)
- add add-cluster-role-to-user support for -z (jvallejo@redhat.com)
- Fix test hosts to be unique (different). (smitram@gmail.com)
- add sample using openshift-client-plugin syntax; add ext test
  (gmontero@redhat.com)

* Mon Feb 13 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.20
- set php latest to v7 (bparees@redhat.com)
- Enforce authz(n) on controller (ccoleman@redhat.com)
- Add Path and IsNonResourceURL to SAR (like upstream) (ccoleman@redhat.com)
- Move quasi-generic handlers out so controller can reuse them
  (ccoleman@redhat.com)
- don't include the project name in the new-app validation check
  (bparees@redhat.com)
- Change how the router was logging (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  35f07305cdd184856a5a953215e2a350adbe8c26 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8d3d3c618780c9b4885dee91c362835140269b53 (dmcphers+openshiftbot@redhat.com)
- Perform a mv instead of cp during RPM release (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  95fab83b258d25272bbbbb1fc9c5d81c666d7e30 (dmcphers+openshiftbot@redhat.com)
- remove mongo clustered test: replaced by statefulset example and test
  (jminter@redhat.com)
- Remove special handling of --token and --context for whoami
  (jliggitt@redhat.com)
- Add a dnsBindAddress configuration to the node (ccoleman@redhat.com)
- Swagger generation uses a hardcoded directory (ccoleman@redhat.com)
- master: allow to specify command (gscrivan@redhat.com)
- Take more care in handling statefulset pods in mongodb test
  (jminter@redhat.com)
- node: install conntrack-tools in the node image (gscrivan@redhat.com)
- node: fix typo in the system container image (gscrivan@redhat.com)
- build extended tests: don't use large Jenkins image unnecessarily
  (jminter@redhat.com)
- Remove legacy kube test runner (marun@redhat.com)
- Add a status function to excluder (sdodson@redhat.com)

* Fri Feb 10 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.19
- Dump router pod logs on e2e failures (ccoleman@redhat.com)
- Wait for the `kubernetes` service on server start (skuznets@redhat.com)
- Revert "Revert "Revert "Refactored custom `tito` tagger and builder"""
  (skuznets@redhat.com)
- use src distinct directory for context-dir builds (bparees@redhat.com)
- return partial matches when default latest tag is unavailable
  (bparees@redhat.com)
- Add a new cluster-debugger role and enable debugging on masters
  (ccoleman@redhat.com)
- Revert "install ceph-common pkg on origin to support rbd provisioning"
  (ccoleman@redhat.com)
- deploy: bump number of retries in trigger integration test
  (mfojtik@redhat.com)
- Fix wrong type for printf (song.ruixia@zte.com.cn)
- Sort `List` for Project virtual storage (mkhan@redhat.com)
- Sort `List` for RoleBinding virtual storage (mkhan@redhat.com)
- Sort `List` for Role virtual storage (mkhan@redhat.com)
- fix typo (noblea1117@gmail.com)
- generated: bootstrap bindata (ccoleman@redhat.com)
- Include prometheus and heapster in bootstrap bindata (ccoleman@redhat.com)
- Update heapster and prometheus examples for consistent ports
  (ccoleman@redhat.com)
- Build rpms on aarch64, ppc64le and s390x (tdawson@redhat.com)
- install the types for the ingress admission controller (jtanenba@redhat.com)
- UPSTREAM: 41147: Add debug logging to eviction manager (decarr@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3f779bc6a5c8b1529c16a6c6bd16aad9ab85691a (dmcphers+openshiftbot@redhat.com)
- Bug 1393716 - Fix network diagnostics on containerized openshift install
  (rpenta@redhat.com)
- minor doc correction (pweil@redhat.com)
- Deprecate User.groups field (jliggitt@redhat.com)
- update generated completetion and docs (mfojtik@redhat.com)
- image: add --reference-policy to oc tag (mfojtik@redhat.com)
- add test for resolving images with reference policy (mfojtik@redhat.com)
- 503 page does not show guide detail as expected. (pcameron@redhat.com)
- bug 1419472 print master config error if it exists (jvallejo@redhat.com)
- router: fix ingress compatibility with f5 (marun@redhat.com)
- Changed the router default to roundrobin if non-zero weights are used
  (bbennett@redhat.com)
- Revert "Install tito from source in `openshift/origin-release`"
  (skuznets@redhat.com)
- Revert "Revert "Refactored custom `tito` tagger and builder""
  (skuznets@redhat.com)
- add ceph-common (hchen@redhat.com)
- image(router): Add logging facility to router tmpl (sjr@redhat.com)
- use ImageStreamImport for container command lookup in oc debug
  (jminter@redhat.com)
- install ceph-common pkg on origin to support rbd provisioning
  (hchen@redhat.com)

* Wed Feb 08 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.18
- bump(github.com/openshift/origin-web-console):
  be73cc17031099e9eb4947a6d1ce9024d774910f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3d2855d6c1920345a14b3bad4e91839d5080fd2d (dmcphers+openshiftbot@redhat.com)
- recreate generated service cert secret when deleted (mfojtik@redhat.com)
- UPSTREAM: 41089: Use privileged containers for statefulset e2e tests
  (skuznets@redhat.com)
- only run handleBuildCompletion on completed builds (bparees@redhat.com)
- Fixed the multicast CIDR (was 224.0.0.0/3 not /4) (bbennett@redhat.com)
- treat fatal errors as actually fatal (bparees@redhat.com)
- Allow users to parameterize the image prefix (skuznets@redhat.com)
- add newline to login output (deads@redhat.com)
- Add multicast test to extended networking test (danw@redhat.com)
- change argument --wildcardpolicy to --wildcard-policy in 'oc expose'
  (jtanenba@redhat.com)
- added clear-route-status.sh script to images/router/ (root@wsfd-
  netdev29.ntdv.lab.eng.bos.redhat.com)
- Refactor release packaging to be DRYer (skuznets@redhat.com)
- deploy: rework extended test to use rollout latest and increase timeout for
  getting logs (mfojtik@redhat.com)
- build: take referencePolicy into account when resolving istag
  (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  fbc282d0f3f136c9d7eb6b72d89afa3db7b16feb (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  68c65908b68c559777cc46a1e8a501f992859469 (dmcphers+openshiftbot@redhat.com)
- Allow multicast for VNID 0 (danw@redhat.com)
- report a useful error when wide mode is used with new-app/new-build
  (bparees@redhat.com)
- Fix OVS connection tracking in networkpolicy plugin (danw@redhat.com)
- UPSTREAM :41034: use instance's Name to attach gce disk (hchen@redhat.com)
- 1408172 - ipfailover - `Permission denied for check and notify scripts
  (pcameron@redhat.com)
- cluster up: add brew install instructions (cewong@redhat.com)
- cluster up: add registry instructions (cewong@redhat.com)
- Make haproxy maxconn configurable (pcameron@redhat.com)
- Fix cluster up documentation of Linux firewalld instructions
  (cewong@redhat.com)
- Expose product-specific string literals as ldflags (skuznets@redhat.com)
- UPSTREAM: google/cadvisor: 1588: disable thin_ls due to excessive iops
  (decarr@redhat.com)
- Add `$GOPATH/bin` to `$PATH` only when `$GOPATH` is set (skuznets@redhat.com)
- Replace our Validator interface with upstream one (maszulik@redhat.com)
- node: add files for running as a system container (gscrivan@redhat.com)
- Correct the way that haproxy uses the secure cookie attribute
  (jtanenba@redhat.com)
- Fix type %%s to %%d for int type (song.ruixia@zte.com.cn)
- perform both http and https checks for monitoring f5 pools
  (rchopra@redhat.com)
- UPSTREAM: 39842: Remove duplicate calls to DescribeInstance during volume
  operations (hekumar@redhat.com)
- origin: add files for running as a system container (gscrivan@redhat.com)
- openvswitch: add files for running as a system container
  (gscrivan@redhat.com)

* Mon Feb 06 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.17
- Add standalone heapster as an example (ccoleman@redhat.com)
- Provide an all in one prometheus template example (ccoleman@redhat.com)
- use oc logs to get log (li.guangxu@zte.com.cn)
- bump(github.com/openshift/origin-web-console):
  aac7d52b246630d467ea19881b724a34d1ed08e2 (dmcphers+openshiftbot@redhat.com)
- updated man page (jtanenba@redhat.com)
- Used `os::log` functions better in protobuf script (skuznets@redhat.com)
- UPSTREAM: 40859: PV binding: send an event when there are no PVs to bind
  (jsafrane@redhat.com)
- Fix matchPattern log level to be like debug messages - otherwise all the
  other messages logged at loglevel 4 get overwhelmed. (smitram@gmail.com)
- Disable the admission plugin LimitPodHardAntiAffinityTopology by default.
  (avagarwa@redhat.com)
- Attempt at resolving
  github.com/openshift/origin/pkg/cmd/server/origin.TestAdmissionPluginNames
  github.com/openshift/origin/pkg/cmd/server/start.TestAdmissionOnOffCoverage
  (jtanenba@redhat.com)
- Changed using a map to k8s.io/apimachinery/pkg/utils/sets and added a test
  for adding a hostname (jtanenba@redhat.com)
- addresses Ben's initial comments (jtanenba@redhat.com)
- took out the decription in api/v1/types.go because govet.sh complained
  (jtanenba@redhat.com)
- added swagger docs and verified the gofmt (jtanenba@redhat.com)
- updated generated docs (jtanenba@redhat.com)
- add admission controller to restrict updates to ingresss objects hostname
  field. (jtanenba@redhat.com)
- changes to the cli for creating routes (jtanenba@redhat.com)
- router: provide better indication of failure in ingress test
  (marun@redhat.com)
- update docs/tests (sjenning@redhat.com)
- adding option '--insecure-policy' for passthrough and reencrypt route for CLI
  (jtanenba@redhat.com)
- Changed the router to default to roundrobin with multiple services
  (bbennett@redhat.com)
- UPSTREAM: <carry>: kubelet: change image-gc-high-threshold below docker
  dm.min_free_space (sjenning@redhat.com)

* Fri Feb 03 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.16
- Revert "Merge pull request #12751 from stevekuznetsov/skuznets/path-updates"
  (tdawson@redhat.com)
- Router tests should check for curl errors (ccoleman@redhat.com)
- Add DisableAttachDetachReconcilerSync new flag to master_config_test
  (maszulik@redhat.com)

* Fri Feb 03 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.15
- Merge remote-tracking branch enterprise-3.5, bump origin-web-console 7d868ff
  (tdawson@redhat.com)
- Use watch.Deleted event on the endpoints to ensure we do a valid transition
  Deleted -> Added instead of Modified -> Added. Otherwise the event queue code
  panics and kills the cache reflector goroutine which causes events to never
  be delivered to the test router process. fixes #12736 (smitram@gmail.com)
- Make sure to honor address binding request for all services
  (jonh.wendell@redhat.com)
- UPSTREAM: 38527: Fail kubelet if runtime is unresponsive for 30 seconds
  (decarr@redhat.com)
- Increased the time the proxy will hold connections when unidling
  (bbennett@redhat.com)
- bump(github.com/openshift/origin-web-console):
  16ebed8ff9127d6316caa01888935dc48ec69838 (dmcphers+openshiftbot@redhat.com)
- use secrets in sample templates (bparees@redhat.com)
- No one needs this info (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  774a733104caa86180127ea8cf5d05639a76e8fb (dmcphers+openshiftbot@redhat.com)
- Search for `goimports` in `$GOPATH`, not `$PATH` (skuznets@redhat.com)
- Update `$PATH` to include `$GOPATH/bin` by default (skuznets@redhat.com)
- increase mongodb test image pull timeout (jminter@redhat.com)
- Preserve file and directory validation errors (mkhan@redhat.com)
- UPSTREAM: 40763: reduce log noise when aws cannot find public-ip4 metadata
  (decarr@redhat.com)
- only set CGO_ENABLED=0 for tests (jminter@redhat.com)
- enable Azure disk provisioner (hchen@redhat.com)
- bump(github.com/openshift/origin-web-console):
  936eff71a6d7bc105b642642c8698d42f1474865 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4eaf2880b0992378b873ab223237110d1408a0bf (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  52f496aef9bd29703bf878b68f1d3bdafe05d205 (dmcphers+openshiftbot@redhat.com)
- include thin_ls package in base image (sjenning@redhat.com)
- generated-protobuf is using the wrong method (ccoleman@redhat.com)
- Adding wildcardpolicy flag to `oc create route` and a column for the
  wildcardpolicy to `oc get route' (jtanenba@redhat.com)
- Add check for empty directory to recycler (jsafrane@redhat.com)

* Wed Feb 01 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.14
- bump(github.com/openshift/origin-web-console):
  7b83464ba2c7fbc936d40ed1ed5f4f4a93d180a3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5fe0d17f41073fdd3e271cb8f8a746ce762cc66c (dmcphers+openshiftbot@redhat.com)
- Pick a smaller image for idling unit tests (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  63ae71a1ec299c1d0198bdfa518c25aab36b8c41 (dmcphers+openshiftbot@redhat.com)
- cluster up: remove hard-coded docker root mount (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  285733b9e8b762b06a301ca4d67e4f8fe870aee9 (dmcphers+openshiftbot@redhat.com)
- cluster up: fix port checking, Mac startup (cewong@redhat.com)
- Change MEMORY LIMIT parameter to be required one for multiple DBs
  (vdinh@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e14b670c7a12564723b573bf9e98af431df270e6 (dmcphers+openshiftbot@redhat.com)
- Replace find with grep -rl in update scripts (agoldste@redhat.com)
- treat binary buildconfig instantiate requests as long running
  (bparees@redhat.com)
- Remove List and ListOptions from non-round-trippable types in serialization
  test (mfojtik@redhat.com)
- deploy: add support for dc --dry-run to rollout undo (mfojtik@redhat.com)
- Short-circuit path shortening logic when possible (skuznets@redhat.com)
- Use `os::log` better in `os::util::ensure` functions (skuznets@redhat.com)
- Use `exit` instead of `return` in `os::log::fatal` (skuznets@redhat.com)

* Tue Jan 31 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.13
- Use a greedy match instead of a lazy one for versions (skuznets@redhat.com)
- make sure all projects are deleted on test start (jvallejo@redhat.com)
- Revert "Merge pull request #12671 from openshift/revert-12328-jvallejo
  /update-oc-status-warning" (jvallejo@redhat.com)

* Tue Jan 31 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.12
- Revert "Refactored custom `tito` tagger and builder" (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  46c6d8bc6c881c6ed4e520a62e8c2398ae464d9d (dmcphers+openshiftbot@redhat.com)
- Revert the creation of `images/ose` symlink (skuznets@redhat.com)
- Revert URL updates in source code and READMEs (skuznets@redhat.com)
- Fix stub interface so Mac compilations work (bbennett@redhat.com)
- Added an option to `hack/env` to use a new volume (skuznets@redhat.com)
- Bumped minimum acceptable tito version (skuznets@redhat.com)
- Refactored custom `tito` tagger and builder (skuznets@redhat.com)
- Don't give up copying from `hack/env` on failure (skuznets@redhat.com)

* Mon Jan 30 2017 Jenkins CD Merge Bot <tdawson@redhat.com> 3.5.0.11
- Merge remote-tracking branch upstream/master, bump origin-web-console 9989eef
  (tdawson@redhat.com)
- genman: error message update (mkargaki@redhat.com)
- Update README.md (ccoleman@redhat.com)
- Update README (ccoleman@redhat.com)
- setup watches before creating the buildconfig (bparees@redhat.com)
- Filter disallowed outbound multicast (danw@redhat.com)
- Add an annotation for enabling multicast on a namespace (danw@redhat.com)
- podManager simplification (danw@redhat.com)
- Change how multicast rule updates on VNID change work (danw@redhat.com)
- Update comments on multicast flow rules, simplify vxlan multicast rule
  (danw@redhat.com)
- Fix pod multicast route (danw@redhat.com)
- Allow `os::log` functions to print multi-line messages (skuznets@redhat.com)
- start the next serial build immediately after a build is canceled
  (bparees@redhat.com)
- Refactor logic for OSX etcd cURL statement (skuznets@redhat.com)
- Break out `os::cmd` framework tests from `hack/test-cmd.sh`
  (skuznets@redhat.com)
- Make use of `os::util::absolute_path` over `realpath` (skuznets@redhat.com)
- Update completions (jvallejo@redhat.com)
- improve bash completions for namespace flags (jvallejo@redhat.com)
- fix jenkins blue-green ext test so it does not block forever on build/deploy
  error (gmontero@redhat.com)
- Update version regex to handle longer dot-versions (skuznets@redhat.com)
- Update version regex to handle longer dot-versions (skuznets@redhat.com)
- regen origin docs/completions/openapi (sjenning@redhat.com)
- UPSTREAM: 37228: kubelet: storage: teardown terminated pod volumes
  (sjenning@redhat.com)
- add persistent examples to quickstarts (bparees@redhat.com)
- Fix messages, so that the ui tooltips don't have unicode characters.
  (smitram@gmail.com)
- Allow multiple ipfailover configs on same node (pcameron@redhat.com)
- Increase test coverage (agladkov@redhat.com)
- Restore `hack/common.sh` to the version in Origin (skuznets@redhat.com)
- Remove back-ported container manifests from dist-git (skuznets@redhat.com)

* Fri Jan 27 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.10
- bump(github.com/openshift/origin-web-console):
  9989eefc00d8379246015350374f4cdfcdc89933 (dmcphers+openshiftbot@redhat.com)
- backup and remove keys during migration (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  960a94b5e2a1416e26fbb16f3d13eda3c45f4f77 (dmcphers+openshiftbot@redhat.com)
- Update clusterup documentation (persistence, proxy) (cewong@redhat.com)
- tweak pipeline build background monitor and ginkgo integration
  (gmontero@redhat.com)
- Router sets its dns name in admitted routes test (pcameron@redhat.com)
- Fix http status for error and remove dead code (agladkov@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ad2fdea2b7ac03e14942f96c7447ad0bdbfed8fe (dmcphers+openshiftbot@redhat.com)
- Minor fixes for dockerregistry/server tests (dmage@yandex-team.ru)
- Revert "Update "no projects" warning in `oc status`" (ccoleman@redhat.com)
- add migration script to fix etcd paths (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f517fe8456628870ad7dfdac6e247abf385b43a2 (dmcphers+openshiftbot@redhat.com)
- Upgrade hack/env to support shell, env vars (ccoleman@redhat.com)
- leave oauth on jenkins extended test, use token for http-level access
  (gmontero@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e202d9ff4575f32427d39ac3b2e0cf21a7a9ff60 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a506596e52864fbfbc8d3cafa40a83156164f4ca (dmcphers+openshiftbot@redhat.com)
- Add audit log (agladkov@redhat.com)
- Update extended tests to run StatefulSet tests (maszulik@redhat.com)
- Rename PetSet to StatefulSet (maszulik@redhat.com)
- Install tito from source in `openshift/origin-release` (skuznets@redhat.com)
- Configure global git options in the release images (skuznets@redhat.com)
- use correct context dir during s2i build (bparees@redhat.com)
- Fix connection URL in postgresql examples (mmilata@redhat.com)
- Existing images are tagged with correct reference on push
  (miminar@redhat.com)
- Stop generating router/registry client certs (jliggitt@redhat.com)
- Remove deprecated credentials flag (jliggitt@redhat.com)
- bump(github.com/openshift/source-to-image):
  72d1d47c7c9e543320db4f2622a152cd6a12fcb3 (bparees@redhat.com)
- prevent Normalize from running twice on oadm drain (jvallejo@redhat.com)
- UPSTREAM: <drop>: request logs when attaching to a container
  (agoldste@redhat.com)
- UPSTREAM: docker/engine-api: 26718: Add Logs to ContainerAttachOptions
  (agoldste@redhat.com)
- Add comment pointing out incorrect SDN annotation naming (danw@redhat.com)
- UPSTREAM: 37846: error in setNodeStatus func should not abort node status
  update (sjenning@redhat.com)

* Wed Jan 25 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.9
- bump(github.com/openshift/origin-web-console):
  c9474fc80ed7b5174e97ff168531384dccb049ba (dmcphers+openshiftbot@redhat.com)
- Replace utilruntime.HandleError with glog (cdaley@redhat.com)
- Add a changelog generator for GitHub releases (ccoleman@redhat.com)
- avoid to return erroneous sa (salvatore-dario.minonne@amadeus.com)
- prevent ctx-switching from failing on server err (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2e4f477035b5cf5ce480d5aa2bbfe3ae0bf046a8 (dmcphers+openshiftbot@redhat.com)
- Update "no projects" warning in `oc status` (jvallejo@redhat.com)
- Add `os::log::debug` and add statements to `hack/env` (skuznets@redhat.com)
- normalize server url before writing to config (jvallejo@redhat.com)
- Make the OS_RELEASE=n ./hack/build-images.sh work again (miminar@redhat.com)
-  modified the comment format (luo.yin@zte.com.cn)
- Use - instead of + in tar filenames because GitHub changes them
  (ccoleman@redhat.com)
- generated: docs (ccoleman@redhat.com)
- Add the certificate command into admin (ccoleman@redhat.com)
- Blacklist pkg/bootstrap/run from govet (ccoleman@redhat.com)
- Add a simple bootstrap mode that waits for a secret (ccoleman@redhat.com)
- Add `oc cluster join` (ccoleman@redhat.com)
- Fix build controller performance issues (cewong@redhat.com)
- Test flakes? Just wait longer in more places (ccoleman@redhat.com)
- Switch empty extension files to return 200 with Content-Length zero
  (jforrest@redhat.com)
- cancel binary builds if they hang (bparees@redhat.com)
- Fix test-go.sh for directories with "-" in their names (danw@redhat.com)
- Generated changes (maszulik@redhat.com)
- Remove copyright from our source code (maszulik@redhat.com)
- UPSTREAM: 40023: Allow setting copyright header file for generated
  completions (maszulik@redhat.com)
- Bug 1415440: Check image history for zero size (mfojtik@redhat.com)
- Router improvement (yhlou@travelsky.com)
- update issue template from oadm (surajssd009005@gmail.com)

* Mon Jan 23 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.8
- Merge remote-tracking branch upstream/master, bump origin-web-console 8e6fe69
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  32319869d5fdfc8a184dacca6e2be489d4898e04 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e3d3db7a2dc0ac26a515d6c11ef052062d903488 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5d5983c3ee7842cbd2ee38345192b87178d4c1e4 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: <carry>: Retry resource quota lookup until count stabilizes
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7f971336337ad19f87b8cae4586f589dfab3f294 (dmcphers+openshiftbot@redhat.com)
- Restore custom etcd prefixes (jliggitt@redhat.com)
- Check for 'in-cluster configuration' in both oc logs and output from oc run
  (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  880fcfc5ca7bf2a26191412f4f6ed824179d0e1f (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 39831: Check if error is Status in result.Stream()
  (mfojtik@redhat.com)
- bump(github.com/coreos/etcd):v3.1.0 (ccoleman@redhat.com)
- Origin does not compile on Macs (ccoleman@redhat.com)
- UPSTREAM: coreos/rkt: <drop>: Workaround etcd310 / grpc incompat
  (ccoleman@redhat.com)
- UPSTREAM: <drop>: Workaround etcd310 / gprc version conflict with CRI
  (ccoleman@redhat.com)
- modify comments  t2T in ut file (bai.miaomiao@zte.com.cn)
- modify Run function comment (xu.zhanwei@zte.com.cn)
- bump(github.com/openshift/source-to-image):
  e01e9b7cdd51bda33dcefa1f28ffa5ddc50e06d7 (bparees@redhat.com)
- UPSTREAM: <carry>: Wait longer for pods to stop (ccoleman@redhat.com)
- Fix expected error message on new test (danw@redhat.com)
- sdn: add multicast support (dcbw@redhat.com)
- util/ovs: add GetOFPort() (dcbw@redhat.com)
- sdn: track running pod VNIDs too (dcbw@redhat.com)
- Router sets its host name in admitted routes (pcameron@redhat.com)
- record build durations properly on build completion (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  448422b31bd1d74cb2f02977e13418ddf20bc181 (dmcphers+openshiftbot@redhat.com)
- Use a differently-named DC in test/cmd/idle.sh (sross@redhat.com)
- Fixes as per @danwinship review comments. (smitram@gmail.com)
- bump(github.com/openshift/origin-web-console):
  e50c0c24b878e52eccf1c08b153973ca42211934 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  857a4e66423f7ef2ef325e119f7d7f0625225b32 (dmcphers+openshiftbot@redhat.com)
- improve extended test documentation (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4690daf1c8f5db5524c1bd10fa9bfdf46a07e10a (dmcphers+openshiftbot@redhat.com)
- Store Jobs under batch/v1 instead of deprecated extensions/v1beta1
  (maszulik@redhat.com)
- Wait for the token to be added to serviceaccount before running command
  (maszulik@redhat.com)
- Implement NetworkPolicies with PodSelectors (danw@redhat.com)
- Implement NetworkPolicies with NamespaceSelectors (danw@redhat.com)
- Implement NetworkPolicies with ports (danw@redhat.com)
- Add initial implementation of NetworkPolicy-based SDN plugin
  (danw@redhat.com)
- Allow requiring a minimum OVS version (danw@redhat.com)
- Migrate build constants from `hack/common.sh` (skuznets@redhat.com)
- registry: refactor verification to share common helper (mfojtik@redhat.com)
- Store built image digest in the build status (mmilata@redhat.com)
- Make the router handle ports properly (don't strip them). (smitram@gmail.com)
- UPSTREAM: <carry>: Increase pod deletion test timeout (ccoleman@redhat.com)
- UPSTREAM: 35436: Add a package for handling version numbers (including non-
  semvers) (danw@redhat.com)
- registry: add image signature endpoint (mfojtik@redhat.com)
- Remove erroneous instance of `${OS_IMAGE_COMPILE_GOFLAGS}`
  (skuznets@redhat.com)

* Fri Jan 20 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.7
- bump(github.com/openshift/origin-web-console):
  5ca208d4d341c22752a25127dcd08450ccd27e76 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  036bca26763207ec4014eb6e8a993c0c0bad2e12 (dmcphers+openshiftbot@redhat.com)
- Wait for the token to be added to serviceaccount before running command
  (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4d03d9894e749c30524b1b07b85f44de5d915553 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ce46f5a444df26078d048ee19b7b064e725638e9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5371b156fbb324d5700562772215a1c4fefa485a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c298116958c86d3dc27e3a3e31df3fff2a5dfd6b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0c6f32f9c822af3a7710e8a7f4ad53c4f3c7930e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4b8b74b74af7b99de3dd8fdf1b7f12e3aae32e0b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c8051e9b45848184b98ce0b79a98f953d7bfb4ca (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f8380670ab9bee828a7dc01382b627b2618384ef (dmcphers+openshiftbot@redhat.com)
- router: validate ingress compatibility with namespace filtering
  (marun@redhat.com)
- router: add support for configuration by ingress (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  37d4d789e90d78264c5b1e77127188320a0a111e (dmcphers+openshiftbot@redhat.com)
- Prune manifest config of schema 2 images (miminar@redhat.com)
- UPSTREAM: 39844: fix bug not using volumetype config in create volume
  (hchiramm@redhat.com)
- bump(github.com/openshift/origin-web-console):
  487f7df29c1b7167c35da40f11f4c8cec657be00 (dmcphers+openshiftbot@redhat.com)
- Update quota test to better express intent (skuznets@redhat.com)
- Add `os::cmd::try_until_not_text` utility function (skuznets@redhat.com)
- Update `os::cmd` error messages to be more correct (skuznets@redhat.com)
- Run output tests only on the last attempt for `os::cmd` (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  32bbb0ff1f294cfdc8eb81118c8717fba185fc0e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  01a4bedf245788cbca27c893c87a99f00547bdc8 (dmcphers+openshiftbot@redhat.com)
- Re-enable etcd3 mode and proto storage (ccoleman@redhat.com)
- Refactor `os::cmd` result printer (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  709cca2cbb68cb840a747aeb012dcc9a46aee198 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  dee65074bb6243f3a284eff2e546fea12f9647a2 (dmcphers+openshiftbot@redhat.com)
- Fixed attempt marking code for try_until stderr (skuznets@redhat.com)
- Updating external examples (cdaley@redhat.com)
- bump(github.com/openshift/origin-web-console):
  61b0d0fa14b68177c16cbc5b9fa2140988fe0324 (dmcphers+openshiftbot@redhat.com)
- Remove unused make target (maszulik@redhat.com)
- cluster up: mount host /dev into origin container (cewong@redhat.com)
- UPSTREAM: 38579: Let admin configure the volume type and parameters for
  gluster DP volumes (hchiramm@redhat.com)
- UPSTREAM: 38378: glusterfs: properly check gidMin and gidMax values from SC
  individually (hchiramm@redhat.com)
- UPSTREAM: 37986: Add `clusterid`, an optional parameter to storageclass.
  (hchiramm@redhat.com)
- Bump to golang-1.8rc1 (ccoleman@redhat.com)
- Add ut test for the function getContainerNameOrID (meng.hao1@zte.com.cn)
- bump(github.com/openshift/origin-web-console):
  5cb015108589cb3e9bbabe953ce86275c5a3a082 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f6709f685529329139b6b61aaf538dbcaf2d85f3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2cb38e1044b8ac640b464f48765d9d7ba22c250f (dmcphers+openshiftbot@redhat.com)
- Add headers that provide extra security protection in browsers
  (jforrest@redhat.com)
- bump(github.com/openshift/origin-web-console):
  04dcf30d6be1481cbb8a51967039015a4741864d (dmcphers+openshiftbot@redhat.com)
- Add annotations to roles. (mkhan@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f3d64d5d546617b431ee4f5a20803c6bba4111c2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2d9fec830c03c77a877d8afc63832dea206ad72e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3c430699d21d47007b3fafdb6736881974c8bfa1 (dmcphers+openshiftbot@redhat.com)
- kubelet must have rw to cgroups for pod/qos cgroups to function
  (decarr@redhat.com)
- bump(github.com/openshift/origin-web-console):
  832cec030ff015081965dd6675e5b0fe98ec02c2 (dmcphers+openshiftbot@redhat.com)
- cluster up: add proxy support (cewong@redhat.com)
- update to compile against bumped glog godep (jminter@redhat.com)
- bump(github.com/golang/glog) 44145f04b68cf362d9c4df2182967c2275eaefed
  (jminter@redhat.com)
- fix broken "failing postCommit default entrypoint" test and reformat for
  clarity (jminter@redhat.com)
- ipfailover - user check script overrides default (pcameron@redhat.com)
- Update help text as per @knobunc review comments and regenerate the docs. Fix
  failing integration test. (smitram@gmail.com)
- Add support to disable the namespace ownership checks. This allows routes to
  claim non-overlapping hosts (+ paths) and wildcards across namespace
  boundaries. Update generated docs and completions. (smitram@gmail.com)

* Wed Jan 18 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.6
- bump(github.com/openshift/origin-web-console):
  c42118856232d57d410b17ca20d8a193cd22995b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  695c74dc0fac848444c79c555fb689c4d8431973 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: <carry>: Pod deletion can be contended, causing test failure
  (ccoleman@redhat.com)
- Better haproxy 503 page (ffranz@redhat.com)
- bump external template examples (bparees@redhat.com)
- Build extended.test in build-cross (ccoleman@redhat.com)
- cluster up: add persistent volumes on startup (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  738a098ce3e0e1ab13b788e812eaba08f90f1146 (dmcphers+openshiftbot@redhat.com)
- Increase deployment test timeout (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  dda5f7ab8684393e6f376ae249f20a8a7d77c137 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b1dcc6f8daa1426f2bcdbcd36878b343ff6c69b9 (dmcphers+openshiftbot@redhat.com)
- Generated changes (maszulik@redhat.com)
- UPSTREAM: 39997: Fix ScheduledJob -> CronJob rename leftovers
  (maszulik@redhat.com)
- Add service UID as x509 extension to service server certs
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: Increase service endpoint test timeout
  (ccoleman@redhat.com)
- only set term if rsh is running /bin/sh (bparees@redhat.com)
- Adding a test for not having a route added if the gateway and the destination
  addresses are the same (fgiloux@redhat.com)
- bump(github.com/openshift/origin-web-console):
  15e4baa64a7d1a39771c32e67753edd0a0ee7cf5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a3da0eff67c443ed5b5a0465913c3a3cf6e93839 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e9ab6af6f5520780c28da2294965c72f57e6d2f4 (dmcphers+openshiftbot@redhat.com)
- add swaggerapi to discovery rules (deads@redhat.com)
- add a test for fetchruntimeartifacts failure (ipalade@redhat.com)
- UPSTREAM: <carry>: Double container probe timeout (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0102c49ad4d39d9bd74d96fd85222e95b3c69743 (dmcphers+openshiftbot@redhat.com)
- Hide DockerImageConfig as well as DockerImageManifest (agladkov@redhat.com)
- Modify comment for pkg/controller/scheduler.go (song.ruixia@zte.com.cn)
- Adding --build-env and --build-env-file to oc new-app (cdaley@redhat.com)
- Uses patch instead of update to mark nodes (un)schedulable
  (ffranz@redhat.com)
- add affirmative output to oc policy / oadm policy (jvallejo@redhat.com)
- Remove vestiges of atomic-enterprise in the codebase (ccoleman@redhat.com)
- Do not reuse pkgdir across architectures (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: 2140: Add 'ca-central-1' region for registry
  S3 storage driver (mfojtik@redhat.com)
- Links to docker and openshift docs updated (mupakoz@gmail.com)
- add log arguments (wang.xiaogang2@zte.com.cn)

* Mon Jan 16 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.5
- bump(github.com/openshift/origin-web-console):
  fa6f2af00717135ec5c8352daf73a3db2a4f4255 (dmcphers+openshiftbot@redhat.com)
- Increase timeout of deployment test (ccoleman@redhat.com)
- Networking tests must be able to schedule on all nodes (ccoleman@redhat.com)
- Generated changes (maszulik@redhat.com)
- Necessary k8s 1.5.2 updates (maszulik@redhat.com)
- UPSTREAM: 39886: Only set empty list for list types (maszulik@redhat.com)
- UPSTREAM: 39834: Ensure empty lists don't return nil items fields
  (maszulik@redhat.com)
- UPSTREAM: 39496: Use privileged containers for host path e2e tests
  (skuznets@redhat.com)
- UPSTREAM: 39059: Don't evict static pods - revert (maszulik@redhat.com)
- UPSTREAM: 38836: Admit critical pods in the kubelet - revert
  (maszulik@redhat.com)
- UPSTREAM: 39114: assign -998 as the oom_score_adj for critical pods (e.g.
  kube-proxy) - revert (maszulik@redhat.com)
- bump(k8s.io/kubernetes): 43a9be421799afb8a9c02d3541212a6e623c9053
  (maszulik@redhat.com)
- UPSTREAM: revert: eb8415dffd1db720429f8ea363defb981d0d9ebe: 39496: Use
  privileged containers for host path e2e tests (maszulik@redhat.com)
- UPSTREAM: revert: c610047330c232d7550ec17d2cfd687a295a5d54: <drop>: Disable
  memcfg notifications until softlockup fixed (maszulik@redhat.com)
- Registry manifest extended test is local only (ccoleman@redhat.com)
- UPSTREAM: <drop>: Disable memcfg notifications until softlockup fixed
  (ccoleman@redhat.com)
- Move kubelet e2e test to serial suite because it saturates nodes
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d8331ea4e4910a556771ae1e5416e389ab8a3fa5 (dmcphers+openshiftbot@redhat.com)
- Add golang source detector for new-app (cdaley@redhat.com)
- bump(github.com/docker/go-units): e30f1e79f3cd72542f2026ceec18d3bd67ab859c
  (ffranz@redhat.com)
- bump(github.com/openshift/origin-web-console):
  865b35e623a66e43acae810a03de9b5b1a94f694 (dmcphers+openshiftbot@redhat.com)
- Switch image quota to user shared informers (maszulik@redhat.com)
- Allow to use selector when listing clusterroles and rolebindings
  (mfojtik@redhat.com)
- return value all use observed (xu.zhanwei@zte.com.cn)
- Fix /.well-known/oauth-authorization-server panic and add test
  (stefan.schimanski@gmail.com)
- remove old version check for sup group test (pweil@redhat.com)
- update generated content (mfojtik@redhat.com)
- Remove pullthrough request from cross-repo mount test (agladkov@redhat.com)
- dind: Ensure help text documents the order of command and options
  (marun@redhat.com)
- dind: Switch OPENSHIFT_SKIP_BUILD to treat non-empty as true
  (marun@redhat.com)
- deploy: add ready replicas to DeploymentConfig (mfojtik@redhat.com)

* Fri Jan 13 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.4
- Merge remote-tracking branch upstream/master, bump origin-web-console 865715f
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  865715f679a193b659083a2f239a4cabb1a10937 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bff896f04a4ece555229734e481d0c5de954b6a0 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 39496: Use privileged containers for host path e2e tests
  (skuznets@redhat.com)
- allow proxy values to be specified with non-http git uris
  (bparees@redhat.com)
- umount openshift local volumes in clean up script (li.guangxu@zte.com.cn)
- bump(github.com/openshift/origin-web-console):
  7e591430787f0a150b62e6fb0a533956235e72fe (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ba28a7432c175c0600330819e303ba4a07638a93 (dmcphers+openshiftbot@redhat.com)
- cluster up: use serverIP on Mac when public host is not specified
  (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  007f1d33fc639a1a99ade6e83ae81e1acf4ccb6a (dmcphers+openshiftbot@redhat.com)
- Add a nil check to Container.SecurityContext (bbennett@redhat.com)
- Add some optimization between command new-app and new-build
  (li.guangxu@zte.com.cn)
- Updating oc cluster up command to include a --image-streams flag
  (cdaley@redhat.com)
- ipfailover keepalived split brain (pcameron@redhat.com)
- Check and notify scripts are not executable (pcameron@redhat.com)
- UPSTREAM: 39493: kubelet: fix nil deref in volume type check
  (sjenning@redhat.com)
- Add a short explination of Subdomain wildcard policy (jtanenba@redhat.com)
- added wildcardpolicy flag to 'oc expose' (jtanenba@redhat.com)

* Wed Jan 11 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.3
- Merge remote-tracking branch upstream/master, bump origin-web-console 65ebe13
  (tdawson@redhat.com)
- Increase time 'oc cluster up' waits for dns to be available
  (agoldste@redhat.com)
- Only call UnrefVNID if the pod's netns is still valid (agoldste@redhat.com)
- bump(github.com/openshift/origin-web-console):
  65ebe1391f7798fdea1f93ce29eb45b932548859 (dmcphers+openshiftbot@redhat.com)
- Update import docker-compose test expectations (agoldste@redhat.com)
- Make test/extended/alternate_launches.sh pass (agoldste@redhat.com)
- Pushing busybox no longer necessary now that we have pull-through
  (agoldste@redhat.com)
- Add missing junit suite declarations (agoldste@redhat.com)
- Disable kube-proxy urls test for now (agoldste@redhat.com)
- Remove unnecessary line (agoldste@redhat.com)
- Set required ReferencePolicy field so tests pass (agoldste@redhat.com)
- Fix extended tests (maszulik@redhat.com)
- Fix hack/test-cmd.sh (maszulik@redhat.com)
- Switch tooling to go 1.7 (maszulik@redhat.com)
- Remove kube .proto files for api groups that no longer exist
  (agoldste@redhat.com)
- Fix defaults test expectations (agoldste@redhat.com)
- Match upstream client changes (agoldste@redhat.com)
- skip DNS config map e2e test (agoldste@redhat.com)
- Make test-end-to-end work (agoldste@redhat.com)
- Fix jenkins admission plugin (maszulik@redhat.com)
- Remove omitEmpty from DeploymentConfigStatus int fields so it will emit them
  even if 0 (agoldste@redhat.com)
- Update User-Agent test expectations (agoldste@redhat.com)
- Fix test-cmd.sh (agoldste@redhat.com)
- Image streams: move BeforeCreate/BeforeUpdate <carry> logic to
  Validate/ValidateUpdate (agoldste@redhat.com)
- Wrap checking for deleted routes in wait.Poll in TestRouterReloadCoalesce
  (agoldste@redhat.com)
- Set PreferredAddressTypes when we create the KubeletClientConfig
  (agoldste@redhat.com)
- Make jobClient use batch/v2alpha1 to support cronjobs (agoldste@redhat.com)
- policy: allow GCController to list/watch nodes (agoldste@redhat.com)
- policy: allow statefulsetcontroller access to statefulsets
  (agoldste@redhat.com)
- SA controller Run() is now blocking; call via goroutine (agoldste@redhat.com)
- Add DestroyFunc to ApplyOptions and tests (maszulik@redhat.com)
- Start kube shared informer factory (agoldste@redhat.com)
- Add support for kube admission plugin initializers (agoldste@redhat.com)
- Enable PodNodeSelector admission plugin (agoldste@redhat.com)
- Use kube's SA cache store/lister (agoldste@redhat.com)
- Use kube serviceaccounts shared informer (agoldste@redhat.com)
- Added certificate to ignored kube commands in tests (maszulik@redhat.com)
- Update bootstrap policy (maszulik@redhat.com)
- Update expected kube api group versions (agoldste@redhat.com)
- Add new arg for iptables min sync duration when creating proxiers
  (agoldste@redhat.com)
- Update master setup to match kube 1.5 changes (agoldste@redhat.com)
- Apply defaults the new way now that conversion and defaulting are separate
  (agoldste@redhat.com)
- Fixes based on updated generated code (agoldste@redhat.com)
- Use generated defaulters (agoldste@redhat.com)
- Regenerate generated files (agoldste@redhat.com)
- Add defaulter-gen comments (agoldste@redhat.com)
- Match clientcmd Factory changes from upstream (agoldste@redhat.com)
- Match upstream change for --allow-missing-template-keys (agoldste@redhat.com)
- UPSTREAM: 39486: Allow missing keys in templates by default
  (agoldste@redhat.com)
- UPSTREAM: <drop>: add origin resource shortcuts to kube shortcut restmapper
  (agoldste@redhat.com)
- UPSTREAM: 38647: make kubectl factory composeable (maszulik@redhat.com)
- Replace legacy kube informers with actual upstream kube informers
  (agoldste@redhat.com)
- Switch deployment controller/utils to use []*RC instead of []RC
  (agoldste@redhat.com)
- Pass sharedInformerFactory through to quota registry (agoldste@redhat.com)
- Fix Rollback signature (agoldste@redhat.com)
- Boring: match upstream moves/refactors/renames (agoldste@redhat.com)
- Implement CanMount() for emptyDirQuotaMounter (agoldste@redhat.com)
- PetSet -> StatefulSet (agoldste@redhat.com)
- ScheduledJob -> CronJob (agoldste@redhat.com)
- Boring: match upstream renames/moves (agoldste@redhat.com)
- Correct Godeps.json (agoldste@redhat.com)
- UPSTREAM: 38908: Remove two zany unit tests (agoldste@redhat.com)
- UPSTREAM: opencontainers/runc: 1216: Fix thread safety of SelinuxEnabled and
  getSelinuxMountPoint (pmorie@redhat.com)
- UPSTREAM: 38339: Exponential back off when volume delete fails
  (agoldste@redhat.com)
- UPSTREAM: 37009: fix permissions when using fsGroup (agoldste@redhat.com)
- UPSTREAM: 38137: glusterfs: Fix all gid types to int to prevent failures on
  32bit systems (agoldste@redhat.com)
- UPSTREAM: 37886: glusterfs: implement GID security in the dynamic provisioner
  (agoldste@redhat.com)
- UPSTREAM: 36437: glusterfs: Add `clusterid`, an optional parameter to
  storageclass. (agoldste@redhat.com)
- UPSTREAM: 38196: fix mesos unit tests (agoldste@redhat.com)
- UPSTREAM: 37721: Fix logic error in graceful deletion (agoldste@redhat.com)
- UPSTREAM: 37649: Fix top node (agoldste@redhat.com)
- bump(golang.org/x/sys) 8d1157a435470616f975ff9bb013bea8d0962067
  (jminter@redhat.com)
- bump(k8s.io/gengo): 6a1c24d7f08e671c244023ca9367d2dfbfaf57fc
  (agoldste@redhat.com)
- Fix imports for genconversion/gendeepcopy to use k8s.io/gengo
  (agoldste@redhat.com)
- bump(google.golang.org/grpc): b1a2821ca5a4fd6b6e48ddfbb7d6d7584d839d21
  (agoldste@redhat.com)
- bump(github.com/onsi/gomega): d59fa0ac68bb5dd932ee8d24eed631cdd519efc3
  (agoldste@redhat.com)
- UPSTREAM: 38603: Re-add /healthz/ping handler in genericapiserver
  (stefan.schimanski@gmail.com)
- UPSTREAM: 38690: unify swagger and openapi in config
  (stefan.schimanski@gmail.com)
- UPSTREAM: <drop>: add ExtraClientCACerts to SecureServingInfo
  (stefan.schimanski@gmail.com)
- UPSTREAM: <drop>: add missing index to limit range lister
  (agoldste@redhat.com)
- Match upstream singular -> singleItemImplied rename (agoldste@redhat.com)
- UPSTREAM: 39038: Fix kubectl get -f <file> -o <nondefault printer> so it
  prints all items in the file (agoldste@redhat.com)
- UPSTREAM: 38986: Fix DaemonSet cache mutation (agoldste@redhat.com)
- UPSTREAM: 38631: fix build tags for !cgo in kubelet cadvisor
  (agoldste@redhat.com)
- UPSTREAM: 38630: Fix threshold notifier build tags (agoldste@redhat.com)
- UPSTREAM: coreos/go-systemd: 190: util: conditionally build CGO functions
  (agoldste@redhat.com)
- bump(github.com/onsi/ginkgo/ginkgo): 74c678d97c305753605c338c6c78c49ec104b5e7
  (agoldste@redhat.com)
- bump(k8s.io/client-go): ecd05810bd98f1ccb9a4558871cb0de3aefd50b4
  (agoldste@redhat.com)
- bump(github.com/juju/ratelimit): 77ed1c8a01217656d2080ad51981f6e99adaa177
  (agoldste@redhat.com)
- UPSTREAM: 38294: Re-use tested ratelimiter (agoldste@redhat.com)
- bump(k8s.io/kubernetes): 225eecce40065518f2eabc3b52020365db012384
  (agoldste@redhat.com)
- Update copy-kube-artifacts to current k8s (maszulik@redhat.com)
- Update group versions in genprotobuf (agoldste@redhat.com)
- Add generators for defaulters, openapi, listers (agoldste@redhat.com)
- bump(github.com/rackspace/gophercloud):
  e00690e87603abe613e9f02c816c7c4bef82e063 (agoldste@redhat.com)
- bump(github.com/onsi/gomega): d59fa0ac68bb5dd932ee8d24eed631cdd519efc3
  (agoldste@redhat.com)
- bump(github.com/jteeuwen/go-bindata):
  a0ff2567cfb70903282db057e799fd826784d41d (agoldste@redhat.com)
- bump(github.com/fsnotify/fsnotify): f12c6236fe7b5cf6bcf30e5935d08cb079d78334
  (agoldste@redhat.com)
- Add extra required dependencies to hack/godep-save.sh (agoldste@redhat.com)
- godepchecker: add option to compare forks (agoldste@redhat.com)
- Pin godep to v75 (agoldste@redhat.com)
- bump(golang.org/x/sys): revert: 264b1bec8780c764719c49a189286aabbeefc9c2:
  (bump to 8d1157a435470616f975ff9bb013bea8d0962067) (agoldste@redhat.com)
- UPSTREAM: revert: bedff43594597764076a13c17b30a5fa28c4ea76: docker/docker:
  <drop>: revert: 734a79b: docker/docker: <carry>: WORD/DWORD changed
  (agoldste@redhat.com)
- origin: revert: 5d5c0c90bea65fea41a2b72d6e28bf0800696f95: 37649: Fix top node
  (agoldste@redhat.com)
- UPSTREAM: revert: 5d5c0c90bea65fea41a2b72d6e28bf0800696f95: 37649: Fix top
  node (agoldste@redhat.com)
- UPSTREAM: revert: 372e9928f1ada5f49d4c44684f7151798bcca200: 37721: Fix logic
  error in graceful deletion (agoldste@redhat.com)
- UPSTREAM: revert: 74f64a736154ab6bcc508abaa6d045828b7eacd8: 38196: fix mesos
  unit tests (agoldste@redhat.com)
- bump(github.com/heketi/heketi): revert:
  7fd7ced6cb2d2eeaf66408bb7b2e4491f7036cbb (bump to
  28b5cc4cc6d2b9bdfa91ed1b93efaab4931aa697) (agoldste@redhat.com)
- UPSTREAM: revert: 9bffc75ed9a26d0cd226febc93cad61d0a500d77: 37886: glusterfs:
  implement GID security in the dynamic provisioner (agoldste@redhat.com)
- UPSTREAM: revert: 6974f328a5577926c8a62a4ed456ed388f9e477d: 38137: glusterfs:
  Fix all gid types to int to prevent failures on 32bit systems
  (agoldste@redhat.com)
- UPSTREAM: revert: 64982639ab1321fb1a1332dd78101a1ff5f8c59a: 37009: fix
  permissions when using fsGroup (agoldste@redhat.com)
- UPSTREAM: revert: b049251dc647418edf2b1a700d1a52ce85dc9696: 38339:
  Exponential back off when volume delete fails (agoldste@redhat.com)
- UPSTREAM: revert: 33ee3c4f6a4a3b07d9c758c0271a950b6a52d26a:
  opencontainers/runc: 1216: Fix thread safety of SelinuxEnabled and
  getSelinuxMountPoint (agoldste@redhat.com)
- UPSTREAM: revert: 8f14f3b701aa10757d1edad3a566672c72b7b55f: <drop>: Handle Go
  1.6/1.7 differences in upstream (agoldste@redhat.com)
- if2If at the begining of one line (lu.jianping2@zte.com.cn)
- cluster up: add pod networking check (cewong@redhat.com)
- Adding aos-3.5 tito build queue (tdawson@redhat.com)
- Update ose_images.sh - 2017-01-09 (tdawson@redhat.com)
- modify comment for CommandFor (xu.zhanwei@zte.com.cn)
- Add extended test for manifest migration (agladkov@redhat.com)
- Migrate manifest from Image when receiving get-request (agladkov@redhat.com)
- Add tests for pullthroughManifestService (agladkov@redhat.com)
- Save manifest in docker-registry and make pullthrough for manifests
  (agladkov@redhat.com)
- Move ManifestService to separate object (agladkov@redhat.com)

* Mon Jan 09 2017 Troy Dawson <tdawson@redhat.com> 3.5.0.2
- Merge remote-tracking branch upstream/master, bump origin-web-console 6f48eb0
  (tdawson@redhat.com)
- remove openshift.io/container.%%s.image.entrypoint annotation and clarify
  new-app maximum name length (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6f48eb058bdf5e1e8b0fe75f124530be44acd1aa (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  abdc02074315ba9b3a7e26c2ca081aacbfa9c173 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1bea253df7f420ee7a27478e6c7f1b38bad4bf17 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6f0b723c2f9c6473dab380f8b77c93e80774660a (dmcphers+openshiftbot@redhat.com)
- Implement secret injector admission controller for buildconfigs
  (jminter@redhat.com)
- bump(github.com/openshift/source-to-image):
  8de9f04445c8a58f244df30f0798d1d1fdb3023a (ipalade@redhat.com)
- bump(github.com/openshift/origin-web-console):
  81127a4039a90ce5e790b3a12b602aaf6950de91 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3656be70e9d4c88a91818ca148ce3aa3cc000cbd (dmcphers+openshiftbot@redhat.com)
- always set the commitid env variable in the output image (bparees@redhat.com)
- cluster up: do not ignore public-hostname on Mac with Docker for Mac
  (cewong@redhat.com)
- ipfailover - fix typo in script (pcameron@redhat.com)
- Fix manifest verification if pullthrough enabled (agladkov@redhat.com)
- fix typo and  drop = nil drop []string (yu.peng36@zte.com.cn)
- bump(github.com/openshift/origin-web-console):
  f0e8144f93ef69e996933c572fc6ecee619d2ebe (dmcphers+openshiftbot@redhat.com)
- Add --default-mode flag to oc set volume (rymurphy@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b55a35d8f6f7aed63de5a2d6e19e52c9053e841e (dmcphers+openshiftbot@redhat.com)
- Ensure default ingress cidr is a private range (marun@redhat.com)
- Fix 404 error in Jenkins plugin extended tests (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  025ded5e81d0bba659e5b88ba1d458bc751808eb (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  76040f12bbb6849467bec657bdd1d9ab20835ffd (dmcphers+openshiftbot@redhat.com)
- Ensure an invalid bearer token returns an error (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ddceed94f956f73363f3113eaaa73341e6f40fa2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ac02e04d55d69d23c2f4b701bca729d00dd2dfd8 (dmcphers+openshiftbot@redhat.com)
- Bug 1348174 - error on non-existing tag when importing from
  .spec.dockerImageRepository (maszulik@redhat.com)
- enable hostpath provisioning (somalley@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0b1435e239c27e3c52ad745a76ea29b5f8ee1ffd (dmcphers+openshiftbot@redhat.com)
- reduce some empty line && code, add some TODO comments (shiywang@redhat.com)
- clarify unknown type log message (bparees@redhat.com)
- Fix printing args in image pruning (maszulik@redhat.com)
- Disable registry test that compares sha256 checksum of layers due to
  go1.6/1.7 incompatibility (mfojtik@redhat.com)
- Label idling tests [local] for now, add more tests to suite
  (ccoleman@redhat.com)
- Use [local] consistently (ccoleman@redhat.com)
- note about net.ipv4.ip_forward=1 (Oleksii.Prudkyi@gmail.com)
- Merge extended test suite JUnit files to avoid duplicate skips
  (ccoleman@redhat.com)
- Allow additional extended tests to be skipped with TEST_EXTENDED_SKIP
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  34536058a2d795587040db3a16505360b1e9eb1b (dmcphers+openshiftbot@redhat.com)
- adapt secret key names to new policy (raaflaub@puzzle.ch)
- These tests require local access (ccoleman@redhat.com)
- Router tests should not assume access to pod network (ccoleman@redhat.com)
- Scheduler predicates test should run without node constraints
  (ccoleman@redhat.com)
- Fixing typos (mmahut@redhat.com)
- Allow much higher extended test bursting by increasing retry
  (ccoleman@redhat.com)
- Disable color on extended test suite (ccoleman@redhat.com)
- UPSTREAM: onsi/ginkgo: 318: Capture test output (ccoleman@redhat.com)
- Disable pullthrough in test case (ccoleman@redhat.com)
- Remove backup files (maszulik@redhat.com)
- Apply defaulting in image stream import (ccoleman@redhat.com)
- generated: swagger (ccoleman@redhat.com)
- Add an extended test for deployment resolution (ccoleman@redhat.com)
- Add reference policy to describe output (ccoleman@redhat.com)
- Respect tag referencePolicy in builds and deployments (ccoleman@redhat.com)
- Enable pullthrough by default (ccoleman@redhat.com)
- generated: api changes (ccoleman@redhat.com)
- Support a new image stream tag reference policy (ccoleman@redhat.com)
- Add extended tests for pipeline examples (cewong@redhat.com)
- Do not print findmnt on startup (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6bd844376083142cc89c7ce64ef39d49a887a8d0 (dmcphers+openshiftbot@redhat.com)
- Pass no focus spec in `make test-extended` when unset (skuznets@redhat.com)
- update debugging doc and sample-app README (li.guangxu@zte.com.cn)
- Fix hard coded binary path (tdawson@redhat.com)

* Thu Dec 22 2016 Troy Dawson <tdawson@redhat.com> 3.5.0.1
- Update version to 3.5.0.0 (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ab61c6fca709340f431605e92cedfcac447d998b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4e95180904ce953eb73793a40476af51217ea01f (dmcphers+openshiftbot@redhat.com)
- redo forcepull test so localnode is not required (gmontero@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0279eece98b9c8a3338be0e32968bfb22329d2af (dmcphers+openshiftbot@redhat.com)
- deprecate images based on EOL SCL packages (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e66c5add6055e8e2c15e8d277df590692fae8c7b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ea2a5b6d21c5a720baabccf53f4e5117cc95219f (dmcphers+openshiftbot@redhat.com)
- Return non-error msg on `oc status` if no projects (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  171a0675e7924d835afed76c933215d142700c7f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  723de04d1e7707b6d9801816654b4a5efa9473b4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  019df5167c23d499463e332992b85b1d0b5d58cc (dmcphers+openshiftbot@redhat.com)
- s2i_extended_build: update repo url. (vsemushi@redhat.com)
- router: Cleanup logging of routing state changes (marun@redhat.com)
- router: clean up service unit configuration (marun@redhat.com)
- router: Avoid reloads when route configuration hasn't changed
  (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b3ce1b68237db08e69e823d00c608af213192a5e (dmcphers+openshiftbot@redhat.com)
- Add a golang-1.8 release image (ccoleman@redhat.com)
- Reuse test artifacts (ccoleman@redhat.com)
- Don't use findmnt if it isn't there (ccoleman@redhat.com)
- Create a package for non-Linux builds (ccoleman@redhat.com)
- Add RHSCL 2.3 docker images to image streams (hhorak@redhat.com)
- bump(github.com/openshift/origin-web-console):
  664a2473bd2b2155499d9210579713431f6a07b6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6010439649cd7550ea95adef27e3698979ee24f4 (dmcphers+openshiftbot@redhat.com)
- Added manifest verification (miminar@redhat.com)
- Registry: moved manifest schema operations to new files (miminar@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7973c2b9c77a236f7ff9516467cb46ee25212624 (dmcphers+openshiftbot@redhat.com)
- add debug to failure reason extended tests (bparees@redhat.com)
- Fix group name in EgressNetworkPolicy-related rules (danw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  960a01bc0bb5d9d2ee129885fbcc8683f99f1a61 (dmcphers+openshiftbot@redhat.com)
- do s2i git cloning up front, not in s2i itself (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eb5c33657e1133bb1b5579405446cdad191ce3d6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ef858d27b8ffb68e1d801f527363d4223608a8e7 (dmcphers+openshiftbot@redhat.com)
- add namespace selector field to crq describe output (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a20a6942d2fdbddb0f71070027c2116c80649313 (dmcphers+openshiftbot@redhat.com)
- Updating usage of the --metrics flag to create a job (cdaley@redhat.com)
- s2i bump related code changes (bparees@redhat.com)
- bump(github.com/openshift/source-to-image):
  3931979363ccb57da0c66c8aa9bf1a1319e492e5 (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  32c3a25c06670b040d076eb1e5fd6cecf443378c (dmcphers+openshiftbot@redhat.com)
- deploy: generated changes for configurable activeDeadlineSeconds
  (mkargaki@redhat.com)
- deploy: make activeDeadlineSeconds configurable for deployer pods
  (mkargaki@redhat.com)
- HAProxy Router: Add option to use PROXY protocol (miciah.masters@gmail.com)
- Add wildfly image stream to maven pipeline example (cewong@redhat.com)
- Make the DIND image creation script executable (skuznets@redhat.com)
- Fix irrelevant error messages during dind setup (danw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  232ddec5343216575c90a69452e167f51efe1bde (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d0e340c5a1144fc04b3cf2c15af7a8b12970e482 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  247492a613162b594d88664baab8ec0fe479912b (dmcphers+openshiftbot@redhat.com)
- Show only project name when using the short option (chmouel@chmouel.com)
- regenerate man page for rsh (mfojtik@redhat.com)
- enable deployments and replica sets in rsh (mfojtik@redhat.com)
- deployments: lowercase the progress messages for deployment configs to match
  upstream (mfojtik@redhat.com)
- Don't mention internal error when docker registry is unreachable
  (mmilata@redhat.com)
- Remove `$TERM` export from our Bash library (skuznets@redhat.com)
- Introduce RestrictUsersAdmission admission plugin (miciah.masters@gmail.com)
- bump(github.com/openshift/origin-web-console):
  8e405523fe7ca626000bbd6fbb69ad79c1fce453 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cfc61cde705639c0d7e23e92c9329e0176d17f65 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2446d7f3b0e38786bb912c0a6ea978eacffd16f3 (dmcphers+openshiftbot@redhat.com)
- Test{NewAppSourceAuthRequired,Dockerfile{Path,From}}: create temp dir at the
  right location. (vsemushi@redhat.com)
- Updated image stream templates and db-templates (rymurphy@redhat.com)
- bump(github.com/openshift/origin-web-console):
  076ed5a47ef7759362bd0418f291625689b1a68a (dmcphers+openshiftbot@redhat.com)
- rebase (li.guangxu@zte.com.cn)
- deploy: revert proportional recreate timeout and default to 10m
  (mfojtik@redhat.com)
- new-app/new-build/process: read envvars/params from file (mmilata@redhat.com)
- deploy: fix timeoutSeconds for initial rolling rollouts (mfojtik@redhat.com)
- bump(github.com/joho/godotenv): 4ed13390c0acd2ff4e371e64d8b97c8954138243
  (mmilata@redhat.com)
- deploy: make retryTimeout proportional to deployer Pod startTime
  (mfojtik@redhat.com)
- Add hack/build-dind-images.sh (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  df2b5b3e01bd65d3ecfa805585ca4bffdf7ae4be (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2206b62d538a3afbfac07ed08d23cdf6fb270efe (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  43e511fc80cbf3a7b25dd112da0157d7943d88d6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  848b7943baad8af222451bf2ff6c3f8100671e9f (dmcphers+openshiftbot@redhat.com)
- Run hack/update-generated-completions.sh and hack/update-generated-docs.sh
  (vsemushi@redhat.com)
- oadm ca, oadm create-node-config, oadm create-api-client-config, openshift
  start: add --expire-days and --signer-expire-days options.
  (vsemushi@redhat.com)
- Run all verification in Makefile rather than exiting early
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9ab85d93337833b591be71cc4b94aa8240436fbb (dmcphers+openshiftbot@redhat.com)
- Improve ipfailover configurability (pcameron@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2ab961f7c76e889a3239c3debfcb02477ff666e3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cbbfb2e773ba326043ab3ac06e7bca03be293522 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7243a21adfb252f2e0d401c9d25394067b72f3d7 (dmcphers+openshiftbot@redhat.com)
- oc process: fix go template output, add jsonpath (mmilata@redhat.com)
- router: Minimize reloads for removal and filtering (marun@redhat.com)
- Review feedback from builds (ccoleman@redhat.com)
- router: Fix detection of initial sync (marun@redhat.com)
- router: Ensure reload on initial sync (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  998521beaf100476eaede2a9c1fbd37a65320735 (dmcphers+openshiftbot@redhat.com)
- router: bypass the rate limiter for the initial commit (marun@redhat.com)
- Fix broken router stress test (marun@redhat.com)
- Turn on jUnit output by default for `make` targets (skuznets@redhat.com)
- fix for bz1400609; if the node status flips on the order of ip addresses
  (when there are multiple NICs to report), do not let the SDN chase it
  (rchopra@redhat.com)
- generated: protobuf definition changes in go 1.7 due to tar changes
  (ccoleman@redhat.com)
- UPSTREAM: <drop>: Handle Go 1.6/1.7 differences in upstream
  (ccoleman@redhat.com)
- Make go 1.7 the default build tool for the release (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  617df3f22248be58636641dc4e7573c03e979b2e (dmcphers+openshiftbot@redhat.com)
- Update docs wrt multiple --env/--param arguments (mmilata@redhat.com)
- Suggest oadm drain instead of oadm manage-node drain (mfojtik@redhat.com)
- oc cluster up: work around docker attach race condition (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3e63c9c5cc314777ca9306b2396093acd9466746 (dmcphers+openshiftbot@redhat.com)
- Punctuation corrected (aktjha@gmail.com)
- Redo isolation OVS rules (danw@redhat.com)
- Split out multitenant-specific and single-tenant-specific SDN code
  (danw@redhat.com)
- Renumber OVS tables (danw@redhat.com)
- Remove useless OVS-related error returns (danw@redhat.com)
- Make the OVS flow tests more generic (danw@redhat.com)
- missed punctuation (aktjha@gmail.com)
- Use the new build logic in *-build-images (ccoleman@redhat.com)
- Add a common function for building images supporting imagebuilder
  (ccoleman@redhat.com)
- Update to Go 1.7.4 in the spec (ccoleman@redhat.com)
- Fix formatting (dmcphers@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8f964a8d439d7a6fbecf10dd9d416e1a76a1684c (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3579a803486cd3d23e7559814d627e607ee28b62 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  50c1d5e4f657ec5b92c3f332788fac3101b3f1e6 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: opencontainers/runc: 1216: Fix thread safety of SelinuxEnabled and
  getSelinuxMountPoint (pmorie@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a3a3d3925ad674948fdfb8b8c85a390c63d5062d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d7af2d96749fe46359b3d5f977131a408de8725d (dmcphers+openshiftbot@redhat.com)
- more debug for failing db queries (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9be69b89076bb4b44f88a89faecf7e65cadc8ca7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1265ebcb846113e5471da1149b69a4e9ff45f05e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1fb702b27ad4f2c47cbd791122555ed14afd7094 (dmcphers+openshiftbot@redhat.com)
- Add `test-extended` target to the Makefile (skuznets@redhat.com)
- Provide the directory for Ginkgo jUnit output explicitly
  (skuznets@redhat.com)
- Logs wait for first build from bc with ConfigChange trigger
  (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d9143865d5db98b4031688692d19cc377db467f4 (dmcphers+openshiftbot@redhat.com)
- Fix excluder incorrectly creating exclude line. (tdawson@redhat.com)
- Add advanced pipeline examples (cewong@redhat.com)
- Fixed Bashisms and temporary directory use in kubeconfig tests
  (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  10c6ca8502411aaecd08360233749493c2915fc8 (dmcphers+openshiftbot@redhat.com)
- rework build list order for oc status (gmontero@redhat.com)
- generated man pages and completetion (mfojtik@redhat.com)
- update extended test and add test cmd for retry (mfojtik@redhat.com)
- deploy: add retry subcommand for rollout (mfojtik@redhat.com)
- deploy: update doc for maxUnavailable (mkargaki@redhat.com)
- Seed math/rand on start (jliggitt@redhat.com)
- UPSTREAM: 38339: Exponential back off when volume delete fails
  (gethemant@gmail.com)
- Add List to token client (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e06dfb55f9d0ee29f2da7db9243e695e4c759acd (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 37009: fix permissions when using fsGroup (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eee4d13f8fca1f0777df49bf352047ebdfa45119 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d257d664947af519bcb0cbaf990ca304a2b4e432 (dmcphers+openshiftbot@redhat.com)
- new-app hidden imagestreams: fix behaviour when no tag is specified
  (jminter@redhat.com)
- add test for s2i source fetch failure (ipalade@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b959663a3ee4d00f16ca0ee7144f795356a09bf4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e28544ec35bcf585416c1bfc92c74a6235aac2cb (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c9b7feb34a35f6ca88158ee381c6a5b991befb7f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  33553e5d5a87b324287aa68a681f3198d33b384a (dmcphers+openshiftbot@redhat.com)
- generate man and bash completion (mfojtik@redhat.com)
- deploy: use rollout cancel in extended test (mfojtik@redhat.com)
- deploy: deprecate --enable-triggers in favor of set triggers
  (mfojtik@redhat.com)
- deploy: add rollout cancel command and deprecate deploy --cancel
  (mfojtik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8fd9c66ca45a40d134a941448b1718d80b998f3a (dmcphers+openshiftbot@redhat.com)
- Prevent jenkins from being killed prematurely (wtrocki@redhat.com)
- UPSTREAM: 38196: fix mesos unit tests (mkargaki@redhat.com)
- Review 1: Comments and bashisms (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a9f4523243228229790c7c39375c0a128d743cdc (dmcphers+openshiftbot@redhat.com)
- add plugin ext test for build clone; verify build triggeredBy; fix build
  clone endpoint setting of triggered by (gmontero@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d264ce48c34d113f5d774f32be52401ae7ad9418 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e024781eb6f4272e66ffc63561ca9c5d63b33027 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a8bd1148758925ca3b5bd366a92b82c0bc86fe73 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8baf00b8624ff918a4c754f01c6fba48e6bc62a0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f697f5017e25cf30e9d5d263568e21c060a6b981 (dmcphers+openshiftbot@redhat.com)
- Wait for old pods to terminate before proceeding to Recreate
  (mkargaki@redhat.com)
- bump(github.com/openshift/origin-web-console):
  99070c15973a5714a3b2826b612f20a462cca0e8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  35b232328cb1dc92fe16b74a4644ab5b495e029d (dmcphers+openshiftbot@redhat.com)
- fix typo (yu.peng36@zte.com.cn)
- UPSTREAM: 38137: glusterfs: Fix all gid types to int to prevent failures on
  32bit systems (obnox@samba.org)
- bump(github.com/openshift/origin-web-console):
  2117097e332b8dea4cd0507439dae217cf39f7d9 (dmcphers+openshiftbot@redhat.com)
- Use default cert dir for oc cluster up client if DOCKER_TLS_VERIFY is set
  (jimmidyson@gmail.com)
- openshift/origin-release: install openssl. (vsemushi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4fd08fea26309fa05100480ce088ac11b5b54ea8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  aa5d310211320d1131ed12f5ad7f9b8e35b70871 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6cbd1a7796abc2ffb0008f6045dc70ca502d0e4a (dmcphers+openshiftbot@redhat.com)
- deploy: generated changes for lastUpdateTime (mkargaki@redhat.com)
- deploy: bring deployment config conditions on par with upstream
  (mkargaki@redhat.com)
- Increase timeouts for idling extended test (mfojtik@redhat.com)
- deploy: cleanup strategy code (mkargaki@redhat.com)
- make /sys/devices/virtual/net r/w for kubelet under oc cluster up
  (jminter@redhat.com)
- UPSTREAM: 37886: glusterfs: implement GID security in the dynamic provisioner
  (obnox@redhat.com)
- bump(github.com/heketi/heketi):28b5cc4cc6d2b9bdfa91ed1b93efaab4931aa697
  (hchiramm@redhat.com)
- Use upstream path segment name validation (jliggitt@redhat.com)
- Compare object DN to structured baseDN (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  8bca523d03e6434245744e16f08194f17baa70bd (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 37721: Fix logic error in graceful deletion (decarr@redhat.com)
- UPSTREAM: 37649: Fix top node (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ac395f8a59a771f5c1cedaa3bed0ec97057bb996 (dmcphers+openshiftbot@redhat.com)
- Reconcile deleted namespaces out of cluster quota status
  (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  281de6b73177b588a76940f6c96acbb4d2313fce (dmcphers+openshiftbot@redhat.com)
- new-app: appropriately warn/error on circular builds (gmontero@redhat.com)
- generated: docs (ccoleman@redhat.com)
- Add `oc serviceaccounts create-kubeconfig` to simplify bootstrapping
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9e5a65b9d95a76f10abebc9a59a4d355b4e24641 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  964762894db8c9cc9e4d94856f4e1c31b08d62ea (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  642c6f354adaeaea8651db8e9c798b2b07ca4ae7 (dmcphers+openshiftbot@redhat.com)
- build gitserver image FROM origin, not origin-base (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4aaf5da4600881a7d63ef8ce7a519c73d04bd118 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/docker/distribution): Add oss storage driver
  (agladkov@redhat.com)
- Enable oss storage driver (agladkov@redhat.com)
- oc new-app support for "hidden" tag in imagestreams (jminter@redhat.com)
- generated: docs (ccoleman@redhat.com)
- Mirror blobs to the local registry on pullthrough (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6aabd724a1069c5b95a301db5eede6c7171549d0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e5dcaf2e91b1d22e610a66cd63d546ca9c3448e1 (dmcphers+openshiftbot@redhat.com)
- bump(gopkg.in/ldap.v2): 8168ee085ee43257585e50c6441aadf54ecb2c9f
  (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6ea5608efac317565723c6ecf28030b3f0fec8b9 (dmcphers+openshiftbot@redhat.com)
- [BZ1394716] Report false diagnostic message for curator-ops DeploymentConfig
  (jcantril@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0eb6441b19746ea39df494bc8ad7220a473fa6df (dmcphers+openshiftbot@redhat.com)
- Changing quickstart credentials to secrets (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  04f331e68ee15916b5f2281b55d892740074e141 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9737260a421e4c459e9f4b45042345a7468f4a4d (dmcphers+openshiftbot@redhat.com)
- added an enviroment variable for balance alg. and ability to disable route
  cookies (jtanenba@redhat.com)
- Removed reference to gssapi variable (rymurphy@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ab6c1a4d04e795ca7ce9fee10c054c497178e740 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c4192691030a79212258d7ff3436b7fba0f011fa (dmcphers+openshiftbot@redhat.com)
- Fix typo introduced in 4b54d01c (mmilata@redhat.com)
- Fail new builds that can't start build pod because it already exists
  (mmilata@redhat.com)
- Add missing arguments to glog.Infof (mmilata@redhat.com)
- bump(github.com/openshift/origin-web-console):
  26fea747e4dac8b25338c07c4c9c6acff9c1b82e (dmcphers+openshiftbot@redhat.com)
- add some optimiztion for new-build command (li.guangxu@zte.com.cn)
- Fix excluder usage bug (tdawson@redhat.com)
- set env vars on oc new-app of template (gmontero@redhat.com)
- UPSTREAM: revert: 734a79b: docker/docker: <carry>: WORD/DWORD changed
  (jminter@redhat.com)
- Implement inscureEdgeTermination options for reencrypt and pasthrough routes
  reencrypt routes work the same as edge routes with Allow, Redirect, and None
  (jtanenba@redhat.com)
- Consideration of imagePullPolicy on lifecycle hook execution
  (roy@fullsix.com)
- bump(github.com/openshift/origin-web-console):
  d4a5a52687e9324dc8c898967b4b133c5017259e (dmcphers+openshiftbot@redhat.com)
- bump(golang.org/x/sys) 8d1157a435470616f975ff9bb013bea8d0962067
  (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  eea002ef608401acc426406d835431e8dd30a527 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5a88c56d465a99b267c59f5c0f9879158f04e197 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b503ddd5e3531fc01d415a55d76980fbecf4ea86 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3e75b27f3c8defc1e22cfed22a7e8e642a24ee1b (dmcphers+openshiftbot@redhat.com)
- add failure reasons (ipalade@redhat.com)
- Remove github.com/RangelReale from preload-remote list (jliggitt@redhat.com)
- Test OAuth state encoding (jliggitt@redhat.com)
- bump(github.com/RangelReale/osin): 1c1a533224dd9c631fdd8df8851b167d24cabe96
  (jliggitt@redhat.com)
- Fix godoc comments and typo in command description. (vsemushi@redhat.com)
- Extend commitchecker to call hack/godep-restore.sh when needed
  (maszulik@redhat.com)
- Add preloading google.golang.org/cloud to godep-restore.sh
  (maszulik@redhat.com)
- bump(github.com/jteeuwen/go-bindata):
  bfe36d3254337b7cc18024805dfab2106613abdf (maszulik@redhat.com)
- bump(github.com/onsi/ginkgo): 74c678d97c305753605c338c6c78c49ec104b5e7
  (maszulik@redhat.com)
- bump(github.com/docker/docker): b9f10c951893f9a00865890a5232e85d770c1087
  (maszulik@redhat.com)
- bump(github.com/docker/distribution):
  12acdf0a6c1e56d965ac6eb395d2bce687bf22fc (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image):
  5741384e376d9c61b5b36156003ede6698ca563b (maszulik@redhat.com)
- bump(github.com/containernetworking/cni):
  52e4358cbd540cc31f72ea5e0bd4762c98011b84 (maszulik@redhat.com)
- bump(k8s.io/kubernetes): a9e9cf3b407c1d315686c452bdb918c719c3ea6e
  (maszulik@redhat.com)
- bump(k8s.io/client-go): d72c0e162789e1bbb33c33cfa26858a1375efe01
  (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  14fe43790c0b1b3f45f03177ef49365d56062e29 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ec1d6de5eab9d5d5d1ab4fb7afb303b5c4e4bf7c (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4d79ddb41a7c4dd1deae999cce2cbb60595c4421 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cf4e10786748312a930219b8da77e24856685f5a (dmcphers+openshiftbot@redhat.com)
- add controller to regenerate service serving certs (deads@redhat.com)
- update canRequestProjects check to include list,create (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4cd45b9fdfd65daa561f672fe90bfd1c32ee3e45 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b94910ff08fdeac83f1eadb2649bfd68b17d9b74 (dmcphers+openshiftbot@redhat.com)
- Adding extended tests for buildconfig postCommit (jupierce@redhat.com)
- Better formatting for jenkinsfile example (dmcphers@redhat.com)
- Fix typos (rhcarvalho@gmail.com)
- deploy: drop unnecessary resource handlers to reduce dc requeues
  (mkargaki@redhat.com)
- Move verify-upstream-commits to a separate make target (maszulik@redhat.com)
- oc: show ready pods next to deployments (mkargaki@redhat.com)
- Fail hard when the commit checker finds no Git repo (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f892e0aa156399e563c9ff92ba43f87135afcd03 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b8f3206572b689e1c899c19e30dfba60d9e9d585 (dmcphers+openshiftbot@redhat.com)
- Copy creationTimestamp from the image (mfojtik@redhat.com)
- Include createrepo in release images (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  151dc5f077ce608cf0239a94ca1adf6d7aac0689 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9061893ef691e6ca2ae13383f1eb71a74442d662 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  66ae612a7b33a79b2e537eff1d2ac1ddf602086f (dmcphers+openshiftbot@redhat.com)
- Fix a multiple-pointers-to-single-loop-variable bug in EgressNetworkPolicy
  (danw@redhat.com)
- Added logic to save JUNIT log with ENV vars (rymurphy@redhat.com)
- bump(github.com/Microsoft/go-winio) 24a3e3d3fc7451805e09d11e11e95d9a0a4f205e
  (jminter@redhat.com)
- Update ose_images.sh - 2016-11-28 (tdawson@redhat.com)
- Regen swagger spec (agoldste@redhat.com)
- UPSTREAM: <carry>: Remove DeprecatedDownwardAPIVolumeSource from protobuf
  (agoldste@redhat.com)
- bump(github.com/openshift/origin-web-console):
  75f0b8d879fac596173dd3735e4ee96e16c9feb2 (dmcphers+openshiftbot@redhat.com)
- added block arguments to remove jenkins deprecation warnings
  (matthias.luebken@gmail.com)
- add *.csproj support to examples/gitserver/hooks/detect-language
  (jminter@redhat.com)
- bump(github.com/Azure/go-ansiterm) 7e0a0b69f76673d5d2f451ee59d9d02cfa006527
  (backwards) (jminter@redhat.com)
- fix the mistake link type (yu.peng36@zte.com.cn)
- Use responsive writer only in help (ffranz@redhat.com)
- Switch to internalclientset - interesting changes (maszulik@redhat.com)
- Switch to internalclientset - boring changes (maszulik@redhat.com)
- rebase and add some change as specified (li.guangxu@zte.com.cn)
- rename jenkins extended test file (bparees@redhat.com)
- Move test suite extended/cmd/oc-on-kube into extended/cmd.
  (vsemushi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d5fa2bbc04c34b74fcd95c8b8c8e3335e13fada8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  46dd41331363efaf22714b1d6af6c533db31d111 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  339a32eff93ba3a1298717fe2e9f82f55ea0d884 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2889c1d747406b85291102cdbf33faa0f3b4fbd9 (dmcphers+openshiftbot@redhat.com)
- Migrate build environment utilities from `common.sh` (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2a982a15c90a7b5da47aaab58fb50a952124a82e (dmcphers+openshiftbot@redhat.com)
- Deprecate process -v/--value in favor of -p/--param (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b0c62bb12e51bb660bc68cbcee5b6c15eebd3dc4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  95b0902ecb3627301fedf2a0a6855cbd5e8c39a2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bc496f8b3e52ad0e6ea5f1e0caaccbd0e60e6850 (dmcphers+openshiftbot@redhat.com)
- make login, project, and discovery work against kube with RBAC enabled
  (deads@redhat.com)
- UPSTREAM: 37296: Fix skipping - protobuf fields (agoldste@redhat.com)
- bump(github.com/openshift/origin-web-console):
  da4a37d40751c459a7d0fad33ab92006efc14f78 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  72e0f59571cef3b9af904088bdbc4e2154d6c6d3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7389ce5638dfe4078a7ac24f6e2fd8e976e5c819 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  23682f14ee9a19931c72c106ed9c6a781b1bfcab (dmcphers+openshiftbot@redhat.com)
- Redirect to server root if login flow is started with no destination
  (jliggitt@redhat.com)
- fix image input extended test (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a60cd15356e51166f4e7a8d5b06760b003edcbd5 (dmcphers+openshiftbot@redhat.com)
- dind: Remove mention of br_netfilter from docs (marun@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cd4bfdc1870fc1edbce797833d7c231c7e9b3bfd (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  843d4aee83dc149fe9271f2cb403512c85bbe5c8 (dmcphers+openshiftbot@redhat.com)
- add jenkins plugin create/delete obj test; allow for jenkins mem mon disable
  (gmontero@redhat.com)
- bump(github.com/openshift/origin-web-console):
  82519db788b66218445f600dd50f22d5e8dec98f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  00e62d07a86341f986d36a3fda9effb89bbdd4f5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  07653ad748a18fd5e55a10c94f7201916cda9a9f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  be993532e28dae2843df42ac5a5c234c05533ea1 (dmcphers+openshiftbot@redhat.com)
- Bug 1339754 - Fix tag sorting according to semantic versioning rules
  (maszulik@redhat.com)
- Remove references to clientset 1_3 (ccoleman@redhat.com)
- Remove generated v1_3 and v1_4 clients (ccoleman@redhat.com)
- generated: clients for release 1.5 (ccoleman@redhat.com)
- new-app: fix priority of Jenkinsfile, Dockerfile, source when strategy
  unspecified (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9686826940b7a6b7f996fefa5e80ed7ceb611fd1 (dmcphers+openshiftbot@redhat.com)
- prevent test/cmd/config.sh from writing config to working dir
  (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  68cce31e00c7fc8ea315ab0903fddecfd6fb0caf (dmcphers+openshiftbot@redhat.com)
- Migrated tests for login from hack/test-cmd.sh preamble (skuznets@redhat.com)
- Support HTTP URLs in oc start-build --from-file (mmilata@redhat.com)
- Support csproj files for identifying .NET Core projects (jminter@redhat.com)
- bump(github.com/openshift/origin-web-console):
  402dd2272fbd495b5e4cc54212671d605a51fbc7 (dmcphers+openshiftbot@redhat.com)
- create printMarkerSuggestions func (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6b5b66af4653f75de791c4a50107b63bc18992d0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  0824fe472aafc97455b7e12f617e1dff09721a4f (dmcphers+openshiftbot@redhat.com)
- Updated DIND scripts to use the new `os::util::find` methods
  (skuznets@redhat.com)
- Updated `hack/update-generated-docs.sh` to use new util functions
  (skuznets@redhat.com)
- Moved documentation generation functions from common.sh (skuznets@redhat.com)
- Refactored to use the `os::util::find` methods where necessary
  (skuznets@redhat.com)
- Declared the need for `sudo` and use it correctly in extended tests
  (skuznets@redhat.com)
- Refactored scripts to use new methods for $GOPATH binaries
  (skuznets@redhat.com)
- Refactored scripts to use `os::util::ensure::iptables_privileges_exist`
  (skuznets@redhat.com)
- Refactored to use `os::util::ensure::system_binary_exists`
  (skuznets@redhat.com)
- Refactored scripts to use binaries directly now that they're in PATH
  (skuznets@redhat.com)
- Updated scripts to use `os::util::ensure::built_binary_exists`
  (skuznets@redhat.com)
- Implemented Bash methods to search for binaries and ensure they exist
  (skuznets@redhat.com)
- Removed old methods used for finding binaries, etc (skuznets@redhat.com)
- Added PATH change to hack/lib/init.sh so all scripts have it
  (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7e9009da2339e5745fc7f6419edb0ed15d406b71 (dmcphers+openshiftbot@redhat.com)
- sdn: garbage-collect dead containers to recover IPAM leases (dcbw@redhat.com)
- sdn: clean up pod IPAM allocation even if we don't have a netns
  (dcbw@redhat.com)
- update missing probe severity to info (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  29241d57543c32458776e376175c9abc119e34c3 (dmcphers+openshiftbot@redhat.com)
- patch dcs and rcs instead of update (jvallejo@redhat.com)
- filter secret mount check if secret cannot be listed (jvallejo@redhat.com)
- Add security response info (sdodson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ca4be237df6935819e795eeb2962e985230876e1 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f43ec95079a848d38637f40f6784f0d58cb4f952 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  98bd4719ec7eccc390cacce9defbffc231d0ccf6 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5e199bb407bb409d6ad063a6d8075567e714bcb3 (dmcphers+openshiftbot@redhat.com)
- Adding extended tests for Jenkins freestyle exec (jupierce@redhat.com)
- bump(github.com/openshift/origin-web-console):
  589a660321721216d285beb350165df9b7a61655 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cf48ed9137425ab9fe49fede1726937a92b86ea5 (dmcphers+openshiftbot@redhat.com)
- bump(k8s.io/kubernetes): a9e9cf3b407c1d315686c452bdb918c719c3ea6e
  (agoldste@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c21f7f6a483dc1396e0dc2d578253194bbdfa4a3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  faa5ed6218cd0062692139901b1e88594c1273e9 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  26e4d1f374d4f03d524212dcff8b4d3f6aa43551 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  28c791a33969bac880c3680767d8057d364956fc (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9f649e9c36bece2f9b73b49a98b208c3f94c4426 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  f839c85b5306ff210d6d7bd6e8c344b4a0bfd146 (dmcphers+openshiftbot@redhat.com)
- Fix issues with Godeps.json (agoldste@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9b21516783e1ca59e72d3b2cad66fdf915fa91b8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b446eb8f12af505b2c52fbad5925ca9e2c8c7d66 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ddad4dd4e800dcfbf80178af2dc9129e48dabdd4 (dmcphers+openshiftbot@redhat.com)
- straight move (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e94bc6c50d52fa4e697a0b63d3c2539b38197ce0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d0a5912a82684aaa78d6759186444f3305582694 (dmcphers+openshiftbot@redhat.com)
- mark nodejs 0.10 image as deprecated (bparees@redhat.com)
- Fix bad format in README.md (zhao.xiangpeng@zte.com.cn)
- Memory monitoring for jenkins during extended tests (jupierce@redhat.com)
- Add OVS tests to exclusion list (agoldste@redhat.com)
- Expand scope of skipping kube federation tests (agoldste@redhat.com)
- Disable kube garbage collector tests (agoldste@redhat.com)
- Allow deployment controller to update pods (agoldste@redhat.com)
- Added a check to prevent pulling scratch (rymurphy@redhat.com)
- Add support for github teams (jliggitt@redhat.com)
- Allow all users to exec protoc in release images (ccoleman@redhat.com)
- Fixes bug 1395545 [link](https://bugzilla.redhat.com/show_bug.cgi?id=1395545)
  empty the pool, or delete the pool when endpoints are deleted
  (rchopra@redhat.com)
- UPSTREAM: 36779: fix leaking memory backed volumes of terminated pods
  (sjenning@redhat.com)
- Fix the command to restart docker on 1.3->1.4 upgrade (danw@redhat.com)
- Reap OAuthClientAuthorizations (jliggitt@redhat.com)
- Remove camel-casing on oauth client API calls (jliggitt@redhat.com)
- Reconfigured test-cmd to continue on failure (skuznets@redhat.com)
- Validate if the openshift master is running with mutitenant network plugin
  (zhao.xiangpeng@zte.com.cn)
- bump(*):remove unused _test.go files (ipalade@redhat.com)
- New-app/new-build support for pipeline buildconfigs (jminter@redhat.com)
- newapp: do not fail when file exists with same name as image
  (mmilata@redhat.com)
- Added error check for naming collisions with build (rymurphy@redhat.com)
- Fixed the SYN eater tests so they have enough privilege (bbennett@redhat.com)
- jenkins master/slave cleanup; fix job xml now that ict is auto==false
  (gmontero@redhat.com)
- Allow to unlink deleted secrets from a service account (mfojtik@redhat.com)
- Add router option to bind ports only when ready (marun@redhat.com)
- UPSTREAM: docker/distribution: 2008: Honor X-Forwarded-Port and Forwarded
  headers (miminar@redhat.com)
- diagnostics: add shell prompt to commands in msgs (lmeyer@redhat.com)
- diagnostics: make cluster role warning info, modify text (lmeyer@redhat.com)
- diagnostics: clarify when logging not configured (lmeyer@redhat.com)
- Added a mention of TEST_ONLY to extended test doc (rymurphy@redhat.com)
- pkg/quota/controller: Fix typo in file name (rhcarvalho@gmail.com)
- test: retry rollout latest on update conflicts (mkargaki@redhat.com)
- default resources for the build (haowang@redhat.com)
- Squash to a single commit (zhao.sijun@zte.com.cn)
- Drop the merge commits and update this (zhao.sijun@zte.com.cn)
- The func 'RequestForConfig' isn't used in the project
  (miao.yanqiang@zte.com.cn)
- Add a missing import to origin custom tito tagger. (dgoodwin@redhat.com)

* Mon Nov 28 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.30
- Update ose_images.sh - 2016-11-23 (tdawson@redhat.com)
- UPSTREAM: 36444: Read all resources for finalization and gc, not just
  preferred (maszulik@redhat.com)

* Wed Nov 23 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.29
- Merge remote-tracking branch upstream/master, bump origin-web-console 35b920b
  (tdawson@redhat.com)
- sdn: garbage-collect dead containers to recover IPAM leases (dcbw@redhat.com)
- sdn: clean up pod IPAM allocation even if we don't have a netns
  (dcbw@redhat.com)
- Update ose_images.sh - 2016-11-18 (tdawson@redhat.com)

* Fri Nov 18 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.28
- bump(k8s.io/kubernetes): a9e9cf3b407c1d315686c452bdb918c719c3ea6e
  (agoldste@redhat.com)
- Fix issues with Godeps.json (agoldste@redhat.com)
- Cherry-pick of https://github.com/openshift/origin/pull/11940 by @rajatchopra
  (commit a631fd33a4b6d079cd76951ea5fa082f3960c90c) (bbennett@redhat.com)
- Fix the command to restart docker on 1.3->1.4 upgrade (danw@redhat.com)
- Update ose_images.sh - 2016-11-16 (tdawson@redhat.com)
- Add OVS tests to exclusion list (agoldste@redhat.com)
- Expand scope of skipping kube federation tests (agoldste@redhat.com)
- Disable kube garbage collector tests (agoldste@redhat.com)
- Allow deployment controller to update pods (agoldste@redhat.com)

* Wed Nov 16 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.27
- oc convert tests (ffranz@redhat.com)
- Fix SDN startup ordering (again) (danw@redhat.com)
- UPSTREAM: 36603: fixes handling lists in convert (ffranz@redhat.com)
- Do not use whoami inside of start scripts (ccoleman@redhat.com)
- We should not be able to create a route when the host is not specified and
  wildcardpolicy is enabled. Fixes bug 1392862 -
  https://bugzilla.redhat.com/show_bug.cgi?id=1392862 Rework as per @liggitt
  review comments. Fix govet issue. (smitram@gmail.com)
- Accept docker0 traffic regardless of firewall (danw@redhat.com)
- updating (cdaley@redhat.com)
- Force image in test-end-to-end-docker.sh for ose (tdawson@redhat.com)
- Warn on login when user cannot request projects (jvallejo@redhat.com)
- Rescue panic (jliggitt@redhat.com)
- Update ose_images.sh - 2016-11-14 (tdawson@redhat.com)
- Wait for remote API response before return in UploadFileToContainer
  (jminter@redhat.com)
- f5 poolname fix (rchopra@redhat.com)
- Increase retained CI log size to 100M (agoldste@redhat.com)
- Fix SRV record lookup for ExternalName service (yhlou@travelsky.com)

* Mon Nov 14 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.26
- bump(github.com/openshift/origin-web-console):
  3d8c136b9089d687db197eb6c80daf243076c06a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  44484b3fa93991a6b9d5f4121eda94600ef55628 (dmcphers+openshiftbot@redhat.com)
- Update the TTY detection logic in Bash text utils (skuznets@redhat.com)
- oc get templates: print only first description line (mmilata@redhat.com)

* Fri Nov 11 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.25
- bump(github.com/openshift/origin-web-console):
  d5d4a253ad5c6e1be4bb06ccdcb255feb7b41bb2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cc2bd92ae1cb470a5d09121b0bbf92f5e52fb619 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a8c369b3f0a87ca757f179d13ae7e48cce0f9c86 (dmcphers+openshiftbot@redhat.com)
- List events for unidling controller (jliggitt@redhat.com)
- Change default to etcd2 (jliggitt@redhat.com)
- bump(github.com/openshift/origin-web-console):
  bb443db5dae2de44042f8cafb0c0b01000f1d1d1 (dmcphers+openshiftbot@redhat.com)
- set automatic to false for the dev DC deployment (bparees@redhat.com)
- Move test case TestGenerateGateway to common_test.go
  (zhao.xiangpeng@zte.com.cn)
- add general build trigger msg and add some other optimizition
  (li.guangxu@zte.com.cn)
- Fix for bugz https://bugzilla.redhat.com/show_bug.cgi?id=1392862 - don't
  allow wildcard policy if host is not specified. (smitram@gmail.com)
- Fix for bugz 1393262 - serve certs for wildcard hosts. (smitram@gmail.com)
- Add missing permission when running 'oc cluster up --metrics'
  (mwringe@redhat.com)
- add test/cmd test (jvallejo@redhat.com)
- UPSTREAM: 36541: Remove duplicate describer errs (jvallejo@redhat.com)
- Fix for bugz 1391878 and 1391338 - for multiple wildcard routes asking for
  the same subdomain and in the same namespace, reject newer routes with
  'HostAlreadyClaimed' if paths are different + address @liggitt review
  comments. (smitram@gmail.com)
- fixup man and test for default seccomp directory (sjenning@redhat.com)
- UPSTREAM: 36375: Fix default Seccomp profile directory (sjenning@redhat.com)
- Fix launching number of pods for testing network diagnostics
  (rpenta@redhat.com)
- Fix ordering of SDN startup threads (danw@redhat.com)
- deployments: stop using --follow to verify the env var in test
  (mfojtik@redhat.com)
- Migrated router and registry startup utilities from util.sh
  (skuznets@redhat.com)
- resolve logic errors introduced in #11077 and return 40x errors instead of
  500 errors where possible (jminter@redhat.com)
- deployments: oc rollout latest should not rollout on paused config
  (mfojtik@redhat.com)
- Allow to use star as repository name in imagestreamimport
  (agladkov@redhat.com)
- UPSTREAM: 36386: Avoid setting S_ISGID on files in volumes
  (sjenning@redhat.com)
- Migrated misc method and removed unused method from util.sh
  (skuznets@redhat.com)

* Wed Nov 09 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.24
- require builder image for source-type binary build creation
  (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  abcecfac2a615383455af9441b65aeca2961c7b5 (dmcphers+openshiftbot@redhat.com)
-  fix readme link for extended tests (ipalade@redhat.com)
- Fix for bugz #1389165 - extended route validation breaks included templates.
  Plus fixes as per @liggitt review comments:   o Clean up errors to not leak
  cert/key data.   o Relax checks on certs which have expired or valid in the
  future for     backward compatibility.   o Add tests for expired, future
  valid and valid certs with intermediate     CAs and pass intermediate chains
  to the x509 verifier.   o Improve readability of test config (certs, keys
  etc).   o Fixup error messages to include underlying certificate parse
  errors.   o Add comment and remove currenttime hack. (smitram@gmail.com)
- allow special hostsubnets to force a vnid on egress packets
  (rchopra@redhat.com)
- Bug 1388026 - Fix network diagnostics cleanup when test setup fails
  (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2684d16e5b68700f70c4fb3c8ee21e2cc34e3a0d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e93ae824f55ea789a60b0b393affb44d309e187e (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  582f844d34935d922cf6af60b5bafccbe32e305b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  61b1b3f78dc744ff2d5c8e303a5071cb3073a8ff (dmcphers+openshiftbot@redhat.com)
- clarify start-build --from-webhook only meant to work with generic webhook
  (jminter@redhat.com)
- bump(github.com/docker/docker/cliconfig):b9f10c951893f9a00865890a5232e85d770c
  1087 (ipalade@redhat.com)
- bump(github.com/openshift/source-to-image):
  5741384e376d9c61b5b36156003ede6698ca563b (ipalade@redhat.com)
- Fix bugz 1392395 - make regexp match more precise with ^$.
  (smitram@gmail.com)
- Update image-stream warning notices (andrew@andrewklau.com)
- bump(github.com/openshift/origin-web-console):
  35eab4849c4243041a53a5c5803bc30d6b96d4f8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  26ed3049f6169971ab109627d0f56e48e0a347ea (dmcphers+openshiftbot@redhat.com)
- sdn: update for bumped CNI (dcbw@redhat.com)
- bump(github.com/containernetworking/cni):
  52e4358cbd540cc31f72ea5e0bd4762c98011b84 (dcbw@redhat.com)
- sdn/eventqueue: handle DeletedFinalStateUnknown delta objects
  (dcbw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c6dbf16e5584671d4d8c84343e890c5f3a9327cf (dmcphers+openshiftbot@redhat.com)
- Warn about the right thing on double-pod-teardown (danw@redhat.com)
- Update ose_images.sh - 2016-11-07 (tdawson@redhat.com)
- Migrate scripts to use `os::log::info` (skuznets@redhat.com)
- Migrated SELinux utility functions to the networking suite
  (skuznets@redhat.com)
- fix --host-config-dir, don't download host path from docker container
  (jminter@redhat.com)
- UPSTREAM: 33014: Kubelet: Use RepoDigest for ImageID when available
  (sross@redhat.com)
- UPSTREAM: 33014: Add method to inspect Docker images by ID (sross@redhat.com)
- UPSTREAM: 36248: Fix possible race in operationNotSupportedCache
  (agoldste@redhat.com)
- UPSTREAM: 36249: fix version detection in openstack lbaas
  (sjenning@redhat.com)
- fix extended argument parsing to config building for etcd (deads@redhat.com)
- Remove uninterpreted newline literal from logging functions
  (skuznets@redhat.com)
- update sourceURI used in BuildConfig (li.guangxu@zte.com.cn)
- Fix ip range check condition (zhao.xiangpeng@zte.com.cn)
- UPSTREAM: 34831: cloudprovider/gce: canonicalize instance name when returning
  instance array (dcbw@redhat.com)
- Tolerate being unable to access OpenShift resources in status
  (ccoleman@redhat.com)

* Mon Nov 07 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.23
- bump(github.com/openshift/origin-web-console):
  22cbe94128168282f06b4e52766f05170a77df92 (dmcphers+openshiftbot@redhat.com)
- Deprecated evacuate in favor of drain node (mfojtik@redhat.com)
- Handle services of type ExternalName by returning a CNAME
  (ccoleman@redhat.com)
- fixes extended test issues caused by #11411 (cdaley@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6304150c9696dc4d6cb02e67d1ac42ec39ba4ee8 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6759555895af3481dbda7b92009015e0cdca3b35 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  42d3d3b85544a85b9c9d6fe7e60c09f1ae95173d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a6e3bf3a68da0b710b97d6312752faf5e7267fe2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  62f53fe551ded617e07b7eb9059b7b587e2f36a5 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2896e83370336132433ebb7aa99642dd922b84a0 (dmcphers+openshiftbot@redhat.com)
- avoid repetitive erros on adding fdb entries if it already exists
  (rchopra@redhat.com)
- bump(github.com/openshift/origin-web-console):
  06723a7cec861573be7a0b96c08bc55bdaecd7d4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4d5a78494e2618f1e5a0da0d7b61922497e1be10 (dmcphers+openshiftbot@redhat.com)
- Refactor uses of `wait_for_command` to use `os::cmd::try_until*`
  (skuznets@redhat.com)
- Migrate scripts to use `os::log::warn` (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  13e9d02b809914e507721d7ea62ec3ce6b9bce48 (dmcphers+openshiftbot@redhat.com)
- Migrate utilities specific to test/cmd/secrets.sh into the test script
  (skuznets@redhat.com)
- Replace old build utility functions with `os::cmd::try_until_text`
  (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  53904fdf20c0b8e188ba56be1a2639952066d20b (dmcphers+openshiftbot@redhat.com)
- Update ose_images.sh - 2016-11-04 (tdawson@redhat.com)
- Allow getting images from registries over untrusted connection
  (agladkov@redhat.com)
- oc start-build --from-repo now uses clone+checkout (rymurphy@redhat.com)
- Added a test for doTestConnection functionality (rymurphy@redhat.com)
- remove set resources from the list of exceptions to the --local flag
  (jtanenba@redhat.com)
- UPSTREAM: 36174: Implemented both the dry run and local flags.
  (jtanenba@redhat.com)
- UPSTREAM: 36174: fixed some issues with kubectl set resources
  (jtanenba@redhat.com)
- UPSTREAM: 36161: Fix how we iterate over active jobs when removing them for
  Replace policy (maszulik@redhat.com)
- integration: rewrite etcd dumper to use new client (mfojtik@redhat.com)
- remove unused go-etcd helper (mfojtik@redhat.com)
- bump(coreos/go-etcd): remove (mfojtik@redhat.com)
- Auto generated docs for NetworkDiagDefaultLogDir description change
  (rpenta@redhat.com)
- Bug 1388025 - Update NetworkDiagDefaultLogDir description (rpenta@redhat.com)
- Correcting log follow user guidance in new-build (jupierce@redhat.com)
- Fix an error variable reuse (miao.yanqiang@zte.com.cn)
- Clarify how we handle compression as a router env (bbennett@redhat.com)
- sdn: don't error deleting QoS if kubelet tears down networking a second time
  (dcbw@redhat.com)

* Fri Nov 04 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.22
- Restore use of oc ex dockerbuild (ccoleman@redhat.com)
- UPSTREAM: openshift/imagebuilder: <drop>: Handle Docker 1.12
  (ccoleman@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6da9b9c706b9bf3b00774835ee8cc4e05dd7ef65 (dmcphers+openshiftbot@redhat.com)
- Fix bugz 1391382 - allow http for edge teminated routes with wildcard policy.
  (smitram@gmail.com)
- bump(github.com/openshift/origin-web-console):
  8aa417ed6584042f46a519be8f688d69cdc45a33 (dmcphers+openshiftbot@redhat.com)
- Placed RPM release in a distinct directory and generated a repository
  (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7a4bd4638a7220d8479f892138ce1973f6803721 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  5f9ad0d846412cf8ae0d51e354248aa776b87f14 (dmcphers+openshiftbot@redhat.com)
- sdn: remove need for locking in pod tests (dcbw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  75f6fe4de4ef6aedbce70280083c79b85b4bd765 (dmcphers+openshiftbot@redhat.com)
- MongoDB replica petset test: increase time of waiting for pods.
  (vsemushi@redhat.com)
- update default subcommand run func (jvallejo@redhat.com)
- UPSTREAM: 35206: update default subcommand run func (jvallejo@redhat.com)
- Test editing lists in oc (mkargaki@redhat.com)
- UPSTREAM: 36148: make edit work with lists (mkargaki@redhat.com)
- Align with other cli descriptions (yu.peng36@zte.com.cn)
- UPSTREAM: 34763: log info on invalid --output-version (jvallejo@redhat.com)
- CleanupHostPathVolumes(): fetch meta info before removing PV.
  (vsemushi@redhat.com)
- Extended deployment timeout (maszulik@redhat.com)
- Cleaned up Bash logging functions (skuznets@redhat.com)
- UPSTREAM: 35285: Remove stale volumes if endpoint/svc creation fails
  (hchiramm@redhat.com)
- Bug 1388026 - Ensure deletion of namespaces created by network diagnostics
  command (rpenta@redhat.com)
- Add new unit tests to cover valid cases where output length is different from
  input length. (avagarwa@redhat.com)
- Fix wrapped word writer for cases where length of output bytes is not equal
  to length of input bytes. (avagarwa@redhat.com)
- Fix to restore os.Stdout. (avagarwa@redhat.com)
- UPSTREAM: 35978: allow PATCH in an API CORS setup (ffranz@redhat.com)
- Add openshift-excluder to origin (tdawson@redhat.com)
- UPSTREAM: 35608: Update PodAntiAffinity to ignore calls to subresources
  (maszulik@redhat.com)
- UPSTREAM: 35675: Require PV provisioner secrets to match type
  (jsafrane@redhat.com)
- Remove redundant constant definition (danw@redhat.com)
- Add permissions to get secrets to pv-controller. (jsafrane@redhat.com)

* Thu Nov 03 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.21
- Bump the version

* Thu Nov 03 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.20
- minor cleanup (rpenta@redhat.com)
- Bug 1390173 - Test more pod to pod connectivity test combinations
  (rpenta@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a51c2d0576902147ba30ad93e4498233d04a36a2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  668739b535bcd16bca6ba7b181939c8f25be6872 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  989f3952453cae0bd25d2f2e2f7337c16fdf7459 (dmcphers+openshiftbot@redhat.com)
- f5 node watch fix - needs a cache to process 'MODIFY' events
  (rchopra@redhat.com)
- bump(github.com/openshift/origin-web-console):
  109b88081b48b1f5f70b7386ff3a8a40e5129ce7 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7862d494be717042245e49b8047b12cb8b5839f0 (dmcphers+openshiftbot@redhat.com)
- Update router debugging instructions (jawnsy@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d7146b50996a5d5e95220613a884fa30a1fb4f65 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  42f3dc01d752b49dfa02389e8f231ee7b6220304 (dmcphers+openshiftbot@redhat.com)
- tests: wait_for_registry: use oc rollout status (mmilata@redhat.com)
- bump(github.com/openshift/origin-web-console):
  00eb12fca5c8009191fecbb4093b969c35886142 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d4bfaeec71e263087e817d61b274f94d0227fd45 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6ab1616d931e6ea0d9fbb0239f728139d3a0ec86 (dmcphers+openshiftbot@redhat.com)
- Allow pv controller to recycle pvs, watch recycler pod events
  (mawong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  71202680a49368e6a72bdfe52f5cb558cb4d60f5 (dmcphers+openshiftbot@redhat.com)
- bump build pod admission test timeout (bparees@redhat.com)
- Update ose_images.sh - 2016-11-02 (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  a0c089fff75cfee4a223497d9114eee2f35db6c3 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  40f7e508b443b64307c068335c5a55faaaa32d5c (dmcphers+openshiftbot@redhat.com)
- Fix creation of macvlan interfaces (danw@redhat.com)
- Update Swagger documentation generator script name (skuznets@redhat.com)
- test: wait for frontend rollout before getting app logs (mkargaki@redhat.com)
- deploy: default maxSurge/maxUnavailable when one is set to zero
  (mkargaki@redhat.com)
- fix error - cannot list all services in the cluster (rchopra@redhat.com)
- dind: make /run a shared mount (dcbw@redhat.com)
- Drop unused add_macvlan code from openshift-sdn-ovs (danw@redhat.com)
- Merging jenkins plugin test templates into one (jupierce@redhat.com)
- Use existing kube method to fetch the container ID (rpenta@redhat.com)
- Bug 1389213 - Fix join/isolate project network (rpenta@redhat.com)
- provide vxlan integration options to the router cmd line (rchopra@redhat.com)
- sdn: fix network-already-set-up detection (dcbw@redhat.com)
- sdn: run pod manager server forever (dcbw@redhat.com)
- oc: update short desc for rollout (mkargaki@redhat.com)
- fix bz1389267 - release subnet leases upon hostsubnet delete
  (rchopra@redhat.com)
- sdn: wait for kubelet network plugin init before processing pods
  (dcbw@redhat.com)
- sdn: start pod manager before trying to update if VNIDs have changed
  (dcbw@redhat.com)
- Fix a bug in the egress router README (danw@redhat.com)
- Check port range for IP Failover configuration (zhao.xiangpeng@zte.com.cn)

* Wed Nov 02 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.19
- Merge remote-tracking branch upstream/master, bump origin-web-console b476d31
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  3d0d7c23d3eb7a858daad9f6995e71326ce2be4b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c971f7568a31038543a4ec3f26188ca563e6e714 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d08915b806cd94bda01941e9589d2f98f39adc3d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2478dab5f3396516a008761df66d71663d80c81d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d62161e496bf902a3c8c30d7b79a7bbb0605df0c (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: 35082: Wait for all pods to be running before checking PDB status
  (maszulik@redhat.com)
- bump(github.com/openshift/origin-web-console):
  7928c08da8959bd851013a9b61c8569c9c7e85a7 (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: <drop>: add Timeout to client-go Config (agoldste@redhat.com)
- Removed `--force` flag from `docker tag` invocations (skuznets@redhat.com)
- Fix the failing serialization test - need a custom fuzzer to default the
  wildcard policy. (smitram@gmail.com)
- Fixes oc help global options hint (ffranz@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ed5d1ccb56350d4e3c72c27f7923d778907a44d5 (dmcphers+openshiftbot@redhat.com)
- Refactor scripts to use `oc get --raw` when possible (skuznets@redhat.com)
- bump(github.com/openshift/origin-web-console):
  4e66d443ec2918e6cba6256c4bfea1c0a8397b68 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  45e492f2f140fa3742895d10d0b379440cf2d06d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2701326256a43fda48739b211b1e88df570a8f88 (dmcphers+openshiftbot@redhat.com)
- Use custom transport for gitlab communication (jliggitt@redhat.com)
- fix oc whoami --show-server output (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  9c9f464e58da31df472fab9edef1502dfb6e2b2a (dmcphers+openshiftbot@redhat.com)
- Fixes bug 1380462 - https://bugzilla.redhat.com/show_bug.cgi?id=1380462
  (cdaley@redhat.com)
- update extended tests to work with non-empty output (jvallejo@redhat.com)
- update e2e test to work with non-emtpy output (jvallejo@redhat.com)
- UPSTREAM: 32722: warn on empty oc get output (jvallejo@redhat.com)
- UPSTREAM: 34434: Print valid json/yaml output (jvallejo@redhat.com)
- print valid json/yaml output (jvallejo@redhat.com)
- bump(github.com/openshift/origin-web-console):
  15caab12bc98d8be9decbcdf41fbbdd572118ff5 (dmcphers+openshiftbot@redhat.com)
- new-app: validate that Dockerfile from the repository has numeric EXPOSE
  directive (when strategy wasn't specified). (vsemushi@redhat.com)
- bump(github.com/openshift/origin-web-console):
  904eaca0a5e0ed5e1562a172ce064d125dec8e31 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  41d96f74bd672d4a4a67ff75796b1819c5d7f6d0 (dmcphers+openshiftbot@redhat.com)
- Generated clientset (maszulik@redhat.com)
- bump(k8s.io/client-go): d72c0e162789e1bbb33c33cfa26858a1375efe01
  (maszulik@redhat.com)
- Add cloud.google.com to supported hosts (maszulik@redhat.com)
- UPSTREAM: <drop>: Continue to carry the cAdvisor patch - revert
  (maszulik@redhat.com)
- UPSTREAM: 33806: Update cAdvisor godeps for v1.4.1 (maszulik@redhat.com)
- bump(github.com/google/cadvisor): ef63d70156d509efbbacfc3e86ed120228fab914
  (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image):
  2dffea37104471547b865307415876cba2bdf1fc - revert (maszulik@redhat.com)
- UPSTREAM: 34368: Node status updater should SetNodeStatusUpdateNeeded if it
  fails to (maszulik@redhat.com)
- UPSTREAM: 35273: Fixed mutation warning in Attach/Detach controller
  (maszulik@redhat.com)
- UPSTREAM: 35071: Change merge key for VolumeMount to mountPath
  (maszulik@redhat.com)
- UPSTREAM: 34955: HPA: fixed wrong count for target replicas calculations
  (maszulik@redhat.com)
- UPSTREAM: 34895: Fix non-starting node controller in 1.4 branch
  (maszulik@redhat.com)
- UPSTREAM: 34851: Only wait for cache syncs once in NodeController
  (maszulik@redhat.com)
- UPSTREAM: 34251: Fix nil pointer issue when getting metrics from volume
  mounter (maszulik@redhat.com)
- UPSTREAM: 34809: NodeController waits for informer sync before doing anything
  (maszulik@redhat.com)
- UPSTREAM: 34694: Handle DeletedFinalStateUnknown in NodeController
  (maszulik@redhat.com)
- UPSTREAM: 34076: Remove headers that are unnecessary for proxy target
  (maszulik@redhat.com)
- UPSTREAM: 33968: scheduler: initialize podsWithAffinity (maszulik@redhat.com)
- UPSTREAM: 33735: Fixes in HPA: consider only running pods; proper denominator
  in avg (maszulik@redhat.com)
- UPSTREAM: 33796: Fix issue in updating device path when volume is attached
  multiple times (maszulik@redhat.com)
- UPSTREAM: 32914: Limit the number of names per image reported in the node
  status (maszulik@redhat.com)
- UPSTREAM: 32807: Fix race condition in setting node statusUpdateNeeded flag
  (maszulik@redhat.com)
- UPSTREAM: 33346: disallow user to update loadbalancerSourceRanges
  (maszulik@redhat.com)
- UPSTREAM: 33170: Remove closing audit log file and add error check when
  writing to audit (maszulik@redhat.com)
- UPSTREAM: 33086: Fix possible panic in PodAffinityChecker
  (maszulik@redhat.com)
- bump(github.com/google/cadvisor): 0cdf4912793fac9990de3790c273342ec31817fb
  (maszulik@redhat.com)
- Fix oauth redirect ref in jenkins service account (cewong@redhat.com)
- Fixing kubernetes_plugin extended test failure
  (root@dhcp137-43.rdu.redhat.com)
- UPSTREAM: 33014: Kubelet: Use RepoDigest for ImageID when available
  (sross@redhat.com)
- Fix EgressNetworkPolicy match-all-IPs special case (danw@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d3402a50e89c5d476f9a41f6e4ac964eaeceddd0 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  05de64a8417d48ef33ba4474c15aa271010895ab (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1029c81d8cfd04534683ebd87b28ca3bad99b339 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d4becd8aedc3f39efdb1272a1cd66ff1212c9e7c (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  6631cc234d01243f12a594b949dad5b60a0211fd (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cd29ae2a7530f67924f5ea6ee26ff8afc9394602 (dmcphers+openshiftbot@redhat.com)
- provide the ability to modify the prefix of error reporting and ignore some
  errors in cmd operations (pweil@redhat.com)
- Jenkins imagestream scm extended tests (cewong@redhat.com)
- bump(github.com/openshift/origin-web-console):
  cb2212c6008c41196a78c43a4b3dd1de4ba8248f (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  b0ce978d52143e0f3d9d4ecc36f40a905d4f31f4 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  78a74f7768b05f2a73508e0d2e9a9b33bbecb7b3 (dmcphers+openshiftbot@redhat.com)
- deploy: correct updating lastTransitionTime in deployment conditions
  (mkargaki@redhat.com)
- bump(github.com/openshift/origin-web-console):
  693c33e8a5724f6b3d7d659411f3eeeea0aad78a (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e5b9d0fbfd2b3cceb0f9674d2361af0960861bb2 (dmcphers+openshiftbot@redhat.com)
- Update ose_images.sh - 2016-10-31 (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d1d6113a8d3dc6f2654d02d33332c110009ff4d0 (dmcphers+openshiftbot@redhat.com)
- Initialize cloud provider in node (jsafrane@redhat.com)
- deploy: drop paused conditions (mkargaki@redhat.com)
- Update rules for upstream deployment rollbacks (mkargaki@redhat.com)
- cli: remove support for comma-separated template values (mmilata@redhat.com)
- Fix as per @liggitt review comments - write wildcard spec as is in the
  rejection. (smitram@gmail.com)
- Fixes as per @liggitt review comments:   o Fix generated route protobuf file
  o Default Route.Spec.WildcardPolicy to None   o Add WildcardPolicy to
  RouteIngress in RouteStatus (defaulting to None)   o Record admitted wildcard
  policy when recording route status   o Reject wildcard routes when wildcards
  are disabled   o Change admission functions to just return errors (no bool)
  and update     checks for policy to use a switch in lieu of a if statement
  o Simpilify RouteLessThan api helper function   o Make
  Route.Spec.WildcardPolicy immutable (in update validation)   o Log details
  about conflicting routes, but don't leak info to the user   o Change error to
  "HostAlreadyClaimed"   o Add checks for defaults on Spec and Ingress
  WildcardPolicy in conversion tests. (smitram@gmail.com)
- Test run with --dry-run and attachable containers (ffranz@redhat.com)
- UPSTREAM: 35732: validate run with --dry-run and attachable containers
  (ffranz@redhat.com)
- UPSTREAM: 34298: Fix potential panic in namespace controller
  (decarr@redhat.com)
- to fix service account informer to podsecuritypolicyreview (salvatore-
  dario.minonne@amadeus.com)
- bump(github.com/spf13/pflag): 5ccb023bc27df288a957c5e994cd44fd19619465
  (mmilata@redhat.com)
- Align with other cli descriptions (yu.peng36@zte.com.cn)
- UPSTREAM: 30836:  fix Dynamic provisioning for vSphere (gethemant@gmail.com)
- Change to use helper function and fix up helper function expectations and
  tests and add missing generated files. (smitram@gmail.com)
- Data structure rework (jliggitt@redhat.com)
-    o Split out cert list and use commit from PR 11217    o Allow wildcard
  (currently only *.) routes to be created and add tests.    o Add a host
  admission controller and allow/deny list of domains and control      the
  admission/blockage of wildcard routes.    o Fix test cases and expection.
  o Add helper to generate valid wildcard regular expressions.    o Add
  wildcard domain map + regex based rules and use the rules for wildcard
  routes.    o Bug fixes and add tests.    o Add generated completions and
  docs.    o Changes as per @marun, @rajatchopra, @smarterclayton review
  comments    o Rework as per api changes to use wildcard policy with routes
  o Add defaults and update generated files. (smitram@gmail.com)
- Fix an issue with route ordering: it was possible for a newer route to be
  reported as the older route based on name/namespace checks. Ensure that we
  have a stable ordering based on the age of a route. (smitram@gmail.com)
- UPSTREAM: 35420: Remove Job also from .status.active for Replace strategy
  (maszulik@redhat.com)
- Fix for bugz https://bugzilla.redhat.com/show_bug.cgi?id=1337322 Add
  Spec.Host validation for dns labels and subdomain at runtime - this is not
  enabled in the API validation checks on route creation. And fixes as per
  @smarterclayton review comments. (smitram@gmail.com)

* Mon Oct 31 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.18
- Merge remote-tracking branch upstream/master, bump origin-web-console 9f1003a
  (tdawson@redhat.com)
- bump(github.com/openshift/origin-web-console):
  e30aa32263beacd860469cc1bd825a16fe41060b (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  12e11b40cc788780f93bde167ce0a7ff04b02d2d (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d8dad85ae673fe5d7be2f5a393f2f67594ea0ad2 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  2ad1f38354a58596a048ebd8a369fd733b165126 (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  d618617a596a4cac65ca6781f7422aa138659d0e (dmcphers+openshiftbot@redhat.com)
- reorder sample pipeline parameters for priority/importance
  (bparees@redhat.com)
- bump(github.com/openshift/origin-web-console):
  c3e34937c3e516d785299571ec20aa1afb3472df (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/origin-web-console):
  1ec5a2a739138f31d5a3f2e3cf9401e07f8cc3e1 (dmcphers+openshiftbot@redhat.com)
- make service controller failure non-fatal (sjenning@redhat.com)
- bump(github.com/openshift/origin-web-console):
  ea5fc5ee0f064cb717bfc93e6adbafa1833a4e13 (dmcphers+openshiftbot@redhat.com)
- Give the release images a fake golang provides (ccoleman@redhat.com)
- Update cluster up documentation (cewong@redhat.com)
- Skip retrieving logs if journalctl is not available (maszulik@redhat.com)
- Revert "Getting docker logs always to debug issue 8399" (maszulik@redhat.com)
- Replace uses of `validate_reponse` with `os::cmd::try_until_text`
  (skuznets@redhat.com)
- UPSTREAM: 33024: Add "PrintErrorWithCauses" kcmdutil helper
  (jvallejo@redhat.com)
- Make status errors with  multiple causes cleaner (jvallejo@redhat.com)
- Update ose_images.sh - 2016-10-28 (tdawson@redhat.com)
- HACKING.md fix format (jay@apache.org)
- Adds display name to image streams, updates PostgreSQL link
  (jacoblucky@gmail.com)
- Add some details to cherry picking for newcomers (jay@apache.org)
- Expose test build option for RPM `make` targets (skuznets@redhat.com)
- Add godoc to justify use of developmentRedirectURI (jvallejo@redhat.com)

* Fri Oct 28 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.17
- Merge remote-tracking branch upstream/master, bump origin-web-console df20542
  (tdawson@redhat.com)
- Fix failing deployment hook fixture (ironcladlou@gmail.com)
- Move namespace lifecycle plugin to the front of the admission chain
  (jliggitt@redhat.com)
- Only pay attention to origin types in project lifecycle admission
  (jliggitt@redhat.com)
- bump(origin-web-console): de8aca535f2762214184cd6bbc28d9948aac3a14
  (noreply@redhat.com)
- Fix deep copy for api.ResourceQuotasStatusByNamespace (jliggitt@redhat.com)
- Fix mutation in OrderedKeys getter (jliggitt@redhat.com)
- fix nil printer panic in oc set sub-cmds (jvallejo@redhat.com)
- add jenkins v2 imagestreams (bparees@redhat.com)
- Added comment to clarify that oc cluster did not work on releases before that
  1.3 (jparrill@redhat.com)
- Adding warning about port 8443 potentially being blocked by firewall rules on
  oc cluster up Fixes issue #10807 (cdaley@redhat.com)
- Replace uses of `wait_for_url_timed` with `os::cmd::try_until_success`
  (skuznets@redhat.com)
- Remove `reset_tmp_dir` function from Bash library (skuznets@redhat.com)
- new-app: validate that Dockerfile from the repository has numeric EXPOSE
  directive. (vsemushi@redhat.com)
- deploy: set condition reason correctly for new RCs (mkargaki@redhat.com)
- Use global LRU cache for layer sizes (agladkov@redhat.com)
- UPSTREAM: 27714: Send recycle events from pod to pv. (jsafrane@redhat.com)
- remove redundant variable 'retryCount' (miao.yanqiang@zte.com.cn)
- ie. is not the correct abbreviation for ID EST (yu.peng36@zte.com.cn)
- Display warning instead of error if ports 80/443 in use (cdaley@redhat.com)
- bump(github.com/openshift/source-to-image):
  2dffea37104471547b865307415876cba2bdf1fc (ipalade@redhat.com)
- test/integration: pass HostConfig at image creation time for router tests
  (dcbw@redhat.com)
- sdn: no longer kill docker0 (dcbw@redhat.com)
- add `oc status` warning for missing liveness probe (jvallejo@redhat.com)
- sdn: fix single-tenant pod setup (dcbw@redhat.com)
- Update ose_images.sh - 2016-10-26 (tdawson@redhat.com)
- Remove usage of deprecated utilities for starting a server
  (skuznets@redhat.com)
- Use `oc get --raw` instead of `curl` in Swagger verification/update scripts
  (skuznets@redhat.com)
- include timestamps in extended test build logs (bparees@redhat.com)
- UPSTREAM: 34997: Fix kube vsphere.kerneltime (gethemant@gmail.com)
- Quota test case is inaccurate (ccoleman@redhat.com)
- fix broken sample pipeline job (bparees@redhat.com)
- Allow users to pass in `make_redistributable` to the Origin spec file
  (skuznets@redhat.com)
- Make extended test build optional in origin.spec (skuznets@redhat.com)
- Fix regression from #9481 about defaulting prune images arguments
  (maszulik@redhat.com)
- Remove node access from the system:router roles (ccoleman@redhat.com)

* Wed Oct 26 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.16
- Merge remote-tracking branch upstream/master, bump origin-web-console c0704ca
  (tdawson@redhat.com)
- Migrated miscellaneous Bash utilities into the `hack/lib` library
  (skuznets@redhat.com)
- removing unused default variables (cdaley@redhat.com)
- Removing ImageChangeControllerFatalError and it's associated references as it
  is no longer used. (cdaley@redhat.com)
- Switch back to using docker build with docker 1.12 (dmcphers@redhat.com)
- deployments: set ActiveDeadlineSeconds in deployer hook pods correctly
  (mfojtik@redhat.com)
- Correctly report the size of overlarge log files (skuznets@redhat.com)
- Updates template and image stream metadata (jacoblucky@gmail.com)
- really re-enable jenkins autoprovisioning (bparees@redhat.com)
- Start-build/env, exec, and multitag jenkins plugin tests
  (jupierce@redhat.com)
- oc: fix export for deployment configs (mkargaki@redhat.com)
- WIP Fix bug 1373330 Invalid formatted generic webhook can trigger new-build
  without warning (jminter@redhat.com)
- serviceaccount: add secret informer to create_dockercfg_secret
  (mfojtik@redhat.com)
- Improve exec/attach error message (jliggitt@redhat.com)
- Create storage-admin role (screeley@redhat.com)
- support non-string template parameter substitution (bparees@redhat.com)
- move jenkins related ext tests to image_ecosystem (gmontero@redhat.com)
- Update ose_images.sh - 2016-10-24 (tdawson@redhat.com)
- Remove `get_object_assert` utility from our Bash libraries
  (skuznets@redhat.com)
- specfile: fix specfile issues after openshift-sdn CNI plugin merge
  (dcbw@redhat.com)
- Removed the `tryuntil` utility from our Bash libraries (skuznets@redhat.com)
- Update man pages (ffranz@redhat.com)
- UPSTREAM: 35427: kubectl commands must not use the factory out of Run
  (ffranz@redhat.com)
- Fix OS_RELEASE=n for build-images.sh (ccoleman@redhat.com)
- generated: resources (ccoleman@redhat.com)
- UPSTREAM: <carry>: Revert extra pod resources change (ccoleman@redhat.com)
- Add additional utilities used during test-cmd (ccoleman@redhat.com)
- Disable test for unwritable config file (ccoleman@redhat.com)
- deploy: tweak enqueueing in the trigger controller (mkargaki@redhat.com)
- extended: use timestamps in deployer logs (mkargaki@redhat.com)
- add nodeselector and annotation build pod overrides and defaulters
  (bparees@redhat.com)
- deploy: cleanup dc controller (mkargaki@redhat.com)
- Created a script to run tito builds and upack leftover artifacts
  (skuznets@redhat.com)
- Tests: don't clone openshift/origin where possible (mmilata@redhat.com)
- bump(github.com/coreos/etcd):v3.1.0-rc.0 (ccoleman@redhat.com)
- bump(google.golang.org/grpc):v1.0.2 (ccoleman@redhat.com)
- Switch to using protobuf and etcd3 storage (ccoleman@redhat.com)
- Fixes bug 1380555 - https://bugzilla.redhat.com/show_bug.cgi?id=1380555 Uses
  registry.access.redhat.com/openshift3/ose as the default for ose builds
  (cdaley@redhat.com)
- Remove -a flag from `os::build::build_static_binaries` (mkhan@redhat.com)
- Remove -a flag from hack/test-integration.sh (mkhan@redhat.com)
- add integration that create project using 1.3 clientset (mfojtik@redhat.com)
- regenerate clientsets for v1_3 (mfojtik@redhat.com)
- make projects non-namespaced (mfojtik@redhat.com)
- improve the client set generator script (mfojtik@redhat.com)
- set the --clientset-api-path=/oapi for origin clients (mfojtik@redhat.com)
- UPSTREAM: 32769: clientgen: allow to pass custom apiPath when generating
  client sets (mfojtik@redhat.com)

* Mon Oct 24 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.15
- Merge remote-tracking branch upstream/master, bump origin-web-console 9a90917
  (tdawson@redhat.com)
- PodDisruptionBudget - generated changes (maszulik@redhat.com)
- Enable PodDistruptionbudget e2e tests in conformance (maszulik@redhat.com)
- UPSTREAM: 35287: Add resource printer and describer for PodDisruptionBudget
  (maszulik@redhat.com)
- UPSTREAM: 35274: Fix PDB e2e test, off-by-one (maszulik@redhat.com)
- Correcting wording of error message (cdaley@redhat.com)
- Added cccp.yml file to build container on CentOS Container Pipeline.
  (mohammed.zee1000@gmail.com)
- sdn: re-add missing NAT for pods that docker used to do (dcbw@redhat.com)
- sdn: convert pod network setup to a CNI plugin (dcbw@redhat.com)
- sdn: use CNI for IPAM instead of docker (dcbw@redhat.com)
- Drop pkg/sdn/plugin/api, add stuff to pkg/sdn/api (danw@redhat.com)
- bump(github.com/containernetworking/cni):
  b8e92ed030588120f9fda47dd359e17a3234142d (dcbw@redhat.com)
- Remove the func 'parseRepositoryTag' (miao.yanqiang@zte.com.cn)
- f5 vxlan integration with sdn (rchopra@redhat.com)
- test-cmd fails when the user is root (ccoleman@redhat.com)
- for now disable jenkins oauth for ext tests (gmontero@redhat.com)
- re-enable jenkins autoprovisioning (bparees@redhat.com)
- Use actual semantic versioning for Git tags (ccoleman@redhat.com)
- hack/env upgrades - use rsync for sync (ccoleman@redhat.com)
- UPSTREAM: 32593: Audit test fails to take into account timezone
  (ccoleman@redhat.com)
- client: fix instantiate call to handle 204 (mkargaki@redhat.com)
- doc and template updates for jenkins openshift oauth plugin
  (gmontero@redhat.com)
- Revert "Add router support for wildcard domains (*.foo.com)"
  (ccoleman@redhat.com)
- Bump origin-web-console (7c57218) (spadgett@redhat.com)
- Bump to tls1.2 (jliggitt@redhat.com)
- Add `bc` to the release Docker image specs for test-cmd (skuznets@redhat.com)
- Generated: docs/bash completions for network diagnostics (rpenta@redhat.com)
- Added network diagnostic validation test (rpenta@redhat.com)
- Expose network diagnostics via 'oadm diagnostics NetworkCheck'
  (rpenta@redhat.com)
- Make network diagnostics log directory configurable (rpenta@redhat.com)
- Added support for Network diagnostics (rpenta@redhat.com)
- Collect and consolidate remote network diagnostic logs (rpenta@redhat.com)
- Create test environment for network diagnostics (rpenta@redhat.com)
- Custom pod/service objects for network diagnostics (rpenta@redhat.com)
- Added support for openshift infra network-diagnostic-pod (rpenta@redhat.com)
- Diagnostics: Collect network debug logs on the node (rpenta@redhat.com)
- Helper methods to capture master/node logs (rpenta@redhat.com)
- Diagnostics: Added service connectivity checks (rpenta@redhat.com)
- Diagnostics: Added external connectivity checks (rpenta@redhat.com)
- Diagnostics: Added pod network checks (rpenta@redhat.com)
- Modify GetHostIPNetworks() to also return host IPs (rpenta@redhat.com)
- Diagnostics: Added node network checks (rpenta@redhat.com)
- Added diagnostics util functions (rpenta@redhat.com)
- adding oc set resources as a wrapper for an upstream commit
  (jtanenba@redhat.com)
- UPSTREAM: 27206: Add kubectl set resources (jtanenba@redhat.com)
- fix annotations test flake (jvallejo@redhat.com)
- Change pipeline sample to use Node.js+MongoDB sample and also "nodejs"
  Jenkins slave. (sspeiche@redhat.com)
- CleanupHostPathVolumes(): remove also directories from the filesystem.
  (vsemushi@redhat.com)
- Update ose_images.sh - 2016-10-21 (tdawson@redhat.com)
- Update shell completions with oc describe storageclass (jsafrane@redhat.com)
- atomic registry systemd install bugfixes (aweiteka@redhat.com)
- UPSTREAM: 34638: Storage class updates in oc output (jsafrane@redhat.com)
- Implement route based annotations for service accounts (mkhan@redhat.com)
- UPSTREAM: 31607: Add kubectl describe storageclass (jsafrane@redhat.com)
- Bug 1386018: use deployment conditions when creating a rc
  (mkargaki@redhat.com)
-    o Split out cert list and use commit from PR 11217    o Allow wildcard
  (currently only *.) routes to be created and add tests.    o Add a host
  admission controller and allow/deny list of domains and control      the
  admission/blockage of wildcard routes.    o Fix test cases and expection.
  o Add helper to generate valid wildcard regular expressions.    o Add
  wildcard domain map + regex based rules and use the rules for wildcard
  routes.    o Bug fixes and add tests.    o Add generated completions and
  docs.    o Changes as per @marun, @rajatchopra, @smarterclayton review
  comments (smitram@gmail.com)
- Fix an issue with route ordering: it was possible for a newer route to be
  reported as the older route based on name/namespace checks. Ensure that we
  have a stable ordering based on the age of a route. (smitram@gmail.com)
- Cleanup: Use wait.ExponentialBackoff instead of retry loop
  (rpenta@redhat.com)
- Continue project cache evaluation in the presence of evaluation errors
  (jliggitt@redhat.com)
- group podLists into single list (jvallejo@redhat.com)
- update generated docs (jvallejo@redhat.com)
- Update oc create success message when using --dry-run (jvallejo@redhat.com)
- UPSTREAM: 31276: Update oc create success message when using --dry-run
  (jvallejo@redhat.com)
- Update completions and man pages for volume cmd (gethemant@gmail.com)
- Add AuditConfig validation and backwards compatibility if no AuditFilePath is
  provided (maszulik@redhat.com)
- Support specifying StorageClass while creating volumes (gethemant@gmail.com)
- Check if hostBits equals zero and add some test cases
  (zhao.xiangpeng@zte.com.cn)
- update oc env, return resources in list (jvallejo@redhat.com)
- Switch to use upstream audit handler - generated changes
  (maszulik@redhat.com)
- Switch to use upstream audit handler (maszulik@redhat.com)
- UPSTREAM: 33934: Add asgroups to audit log (maszulik@redhat.com)
- dind: bump deployment timeout to 120s (marun@redhat.com)
- dind: wait-for-condition forever by default (marun@redhat.com)
- UPSTREAM: 30145: Add PVC storage to Limit Range (mturansk@redhat.com)

* Fri Oct 21 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.14
- Merge remote-tracking branch upstream/master, bump origin-web-console 5e10797
  (tdawson@redhat.com)
- Implement cluster status command (jminter@redhat.com)
- Enable Gluster and RBD provisioners (jsafrane@redhat.com)
- Add compression to haproxy v2 (cw-aleks@users.noreply.github.com)
- Add extended test for PetSet based MongoDB replica set. (vsemushi@redhat.com)
- Enable PV controller to provisiong Gluster volumes (jsafrane@redhat.com)
- UPSTREAM: 34705: Make use of PVC namespace when provisioning gluster volumes
  (jsafrane@redhat.com)
- UPSTREAM: 35141: remove pv annotation from rbd volume (jsafrane@redhat.com)
- Update role binding docs (mkhan@redhat.com)
- Do not use "*" as DockerImageReference.Name for ImageStreamImport
  (agladkov@redhat.com)
- oc new-app --search: don't require docker hub access (mmilata@redhat.com)
- UPSTREAM: 35022: Remove PV annotations for Gluster provisioner
  (hchen@redhat.com)
- add extened test for pipeline build (haowang@redhat.com)
- Work around broken run --attach for now (agoldste@redhat.com)
- Fixes issue #10108 Adds os::build::setup_env to hack/update-generated-
  bootstrap-bindata.sh which ensures that the GOPATH env variable is set and
  not empty (cdaley@redhat.com)
- Update ose_images.sh - 2016-10-19 (tdawson@redhat.com)
- Fixes bug 1345773 - https://bugzilla.redhat.com/show_bug.cgi?id=1345773
  (cdaley@redhat.com)
- change gitserver template strategy to Recrate
  (shiywang@dhcp-140-35.nay.redhat.com)
- Bug 1386054: enqueue in the trigger controller only when really needed
  (mkargaki@redhat.com)
- keepalived vip (vrrp) requires 224.0.0.18/32 (pcameron@redhat.com)
- update build status reasons to StatusReason type (jvallejo@redhat.com)
- IngressIP controller: Fix Service update in an exponential backoff manner
  (rpenta@redhat.com)
- update generated docs (jvallejo@redhat.com)
- bump(k8s.io/client-go): add Timeout field to 1.4 restclient
  (jvallejo@redhat.com)
- rename global flag to --request-timeout (jvallejo@redhat.com)
- UPSTREAM: 33958: update --request-timeout flag to string value
  (jvallejo@redhat.com)
- UPSTREAM: 33958: add global timeout flag (jvallejo@redhat.com)
- Update the cli hacking guide (ffranz@redhat.com)
- Help templates which allow reordering of sections (ffranz@redhat.com)
- Update generated docs (ffranz@redhat.com)
- Tools for checking CLI conventions (ffranz@redhat.com)
- Normalize CLI examples and long descriptions (ffranz@redhat.com)
- Markdown rendering engine (ffranz@redhat.com)
- Commands must always use the provided err output (ffranz@redhat.com)
- Add writers capable of adjusting to terminal sizes (ffranz@redhat.com)
- bump(github.com/mitchellh/go-wordwrap):
  ad45545899c7b13c020ea92b2072220eefad42b8 (ffranz@redhat.com)
- Minor fixups after s2i bump (jminter@redhat.com)
- bump(github.com/openshift/source-to-image):
  5009651d01b7f96b5373979317f4d4f32415f32b (jminter@redhat.com)
- React to new --revision flag in rollout status (mkargaki@redhat.com)
- enable and test owner reference protection (deads@redhat.com)
- UPSTREAM: 34443: kubectl: add --revision flag in rollout status
  (mkargaki@redhat.com)
- oc: add -o revision in rollout latest (mkargaki@redhat.com)
- Reject Builds with unresolved image references (mmilata@redhat.com)
- Allow running extended test without specifying namespace
  (maszulik@redhat.com)
- Fix validation messages for PodSecurityPolicy*Review objects
  (maszulik@redhat.com)
- Add a missing err return (miao.yanqiang@zte.com.cn)
- Fix an imperfect if statement (miao.yanqiang@zte.com.cn)
- To add Informer for ServiceAccount (salvatore-dario.minonne@amadeus.com)
- dind: deploy multitenant plugin by default (marun@redhat.com)
- dind: stop truncating systemd logs (marun@redhat.com)
- Bump origin-web-console (d5318f2) (jforrest@redhat.com)
- Cleaned up the system logging Bash library (skuznets@redhat.com)
- update docs (jvallejo@redhat.com)
- UPSTREAM: 32555: WantsAuthorizer admission plugin support (deads@redhat.com)
- use cmdutil DryRun flag helper (jvallejo@redhat.com)
- UPSTREAM: 34028: add --dry-run option to apply kube cmd (jvallejo@redhat.com)
- UPSTREAM: 34028: add --dry-run option to create root cmd
  (jvallejo@redhat.com)
- Add --dry-run option to create sub-commands (jvallejo@redhat.com)
- UPSTREAM: 34829: add ownerref permission checks (deads@redhat.com)
- Make OAuth provider discoverable from within a Pod (mkhan@redhat.com)
- remove ruby, mysql tags from pipeline sample template (bparees@redhat.com)
- UPSTREAM: 32662: Change the default volume type of GlusterFS provisioner
  (jsafrane@redhat.com)
- spell jenkins correctly (jminter@redhat.com)
- cluster up: add option to install logging components (cewong@redhat.com)
- Remove -f flag from docker tag (gethemant@gmail.com)
- Remove signature store from registry (agladkov@redhat.com)
- UPSTREAM: docker/distribution: 1857: Provide stat descriptor for Create
  method during cross-repo mount (jliggitt@redhat.com)
- UPSTREAM: docker/distribution: 1757: Export storage.CreateOptions in top-
  level package (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Update dependencies
  (agladkov@redhat.com)
- bump(github.com/docker/distribution):
  12acdf0a6c1e56d965ac6eb395d2bce687bf22fc (agladkov@redhat.com)
- Enable exec/http proxy e2e test (agoldste@redhat.com)
- refactor install to support configuration, plus help manpage
  (aweiteka@redhat.com)

* Wed Oct 19 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.13
- Update ose_images.sh - 2016-10-17 (tdawson@redhat.com)
- Remove dependence on external mysql image tag order (jliggitt@redhat.com)
- Set test/extended/networking-minimal.sh +x (marun@redhat.com)
- UPSTREAM: 32084: Do not allow creation of GCE PDs in unmanaged zones
  (jsafrane@redhat.com)
- UPSTREAM: 32077: Do not report warning event when an unknown provisioner is
  requested (jsafrane@redhat.com)
- Clarify new-app messages (jminter@redhat.com)
- allow review endpoints on missing namespaces (deads@redhat.com)
- UPSTREAM: <carry>: update namespace lifecycle to allow review APIs
  (deads@redhat.com)
- Correct secret type in oc secrets new-{basic,ssh}auth (jminter@redhat.com)
- add cmd test (jvallejo@redhat.com)
- oc: generated code for switching from --latest to --again
  (mkargaki@redhat.com)
- oc: deprecate 'deploy --latest' in favor of 'rollout latest --again'
  (mkargaki@redhat.com)
- UPSTREAM: 34010: Match GroupVersionKind against specific version
  (maszulik@redhat.com)
- deploy: generated code for api refactoring (mkargaki@redhat.com)
- deploy: api types refactoring (mkargaki@redhat.com)
- make function addDefaultEnvVar cleaner (li.guangxu@zte.com.cn)
- add client config invalid flags error handling test (jvallejo@redhat.com)
- UPSTREAM: 29236: handle invalid client config option errors
  (jvallejo@redhat.com)
- UPSTREAM: 34020: Allow empty annotation values Annotations with empty values
  can be used, for example, in diagnostics logging. This patch removes the
  client-side check for empty values in an annotation key-value pair.
  (jvallejo@redhat.com)
- Add logging to project request failures (jliggitt@redhat.com)
- Allow cluster up and rsync when no ipv6 on container host
  (jminter@redhat.com)
- UPSTREAM: <carry>: sysctls SCC strategy (agoldste@redhat.com)
- Fix a typo of logging code in pkg/router/template/router.go
  (yhlou@travelsky.com)
- Add wildcard entry to cluster up router cert (ironcladlou@gmail.com)
- Update cluster_up_down.md (chris-milsted@users.noreply.github.com)

* Mon Oct 17 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.12
- Merge remote-tracking branch upstream/master, bump origin-web-console d5318f2
  (tdawson@redhat.com)
- fix oc describe suggestion in oc rsh output (jvallejo@redhat.com)
- add pod bash completion `oc exec` (jvallejo@redhat.com)
- Enable minimization of net e2e runtime (marun@redhat.com)
- ParseDockerImageReference use docker/distribution reference parser
  (agladkov@redhat.com)
- Delete a redundant arg and optimize some codes in the
  (miao.yanqiang@zte.com.cn)
- remove the tmp secret data (li.guangxu@zte.com.cn)
- remove old atomic doc (pweil@redhat.com)
- move images extended tests to new tag and directory (bparees@redhat.com)
- dind: Ensure unique node config and fix perms (marun@redhat.com)
- dind: ensure node certs are generated serially (marun@redhat.com)
- integration: retry instantiate on conflict (mkargaki@redhat.com)
- CLI: add a set build-secret command (cewong@redhat.com)
- Bug 1383138: suggest 'rollout latest' if 'deploy --latest' returns a bad
  request (mkargaki@redhat.com)
- UPSTREAM: 34524: Test x509 intermediates correctly (jliggitt@redhat.com)
- Test x509 intermediates correctly (jliggitt@redhat.com)
- Fixing validation msg (jhadvig@redhat.com)
- Change haproxy router to use a certificate list/map file. (smitram@gmail.com)
- Note the commit for the release packaged by hack/build-cross.sh
  (skuznets@redhat.com)
- new-app: warn when source credentials are needed (cewong@redhat.com)

* Fri Oct 14 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.11
- Merge remote-tracking branch upstream/master, bump origin-web-console b7fa730
  (tdawson@redhat.com)
- increase default vagrant volume size to 25gigs (bparees@redhat.com)
- Update ose_images.sh - 2016-10-12 (tdawson@redhat.com)
- the parameter no need set again (li.guangxu@zte.com.cn)
- Add a `make` target to vendor the web console (skuznets@redhat.com)
- Make evacuate aware of replica set and daemon set (mfojtik@redhat.com)
- Rename Kclient to KubeClient (mfojtik@redhat.com)
- add some optimizations to build code (li.guangxu@zte.com.cn)
- get password from dc; add more diag (gmontero@redhat.com)
- oc: update valid arguments for upstream commands (mkargaki@redhat.com)
- UPSTREAM: <drop>: rollout status should return appropriate exit codes
  (mkargaki@redhat.com)
- Tests for deployment conditions and oc rollout status (mkargaki@redhat.com)
- Extend Builds with list of labels applied to image (mmilata@redhat.com)
- bump(github.com/openshift/source-to-image):
  2a7f4c43b68c0f7a69de1d59040544619b03a21a (mmilata@redhat.com)
- Move "oadm pod-network" tests from extended to integration and cmd
  (danw@redhat.com)
- test/cmd/sdn.sh improvements (danw@redhat.com)
- oc: generated code for rollout status (mkargaki@redhat.com)
- oc: add rollout status (mkargaki@redhat.com)
- deploy: use Conditions in the deploymentconfig controller
  (mkargaki@redhat.com)
- Remove the unuse parameter in function ActiveDeployment
  (wang.yuexiao@zte.com.cn)
- neaten portforward code now that readyChan is accepted in New()
  (jminter@redhat.com)
- Enable extended validation check on all routes admitted in by the router.
  Update generated docs/manpage (smitram@gmail.com)
- One can now allocate hostsubnets for hosts that are not part of the cluster.
  This is useful when a host wants to be part of the SDN, but not part of the
  cluster (e.g. F5) (rchopra@redhat.com)
- bump(github.com/RangelReale/osincli):
  fababb0555f21315d1a34af6615a16eaab44396b (mkhan@redhat.com)

* Wed Oct 12 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.10
- Merge remote-tracking branch upstream/master, bump origin-web-console b60172f
  (tdawson@redhat.com)
- Update ose_images.sh - 2016-10-10 (tdawson@redhat.com)
- deployments: use centos:centos7 instead of deployment-example image
  (mfojtik@redhat.com)
- Warn if no login idps are present (jliggitt@redhat.com)
- Updated sysctl usage for cpu (johannes.scheuermann@inovex.de)

* Mon Oct 10 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.9
- Merge remote-tracking branch upstream/master, bump origin-web-console 6a78401
  (tdawson@redhat.com)
- Test case must handle caches now (ccoleman@redhat.com)
- Avoid doing a naive DeepEquals on all PolicyRules (ccoleman@redhat.com)
- Try a cached RuleResolver prior to the underlying in a few cases
  (ccoleman@redhat.com)
- Collapse local and cluster PolicyRule retrieval (ccoleman@redhat.com)
- Upgrade note and swagger for restoring automatic=false behavior in 1.4
  (mkargaki@redhat.com)
- Update ose_images.sh - 2016-10-07 (tdawson@redhat.com)
- Use utilwait.ExponentialBackoff instead of looping (danw@redhat.com)
- Split out SDN setup code from HostSubnet-monitoring code (danw@redhat.com)
- Split out EgressNetworkPolicy-monitoring code into its own file
  (danw@redhat.com)
- Tests for namespaced pruning and necessary refactor (maszulik@redhat.com)
- Suppress node configuration details during tests (mkhan@redhat.com)
- Bug 1371511 - add namespace awareness to oadm prune commands
  (maszulik@redhat.com)
- add tests, updated docs (jvallejo@redhat.com)
- UPSTREAM: 33319: Add option to set a nodeport (jvallejo@redhat.com)
- oc: generated code for 'rollout latest' (mkargaki@redhat.com)
- oc: add rollout latest (mkargaki@redhat.com)
- Correctly parse git versions with longer short hashes (danw@redhat.com)
- Change default node permission check to <apiVerb> nodes/proxy
  (jliggitt@redhat.com)
- Add support for using PATCH when add/remove vol (gethemant@gmail.com)
- fix to unaddressed nits in #10964 for AGL diagnostics (jcantril@redhat.com)
- extended: move conformance tag in top level test describers
  (mkargaki@redhat.com)
- extended: deployment with multiple containers using a single ICT
  (mkargaki@redhat.com)
- deploy: generated code for deployment conditions (mkargaki@redhat.com)
- deploy: api for deploymentconfig Conditions (mkargaki@redhat.com)
- dind: install 'less' to ease debugging (marun@redhat.com)
- dind: slim down base image size (marun@redhat.com)
- dind: update warning of system modification (marun@redhat.com)
- dind: enable ssh access to cluster (marun@redhat.com)
- Refactor dind (marun@redhat.com)

* Fri Oct 07 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.8
- Merge remote-tracking branch upstream/master, bump origin-web-console 2f78dcc
  (tdawson@redhat.com)
- Update ose_images.sh - 2016-10-05 (tdawson@redhat.com)
- Updated link to host subnet routing documentation. (pep@redhat.com)
- Allow multiple segments in DockerImageReference (agladkov@redhat.com)
- Updates `oc run` examples (jtslear@gmail.com)
- Introduce targets for building RPMs to the Makefile (skuznets@redhat.com)
- fix error messages for clusterrolebinding (jtanenba@redhat.com)
- Fixup tito tagger and builder libs (tdawson@redhat.com)
- Remove manpage generation from Tito build (skuznets@redhat.com)
- Update ovs.AddPort()/DeletePort() semantics (danw@redhat.com)
- Add a "global" type to pkg/util/ovs (danw@redhat.com)
- Make pkg/util/ovs and pkg/util/ipcmd tests check the passed-in command line
  (danw@redhat.com)

* Wed Oct 05 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.7
- Merge remote-tracking branch upstream/master, bump origin-web-console 02b3c89
  (tdawson@redhat.com)
- extended: move deployment fixtures in separate directory
  (mkargaki@redhat.com)
- UPSTREAM: 33677: add linebreak between resource groups (jvallejo@redhat.com)
- [origin-aggregated-logging 207] Add diagnostics for aggregated logging
  (jcantril@redhat.com)
- extended: bump image tagging timeout (mkargaki@redhat.com)
- encode/decode nested objects in SubjectRulesReviewStatus (mfojtik@redhat.com)
- generate protobuf (mfojtik@redhat.com)
- add validation (mfojtik@redhat.com)
- add user options to can-i (deads@redhat.com)
- add other user rules review (deads@redhat.com)
- add option to `oc whoami` that prints server url (jvallejo@redhat.com)
- Update ose_images.sh - 2016-10-03 (tdawson@redhat.com)
- Network plugin dir default removed (ccoleman@redhat.com)
- generated: docs (ccoleman@redhat.com)
- Client defaulting changed in Kube v1.4.0 (ccoleman@redhat.com)
- UPSTREAM: <drop>: Continue to carry the cAdvisor patch (ccoleman@redhat.com)
- bump(k8s.io/kubernetes):v1.4.0 (ccoleman@redhat.com)
- Disable cgo when building extended.test (marun@redhat.com)
- Fix dlv invocation in networking.sh (marun@redhat.com)
- More generated code (mkargaki@redhat.com)
- deploy: generated protobuf for instantiate (mkargaki@redhat.com)
- deploy: update generated code for instantiate (mkargaki@redhat.com)
- Make automatic=false work again as it should (mkargaki@redhat.com)
- Run admission for /instantiate (mkargaki@redhat.com)
- oc: switch deploy --latest to use new instantiate endpoint
  (mkargaki@redhat.com)
- deploy: remove imagechange controller (mkargaki@redhat.com)
- deploy: instantiate deploymentconfigs (mkargaki@redhat.com)
- e2e: schema2 config test amendments (miminar@redhat.com)
- line continue should be instead by break (li.guangxu@zte.com.cn)
- ppc64le platform support for build-cross.sh script (mkumatag@in.ibm.com)

* Mon Oct 03 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.6
- suggest use of `oc explain` in oc get output (jvallejo@redhat.com)
- UPSTREAM: 31818: suggest use of `oc explain` in oc get output
  (jvallejo@redhat.com)
- Fix unclosed tag in OpenAPI spec (ccoleman@redhat.com)
- Enable pods-per-core by default, and increase max pods (decarr@redhat.com)
- fix bug 1371047 When JenkinsPipeline build in New status, should not display
  its stages info (jminter@redhat.com)
- introduce no proxy setting for git cloning, with defaulter
  (bparees@redhat.com)
- relabel build compatibility test (cewong@redhat.com)
- fix fake_buildconfigs methods (guilhermebr@gmail.com)
- Re-enable the networking tests. (marun@redhat.com)
- generated swagger spec (salvatore-dario.minonne@amadeus.com)
- SCC check API: REST (salvatore-dario.minonne@amadeus.com)

* Fri Sep 30 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.5
- Merge remote-tracking branch upstream/master, bump origin-web-console 42e31a5
  (tdawson@redhat.com)
- Login must ignore some SSL cert errors when --insecure (ffranz@redhat.com)
- make oc project work with kube (deads@redhat.com)
- Fix proto generation and swagger generation (ccoleman@redhat.com)
- Master broken due to simultaneous merges around OpenAPI (ccoleman@redhat.com)
- make oc login tolerate kube with --token (deads@redhat.com)
- Add a simple test pod for running a kube-apiserver and etcd
  (ccoleman@redhat.com)
- cluster up: do not re-initialize a cluster that already has been initialized
  (cewong@redhat.com)
- cluster up remove temporary files, fixes #9385 (jminter@redhat.com)
- Bug 1312230 - fix image import command to reflect the actual status
  (maszulik@redhat.com)
- Generate and write an OpenAPI spec and copy protos (ccoleman@redhat.com)
- UPSTREAM: 33337: Order swagger docs properly (ccoleman@redhat.com)
- UPSTREAM: 33007: Register versioned.Event instead of *versioned.Event
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: Stably sort openapi request parameters
  (ccoleman@redhat.com)
- Update descriptions on all resources and remove definitions
  (ccoleman@redhat.com)
- Favor --show-token over --token for whoami (jliggitt@redhat.com)
- bump(github.com/openshift/source-to-image):
  785cfb47dab175271d96de9b6e7007ce5a3b811c (bparees@redhat.com)
- convert between s2i authconfig and fsouza authconfig types
  (bparees@redhat.com)
- deploy: generated code for updatePercent removal (mkargaki@redhat.com)
- Remove deprecated updatePercent field from deployment configs
  (mkargaki@redhat.com)
- A few test/cmd/sdn.sh fixups (danw@redhat.com)
- use field.Required for required username (pweil@redhat.com)
- Add details for field.Required (sgallagh@redhat.com)
- fix tito builder now that provides exceeds argument restriction on subprocess
  (maxamillion@fedoraproject.org)
- Revert "Allow startup to continue even if nodes don't have
  EgressNetworkPolicy list permission" (danw@redhat.com)
- add option to set host for profile endpoint (pweil@redhat.com)
- Bind socat to 127.0.0.1 when using it on OS X (cewong@redhat.com)
- lookup buildconfigs in shared indexed cache instead of listing
  (bparees@redhat.com)
- Add PKCE support (jliggitt@redhat.com)
- Adapt to ClientSecretMatches interface (jliggitt@redhat.com)
- bump(github.com/RangelReale/osin): 839b9f181ce80c6118c981a3f636927b3bb4f3a7
  (jliggitt@redhat.com)
- only set app label if it is not present on any object (bparees@redhat.com)
- master: check both places to fix bug 1372618 (lmeyer@redhat.com)
- sdn: set veth TX queue length to unblock QoS (dcbw@redhat.com)
- Allow job controller to get jobs (jliggitt@redhat.com)
- etcd install and version test is broken (ccoleman@redhat.com)
- Switch to using the new 'embed' package to launch etcd (ccoleman@redhat.com)
- UPSTREAM: coreos/etcd: 6463: Must explicitly enable http2 for Go 1.7 TLS
  (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):v3.1.0-alpha.1 (ccoleman@redhat.com)
- Update to start-build --follow, somewhat contradicting #6268
  (jminter@redhat.com)
- Fix the documentation to show how to set a custom message
  (bbennett@redhat.com)
- Fix osadm manage-node --schedulable (marun@redhat.com)
- Extended previous version compatibility tests (cewong@redhat.com)
- function addDefaultEnvVar may have risk of nil pointer
  (li.guangxu@zte.com.cn)
- start-build from-webhook: use canonical hostport to compare with config
  (cewong@redhat.com)
- Explicit pull the docker base image for docker build (Haowang@redhat.com)
- Update preferred ciphers (jliggitt@redhat.com)

* Wed Sep 28 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.4
- Merge remote-tracking branch upstream/master, bump origin-web-console 068713d
  (tdawson@redhat.com)
- Fix a bug in text/extended/networking/ovs.go if a test fails
  (danw@redhat.com)
- Add test/cmd/sdn.sh (danw@redhat.com)
- Fix use of deprecated PodSpec field name (danw@redhat.com)
- Kill off SDN "Registry" type (danw@redhat.com)
- Make NetworkInfo just cache cluster/service CIDRs (danw@redhat.com)
- Move remaining NetworkInfo-related code out of registry.go (danw@redhat.com)
- Move NetworkInfo from Registry to OsdnNode/OsdnMaster/ovsProxyPlugin
  (danw@redhat.com)
- add test for mixed kinds output (jvallejo@redhat.com)
- UPSTREAM: 32222: ensure resource prefix on single type (jvallejo@redhat.com)
- oc: warn for rolling deployments with RWO volumes (mkargaki@redhat.com)
- The function 'NewPrintNameOrErrorAfter' is defined, but it's not used
  (miao.yanqiang@zte.com.cn)
- add hostmanager option to Vagrantfile (jminter@redhat.com)
- Fixed an error description of func, and deleted a redundant error check
  (miao.yanqiang@zte.com.cn)
- Update generated docs (ffranz@redhat.com)
- Fixes docs generation (ffranz@redhat.com)
- Update to released Go 1.7.1 (ffranz@redhat.com)
- add localhost:9000 as a default redirect URL (jvallejo@redhat.com)
- deployment: use event client to create events directly (mfojtik@redhat.com)
- deployment: use rc namespacer instead of function (mfojtik@redhat.com)
- deploy: forward warnings from deployer to deployment config
  (mfojtik@redhat.com)
- deploy: show replication controller warnings in deployer log
  (mfojtik@redhat.com)
- UPSTREAM: 33489: Log test error (jliggitt@redhat.com)
- Bump origin-web-console (fc61bfe) (jforrest@redhat.com)
- UPSTREAM: 33464: Fix cache expiration check (jliggitt@redhat.com)
- UPSTREAM: 33464: Allow testing LRUExpireCache with fake clock
  (jliggitt@redhat.com)
- extended: fix polling the status of a config that needs to be synced
  (mkargaki@redhat.com)
- Revert "Avoid using bsdtar for extraction during build" (bparees@redhat.com)
- Extract duplicate code (miao.yanqiang@zte.com.cn)
- UPSTREAM: 31163: add resource filter handling (jvallejo@redhat.com)
- refactor cli cmds: newapp, newbuild, logs, request_project and create unit
  tests (guilhermebr@gmail.com)
- Fix bug 1312278 Jenkins template has hardcoded SSL certificate.
  (jminter@redhat.com)
- Changed directory of slave image to /tmp (rymurphy@redhat.com)
- Refactor support for GitLab Push Event test case (admin@example.com)
- UPSTREAM: 32230: suggest-exposable resources in oc expose
  (jvallejo@redhat.com)
- suggest-exposable resources in oc expose (jvallejo@redhat.com)
- add --timeout flag to oc rsh (jvallejo@redhat.com)
- Add support for GitLab Push Event (admin@example.com)
- refactor the function 'getPlugin' in the
  'pkg/cmd/experimental/ipfailover/ipfailover.go' (miao.yanqiang@zte.com.cn)
- Ensure CLI and web console clients are public OAuth clients, pass token url
  to web console (jliggitt@redhat.com)
- suggest specifying flags before resource name (jvallejo@redhat.com)
- delete a unused function in the 'pkg/cmd/admin/policy/policy.go'
  (miao.yanqiang@zte.com.cn)

* Mon Sep 26 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.3
- The VS and dot is seprated (yu.peng36@zte.com.cn)
- add wildfly 10.1 spec (ch.raaflaub@gmail.com)
- fix deployment extended test condition (mfojtik@redhat.com)
- extended: Added new image pruning test (miminar@redhat.com)
- extended: Allow to re-deploy registry pod (miminar@redhat.com)
- Prune image configs of manifests V2 schema 2 (miminar@redhat.com)
- extended: Fix binary build helper (miminar@redhat.com)
- Renamed LayerDeleter to LayerLinkDeleter (miminar@redhat.com)
- fix panic when deployment is nil in describe (mfojtik@redhat.com)
- make oc set image resolve image stream images and tags using --source flag
  (mfojtik@redhat.com)
- UPSTREAM: 33083: Add ResolveImage function to CLI factory
  (mfojtik@redhat.com)
- add test case for function pushImage (li.guangxu@zte.com.cn)

* Fri Sep 23 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.2
- extended: fix latest deployment test flakes (mkargaki@redhat.com)
- verify and update protobufs in make verify/update (bparees@redhat.com)
- refactor cancelbuild cli cmd and add unit tests (guilhermebr@gmail.com)
- extract git revision condition (li.guangxu@zte.com.cn)
- The generated manpages were out of date. (bleanhar@redhat.com)
- Simplify binary build output in new-build #11000 (jminter@redhat.com)
- Fix NetNamespaces, which got accidentally broken in 58e8f93 (danw@redhat.com)
- Remove word additionaly (gethemant@gmail.com)
- UPSTREAM: 33141: don't mutate original client TLS config
  (jliggitt@redhat.com)
- Add bit about downloadable swagger-ui (gethemant@gmail.com)
- Update ose scripts to use 3.4 (tdawson@redhat.com)
- Error when test-cmd is given an invalid regex (mkhan@redhat.com)
- add /spec access for node (deads@redhat.com)
- Bump the timeout for deployment logs (mfojtik@redhat.com)
- Improve BuildConfig error reporting (mmilata@redhat.com)
- Changes the signature of CloneWithOptions function (rymurphy@redhat.com)
- fix noinput error msg in newbuild cli cmd (guilhermebr@gmail.com)
- Make links effective (zhao.sijun@zte.com.cn)
- return back the name you GETed for ISI (deads@redhat.com)
- Should hide the labels for route by oc get route (pcameron@redhat.com)
- Fix clusterresourcequota annotations validation (ffranz@redhat.com)
- extended test (mfojtik@redhat.com)
- Preserve valueFrom when merging envs in deployment strategy params
  (mfojtik@redhat.com)
- use a unique volume name for each input image secret (bparees@redhat.com)
- oc: generated code for 'set image' (mkargaki@redhat.com)
- oc: wire set image from kubectl (mkargaki@redhat.com)
- Remove terminating checks from origin namespace lifecycle admission
  (jliggitt@redhat.com)
- UPSTREAM: 32719: compensate for raft/cache delay in namespace admission
  (jliggitt@redhat.com)
- Made hello-openshift read the message from an environment variable
  (bbennett@redhat.com)
- modify the error info of test case (li.guangxu@zte.com.cn)
- remove call to compinit in zsh completion output (jvallejo@redhat.com)
- UPSTREAM: 32142: remove call to compinit in zsh completion output
  (jvallejo@redhat.com)
- SCC check API: refactor (salvatore-dario.minonne@amadeus.com)
- the open dockerfile need to be closed (li.guangxu@zte.com.cn)
- use const string for auth type if exist (li.guangxu@zte.com.cn)
- Use errors.New() instead of fmt.Errorf() where possible.
  (vsemushi@redhat.com)
- Allow users to choose database images version in the DB templates
  (vdinh@redhat.com)

* Tue Sep 20 2016 Troy Dawson <tdawson@redhat.com> 3.4.0.1
- Update tito to build for 3.4 (tdawson@redhat.com)
- Change spec file to the 3.4 branch (tdawson@redhat.com)
- Do not due bundled deps when building (tdawson@redhat.com)
- Enable defaults from upstream e2e framework, including logging
  (ccoleman@redhat.com)
- Suggest `oc get dc` in output of `oc deploy` (jvallejo@redhat.com)
- Add verbose output for `oadm groups new` (mkhan@redhat.com)
- Immediate exit if the return of  is (miao.yanqiang@zte.com.cn)
- Delete the extra dot (yu.peng36@zte.com.cn)
- Add bash symbol (yu.peng36@zte.com.cn)
- bump(github.com/coreos/etcd):v3.0.9 (ccoleman@redhat.com)
- Retry service account update correctly (jliggitt@redhat.com)
- Added necessary policies for running scheduled jobs (maszulik@redhat.com)
- Fix invalid hyperlink. (warmchang@outlook.com)
- Allow context to be provided to hack/env itself (ccoleman@redhat.com)
- UPSTREAM: docker/docker: <carry>: WORD/DWORD changed (ccoleman@redhat.com)
- Default qps/burst to historical values (jliggitt@redhat.com)
- Ensure system:master has full permissions on non-resource-urls
  (jliggitt@redhat.com)
- Disable go 1.6 in travis - it can't compile in under 10 minutes
  (ccoleman@redhat.com)
- Disable networking until it has been fixed (ccoleman@redhat.com)
- UPSTREAM: google/cadvisor: <drop>: Hack around cgroup load
  (ccoleman@redhat.com)
- generated: proto (ccoleman@redhat.com)
- generated: docs, swagger, and completions (ccoleman@redhat.com)
- Protobuf stringers disabled for OptionalNames (ccoleman@redhat.com)
- Debug: use master IP instead of localhost in nodeips (ccoleman@redhat.com)
- ImageStream tests now require user on context (ccoleman@redhat.com)
- Router event sends ADDED twice in violation of EventQueue semantics
  (ccoleman@redhat.com)
- Extended tests that are not yet ready for origin (ccoleman@redhat.com)
- Add the GC controller but default to off for now (ccoleman@redhat.com)
- Quota exceeded error message changed (ccoleman@redhat.com)
- test/cmd/migrate can race on image import under load (ccoleman@redhat.com)
- JSONPath output for nil changed slightly (ccoleman@redhat.com)
- Finalizers are reenabled temporarily (ccoleman@redhat.com)
- Describe should use imageapi.ParseImageStreamImageName (ccoleman@redhat.com)
- Check for GNU sed in os::util::sed (ccoleman@redhat.com)
- Add a simple prometheus config file (ccoleman@redhat.com)
- Error message from server has changed (ccoleman@redhat.com)
- Generators added scheduled jobs (ccoleman@redhat.com)
- Security admission now uses indexer and requires namespaces
  (ccoleman@redhat.com)
- Deployment related refactors (ccoleman@redhat.com)
- ImageStream limits should not start its own reflector (ccoleman@redhat.com)
- Wait for v2 registry test for server to come up (ccoleman@redhat.com)
- Update all permissions (ccoleman@redhat.com)
- Small refactors to tests to pass (ccoleman@redhat.com)
- Add DefaultStorageClass admission and disable by default ImagePolicyWebhook
  (ccoleman@redhat.com)
- Master configuration changes (ccoleman@redhat.com)
- Enable scheduled jobs controller and disruption budgets conditionally
  (ccoleman@redhat.com)
- Changes to util package reflected in tests (ccoleman@redhat.com)
- Add new top commands to 'oadm top' (ccoleman@redhat.com)
- Remove support for spec.portalIP (ccoleman@redhat.com)
- Dockerfile parser updates (ccoleman@redhat.com)
- Comment that SAR on namespace requires an upstream patch
  (ccoleman@redhat.com)
- DeepCopy requires pointer inputs now (ccoleman@redhat.com)
- Go 1.7 does not allow empty string req.URL.Path (ccoleman@redhat.com)
- Man page generation should skip hidden flags (ccoleman@redhat.com)
- LeaderLease off by one (ccoleman@redhat.com)
- Update integration tests with compilation changes (ccoleman@redhat.com)
- util.Clock moved to util/clock (ccoleman@redhat.com)
- Owner references are now required for kubernetes controllers
  (ccoleman@redhat.com)
- Update extended test code for 1.4 (ccoleman@redhat.com)
- Move the leaderlease package to the new etcd client (ccoleman@redhat.com)
- MasterLeaser was incorrect with new storage config (ccoleman@redhat.com)
- Use Quorum reads for OAuth and service account token reads
  (ccoleman@redhat.com)
- Support new Unstructured flows for get and create in factory
  (ccoleman@redhat.com)
- Add StorageClass access to the PV binder (ccoleman@redhat.com)
- Register Kubernetes log package early (ccoleman@redhat.com)
- Update Kubernetes node and controllers (ccoleman@redhat.com)
- Simple master signature changes (ccoleman@redhat.com)
- Refactor NewConfigGetter again to get closer to correct behavior
  (ccoleman@redhat.com)
- Switch to using storagebackend.Factory consistently (ccoleman@redhat.com)
- Use kubernetes Namespace informer, send Informers to admission
  (ccoleman@redhat.com)
- Accept cache.TriggerPublisher on ApplyOptions, use StorageFactory
  (ccoleman@redhat.com)
- Signature changes to CLI commands (ccoleman@redhat.com)
- Other kubectl and smaller moves (ccoleman@redhat.com)
- Printer takes options by value and GetPrinter takes noHeaders
  (ccoleman@redhat.com)
- Simplify AuthorizationAdapter now that upstream matches our signature
  (ccoleman@redhat.com)
- Deployment changes related to Scaler/Rollbacker/Client (ccoleman@redhat.com)
- Repair package changed (ccoleman@redhat.com)
- Validation constants are no longer public (ccoleman@redhat.com)
- Use go-dockerclient.ParseRepositoryTag() (ccoleman@redhat.com)
- Update to use generic.SelectionPredicate (ccoleman@redhat.com)
- Rename GV*.IsEmpty() to GV*.Empty() (ccoleman@redhat.com)
- Switch to new SchemeBuilder registration (ccoleman@redhat.com)
- Add kapi.Context to PrepareForCreate/PrepareForUpdate (ccoleman@redhat.com)
- Update API types with proper generator tags (ccoleman@redhat.com)
- Update gendeepcopy and genconversion (ccoleman@redhat.com)
- bump(k8s.io/kubernetes):d19513fe86f3e0769dd5c4674c093a88a5adb8b4
  (ccoleman@redhat.com)
- Enable temporary swap when on Linux and memory is low (ccoleman@redhat.com)
- Preserve Go build artifacts from test compilation (ccoleman@redhat.com)
- Update generated clientsets should use get_version_vars (ccoleman@redhat.com)
- Add cloudflare/cfssl as an upstream to restore Godeps from
  (ccoleman@redhat.com)
- The pod GC controller has been moved to a different package
  (ccoleman@redhat.com)
- Switch to github.com/docker/go-units (ccoleman@redhat.com)
- Net utilities have been moved, update imports (ccoleman@redhat.com)
- Strip removed deployment utility package (ccoleman@redhat.com)
- Refactor dockerfile code to latest Docker 1.12 (ccoleman@redhat.com)
- Magic Git tag transformations are awesome too (ccoleman@redhat.com)
- Release regex is awesome (ccoleman@redhat.com)
- remove the redundant spaces (haowang@redhat.com)
- Run zookeeper as non-root user. Fix zookeeper conf volumeMount.
  (ghyde@redhat.com)
- The promt is missed (yu.peng36@zte.com.cn)
- The 'eg' is should be 'e.g.', and 'for example' is better here
  (yu.peng36@zte.com.cn)
- use a post deploy hook that will pass, not fail (bparees@redhat.com)
- Allow annotation selector to match annotation values (jliggitt@redhat.com)
- Add service account for attach-detach controller (pmorie@redhat.com)
- Add logging to assist in determining openid claims (jliggitt@redhat.com)
- tolerate multiple =s in a parameter value (bparees@redhat.com)
- remove all tmp dir after test (li.guangxu@zte.com.cn)
- Fix issue#10853. Route cleanup does not need service key cleanup.
  (rchopra@redhat.com)
- sdn: convert from OpenShift client cache to k8s DeltaFIFO (dcbw@redhat.com)
- delete the unused function NameFromImageStream (li.guangxu@zte.com.cn)
- hack/release.sh should os::log::warn (ccoleman@redhat.com)
- Allow hack/env to reuse volumes (ccoleman@redhat.com)
-   Fix some typos for router_sharding.md (li.xiaobing1@zte.com.cn)
- Don't fail during negotiation when /oapi is missing (ccoleman@redhat.com)
- Make Kube version info best effort, and don't fail fast (ccoleman@redhat.com)
- JenkinsPipelineStrategy nil no need check again (li.guangxu@zte.com.cn)
- Added a mention of 'oc get' to 'oc get --help' (rymurphy@redhat.com)
- extended: add readiness test for initial deployments (mkargaki@redhat.com)
- deploy: deployment utilities and cmd fixes (mkargaki@redhat.com)
- extended: update polling for deployment tests (mkargaki@redhat.com)
- delete the redundant loop that save slice to map (li.guangxu@zte.com.cn)
- deploy: use upstream pod lister method (mkargaki@redhat.com)
- deploy: move cancellation event just after the deployment update
  (mkargaki@redhat.com)
- Fix a func name and return value in the comment of NewREST
  (wang.yuexiao@zte.com.cn)

* Tue Sep 06 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.30
- Fix wrong oadm document name in cli.md (li.xiaobing1@zte.com.cn)
- Image size needs to add a size of manifest config file (miminar@redhat.com)
- New e2e test: fetch manifest schema 2 with old client (miminar@redhat.com)
- Pullthrough blobs using Get() as well (miminar@redhat.com)
- Remember image with matching config reference (miminar@redhat.com)
- website changed for openshift and can be accessed directly
  (li.xiaobing1@zte.com.cn)
- UPSTREAM: <carry>: Tolerate node ExternalID changes with no cloud provider
  (pmorie@redhat.com)
- UPSTREAM: 32000: Update node status instead of node in kubelet
  (pmorie@redhat.com)
- Release should pass OS_GIT_COMMIT as a commit, not a tag
  (ccoleman@redhat.com)
- Reconcile non-resource-urls (jliggitt@redhat.com)
- UPSTREAM: 31627: make deep copy of quota objects before mutations
  (deads@redhat.com)

* Fri Sep 02 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.29
- The etc and dot is seprated (yu.peng36@zte.com.cn)
- Add a Docker image that will be built separately to run observe
  (ccoleman@redhat.com)
- generated: Completions and doc (ccoleman@redhat.com)
- Add an experimental observe command to handle changes to objects
  (ccoleman@redhat.com)
- UPSTREAM: 31714: Allow missing keys in jsonpath (ccoleman@redhat.com)
- UPSTREAM: <carry>: Tolerate node ExternalID changes with no cloud provider
  (pmorie@redhat.com)
- UPSTREAM: 31730: Make it possible to enable controller-managed attach-detach
  on existing nodes (pmorie@redhat.com)
- UPSTREAM: 31531: Add log message in Kuelet when controller attach/detach is
  enabled (pmorie@redhat.com)
- UPSTREAM: revert: 88abe47b9a963b41acd0c4f0fd18827192648468: <carry>: Tolerate
  node ExternalID changes with no cloud provider (pmorie@redhat.com)
- Bump origin-web-console (7d59453) (jforrest@redhat.com)
- deploy: don't requeue configs on stream updates yet (mkargaki@redhat.com)
- eg. should be e.g. (yu.peng36@zte.com.cn)
- bump(k8s.io/kubernetes):52492b4bff99ef3b8ca617d385a3ff0612f9402d
  (ccoleman@redhat.com)
- Make the router data structures remove duplicates cleanly
  (bbennett@redhat.com)
- Generated man/docs for reverting plugin auto detection (rpenta@redhat.com)
- Revert openshift sdn plugin auto detection on the node (rpenta@redhat.com)
- Compare additionally generation when comparing ImageStreamTag status with
  spec (maszulik@redhat.com)
- extended: bump deployment timeout for test about tagging
  (mkargaki@redhat.com)
- suggest use of `oc get bc` on `oc start-build` error output
  (jvallejo@redhat.com)
- extended: require old deployments to be equal to the expected amount
  (mkargaki@redhat.com)
- delete unused function panicIfStopped (li.guangxu@zte.com.cn)

* Wed Aug 31 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.28
- remove unnecessary service account edit step (bparees@redhat.com)
- Bump origin-web-console (39ffada) (jforrest@redhat.com)
- add test case for function GetDockerAuth (li.guangxu@zte.com.cn)
- add permissions for jenkins (deads@redhat.com)
- Make build spec file platform independent (mkumatag@in.ibm.com)
- Switch deployment utils to loop by index (zhao.sijun@zte.com.cn)
- Retry on registry client connect on tag unset (miminar@redhat.com)
- be tolerant of quota reconciler resetting to old values (deads@redhat.com)
- Return the exit code of the hack/env container (ccoleman@redhat.com)
- update the README outputs of the sample (li.guangxu@zte.com.cn)
- fix protobuff generation for the sdn (ipalade@redhat.com)

* Mon Aug 29 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.27
- Merge remote-tracking branch upstream/master, bump origin-web-console 3ef22d6
  (tdawson@redhat.com)
- Scale helpers: always fetch scale object for DCs (sross@redhat.com)
- Regenerate proxy iptables rules on EgressNetworkPolicy change
  (danw@redhat.com)
- Idle command: make sure to always print errors (sross@redhat.com)
- Restores the service proxier when unidling is disabled (bbennett@redhat.com)
- Automatically check and update docker-registry configuration
  (agladkov@redhat.com)
- Bump origin-web-console (b0cc495) (jforrest@redhat.com)
- UPSTREAM: openshift/source-to-image: 576: increase default docker timeout
  (gmontero@redhat.com)
- fix `oc set env` key-value pair matching (jvallejo@redhat.com)
- extended: debug failed tag update (mkargaki@redhat.com)
- Unidling Controller: Deal with tombstones in cache (sross@redhat.com)
- SCC API check: move (salvatore-dario.minonne@amadeus.com)

* Fri Aug 26 2016 Scott Dodson <sdodson@redhat.com> 3.3.0.26
- [RPMs] tito builds are always clean (sdodson@redhat.com)
- UPSTREAM: 31463: Add a line break when no events in describe
  (ffranz@redhat.com)
- Bump origin-web-console (98cd97c) (jforrest@redhat.com)
- UPSTREAM: 31446: Do initial 0-byte write to stdout when streaming container
  logs (jliggitt@redhat.com)
- UPSTREAM: 31446: Make limitWriter respect 0-byte writes until limit is
  reached (jliggitt@redhat.com)
- UPSTREAM: 31446: Send ping frame using specified encoding
  (jliggitt@redhat.com)
- UPSTREAM: 31396: Fixed integer overflow bug in rate limiter
  (deads@redhat.com)
- UPSTREAM: docker/distribution: 1703: GCS: FileWriter.Size: include number of
  buffered bytes if the FileWriter is not closed (mfojtik@redhat.com)
- Remove NotV2Registry check from redhat registry import test
  (mfojtik@redhat.com)
- fix autoprovision enabled field name (bparees@redhat.com)
- UPSTREAM: 31353: add unit test for duplicate error check
  (jvallejo@redhat.com)
- Bump origin-web-console (18a0d95) (jforrest@redhat.com)
- update invalid oc extract usage flag to "keys" (jvallejo@redhat.com)
- UPSTREAM: 31353: fix duplicate validation/field/errors (jvallejo@redhat.com)
- deploy: do not retry when deployer pod fail to start (mfojtik@redhat.com)
- Revert "add test to exercise deployment logs for multi-container pods"
  (mfojtik@redhat.com)
- Revert "Pass the container name to deployment config log options"
  (mfojtik@redhat.com)
- fix 'oadm policy' description error (miao.yanqiang@zte.com.cn)
- cluster up: use rslave propagation mode with nsenter mounter
  (cewong@redhat.com)
- when sercret was found need break the loop (li.guangxu@zte.com.cn)
- Add OVS rule creation/cleanup tests to extended networking test
  (danw@redhat.com)

* Wed Aug 24 2016 Scott Dodson <sdodson@redhat.com> 3.3.0.25
- Emit event when cancelling a deployment (mfojtik@redhat.com)
- deployments: make retries faster on update conflicts (mfojtik@redhat.com)
- UPSTREAM: 30896: Add Get() to ReplicationController lister
  (mfojtik@redhat.com)
- Bump origin-web-console (d24e068) (jforrest@redhat.com)
- Removing the VOLUME directive in the ose image (abhgupta@redhat.com)
- HAProxy Router: Invert health-check idled check (sross@redhat.com)
- note the change to oc tag -d (deads@redhat.com)
- Improve oc new-app examples. (vsemushi@redhat.com)
- sdn: clear kubelet-created initial NetworkUnavailable condition on GCE
  (dcbw@redhat.com)
- add test for non-duplicate error outputs (jvallejo@redhat.com)
- increase jenkins readiness timeout (bparees@redhat.com)
- An independent branch dealing with errors (root@master0.cloud.com.novalocal)
- private function mergeWithoutDuplicates no longer in use
  (li.guangxu@zte.com.cn)
- Clean up requested project if there are errors creating template items
  (jliggitt@redhat.com)
- retrieve keys from cache.Delta objects properly (bparees@redhat.com)
- Bump origin-web-console (0ab36ca) (jforrest@redhat.com)
- Fix gitserver bc search (cewong@redhat.com)
- Support init containers in 'oc debug' (agoldste@redhat.com)
- UPSTREAM: 31150: Check init containers in PodContainerRunning
  (agoldste@redhat.com)
- update swagger dev README with updated script name (aweiteka@redhat.com)
- fix pre-deploy hook args on cakephp example (bparees@redhat.com)
- Enable secure cookie for secure-only edge routes (marun@redhat.com)
- Usability improvements to failures in oc login (ffranz@redhat.com)
- fix json serialization in admissionConfig (deads@redhat.com)
- Re-setup SDN on startup if ClusterNetworkCIDR changes (danw@redhat.com)
- UPSTREAM: 30313: add unit test for duplicate errors (jvallejo@redhat.com)
- UPSTREAM: 30313: remove duplicate errors from aggregate error outputs
  (jvallejo@redhat.com)
- add suggestion to use `describe` to obtain container names
  (jvallejo@redhat.com)
- test: move e2e deployment test in extended suite (mkargaki@redhat.com)
- It is better to add "\n" in printf (yu.peng36@zte.com.cn)
- UPSTREAM: 30717: add suggestion to use `describe` to obtain container names
  (jvallejo@redhat.com)
- Fix typo in gen_man.go (mkumatag@in.ibm.com)
- Run updated docs (mdame@redhat.com)
- UPSTREAM: 28234: Make sure --record=false is acknowledged when passed to
  commands (mdame@redhat.com)
- Fix somee mistakes in script as follow: (wang.yuexiao@zte.com.cn)
- deploy: remove top level generator pkg (mkargaki@redhat.com)
- deploy: update validation for deploymentconfigs (mkargaki@redhat.com)
- Regenerate swagger docs (danw@redhat.com)
- Improve the SDN API docs a little bit (danw@redhat.com)
- Regenerate oc completions (danw@redhat.com)
- Add "oc describe" support for sdn types (danw@redhat.com)

* Mon Aug 22 2016 Scott Dodson <sdodson@redhat.com> 3.3.0.24
- generated: Docs and examples (ccoleman@redhat.com)
- Implement oc set route-backend and A/B to describers / printers
  (ccoleman@redhat.com)
- UPSTREAM: 31047: Close websocket stream when client closes
  (jliggitt@redhat.com)
- Bump origin-web-console (3fb0c3e) (jforrest@redhat.com)
- set image ref namespace when src project not specified (jvallejo@redhat.com)
- Improve the image stream describer (ccoleman@redhat.com)
- test-cmd updates for --raw (deads@redhat.com)
- UPSTREAM: 30445: add --raw for kubectl get (deads@redhat.com)
- Use non-secure cookie when tls and non-tls enabled (marun@redhat.com)
- Add a default ingress ip range (marun@redhat.com)
- UPSTREAM: 25308: fix rollout nil panic issue (agoldste@redhat.com)
- re-enable the build defaulter plugin by default (bparees@redhat.com)
- UPSTREAM: <carry>: JitterUntil should defer to utilruntime.HandleCrash on
  panic (decarr@redhat.com)
- BuildRequire golang greater than or equal to golang_version
  (tdawson@redhat.com)
- -pkgdir should not be root of project (ccoleman@redhat.com)
- add test to exercise deployment logs for multi-container pods
  (mfojtik@redhat.com)
- better debugging info for deployment flakes (mfojtik@redhat.com)
- BZ 1368050: add ability to change connection limits for reencrypt and
  passthrough routes (jtanenba@redhat.com)
- Highlight only active deployment in describer (mfojtik@redhat.com)
- Pass the container name to deployment config log options (mfojtik@redhat.com)
- Retry updating deployment config instead of creating new one in test
  (mfojtik@redhat.com)
- merge duplicate code (li.guangxu@zte.com.cn)
- UPSTREAM: 26680: Don't panic in NodeController if pod update fails
  (agoldste@redhat.com)
- UPSTREAM: 28697: Prevent kube-proxy from panicing when sysfs is mounted as
  read-only. (agoldste@redhat.com)
- UPSTREAM: 28744: Allow a FIFO client to requeue under lock
  (agoldste@redhat.com)
- UPSTREAM: 29581: Kubelet: Fail kubelet if cadvisor is not started.
  (agoldste@redhat.com)
- UPSTREAM: 29672: Add handling empty index key that may cause panic issue
  (agoldste@redhat.com)
- UPSTREAM: 29531: fix kubectl rolling update empty file cause panic issue
  (agoldste@redhat.com)
- UPSTREAM: 29743: Fix race condition found in JitterUntil
  (agoldste@redhat.com)
- UPSTREAM: 30291: Prevent panic in 'kubectl exec' when redirecting stdout
  (agoldste@redhat.com)
- UPSTREAM: 29594: apiserver: fix timeout handler (agoldste@redhat.com)
- disable jenkins auto-deployment (bparees@redhat.com)
- Remove VOLUME from Origin image (ccoleman@redhat.com)
- UPSTREAM: 30690: Don't bind pre-bound pvc & pv if size request not satisfied
  and continue searching on bad size and add tests for bad size&mode.
  (avesh.ncsu@gmail.com)
- update image policy API for first class resolution (deads@redhat.com)
- Router provides an invalid, expired certificate (pcameron@redhat.com)
- clarify idle error and usage output (jvallejo@redhat.com)
- Fixed extended unilding tests (sross@redhat.com)
- HAProxy Router: Don't health-check idled services (sross@redhat.com)
- Fix for BZ#1367937 which prevents CEPH RBD from working on atomic host.
  (bchilds@redhat.com)
- Now check for length on error array (rymurphy@redhat.com)
- UPSTREAM: 30579: Remove trailing newlines on describe (ccoleman@redhat.com)
- To break the for loop when found = true (li.xiaobing1@zte.com.cn)

* Fri Aug 19 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.23
- BuildRequires bsdtar, for hack scripts (tdawson@redhat.com)
- Disable ingress ip when cloud provider enabled (marun@redhat.com)
- Bump origin-web-console (4d411df) (jforrest@redhat.com)
- Fix oc project|projects when in cluster config (ffranz@redhat.com)
- move unrelated extended tests out of images (ipalade@redhat.com)
- bump(k8s.io/kubernetes): 447cecf8b808caa00756880f2537b2bafbfcd267
  (deads@redhat.com)
- UPSTREAM: 29093: Fix panic race in scheduler cache from 28886
  (ccoleman@redhat.com)
- calculate usage on istag creates (deads@redhat.com)
- compute image stream usage properly on istag touches (deads@redhat.com)
- UPSTREAM: 30907: only compute delta on non-creating updates
  (deads@redhat.com)
- UPSTREAM: google/cadvisor: 1359: Make ThinPoolWatcher loglevel consistent
  (agoldste@redhat.com)
- bump(google/cadvisor): 956e595d948ce8690296d297ba265d5e8649a088
  (agoldste@redhat.com)
- Allowed 'true' for the DROP_SYN_DURING_RESTART variable (bbennett@redhat.com)
- Randomize delay in router stress test. (marun@redhat.com)
- Add previous-scale annotation for idled resources (sross@redhat.com)
- Fixed the comment about the different backends we make (bbennett@redhat.com)
- block setting ownerReferences and finalizers (deads@redhat.com)
- UPSTREAM: 30839: queueActionLocked requires write lock (deads@redhat.com)
- UPSTREAM: 30624: Node controller deletePod return true if there are pods
  pending deletion (agoldste@redhat.com)
- UPSTREAM: 30277: Avoid computing DeepEqual in controllers all the time
  (agoldste@redhat.com)
- fix a logical error of the function 'RunCmdRouter' in the , the same as the
  funtion 'RunCmdRegistry' in the (miao.yanqiang@zte.com.cn)
- generate_vrrp_sync_groups calls expand_ip_ranges on an already expanded
  ranges (cameron@braid.com.au)
- Bump origin-web-console (8c03ff4) (jforrest@redhat.com)
- UPSTREAM: 29639: <drop>: Fix default resource limits (node allocatable) for
  downward api volumes and env vars. (avesh.ncsu@gmail.com)
- Fix validation of pkg/sdn/api object updates (danw@redhat.com)
- UPSTREAM: 30731: Always return command output for exec probes and kubelet
  RunInContainer (agoldste@redhat.com)
- UPSTREAM: 30796: Quota usage checking ignores unrelated resources
  (decarr@redhat.com)
- regen protos (bparees@redhat.com)
- Make it easier to extract content from hack/env (ccoleman@redhat.com)
- UPSTREAM: 27541: Attach should work for init containers (ccoleman@redhat.com)
- UPSTREAM: 30736: Close websocket watch when client closes
  (jliggitt@redhat.com)
- Added Git logging to build output (rymurphy@redhat.com)
- deploy: reconcile streams on config creation/updates (mkargaki@redhat.com)
- Bug 1366936: fix ICT matching in the trigger controller (mkargaki@redhat.com)
- Variable definition is not used (li.guangxu@zte.com.cn)
- add test cases for `oc set env` (jvallejo@redhat.com)
- return error when no env args are given (jvallejo@redhat.com)
- check CustomStrategy validation at begin (li.guangxu@zte.com.cn)
- force all plugins to either default off or default on (deads@redhat.com)
- integration: fix imagestream admission flake (miminar@redhat.com)
- Improve tests for extended build. (vsemushi@redhat.com)
- Revert "test: extend timeout in ICT tests to the IC controller resync
  interval" (mkargaki@redhat.com)
- Return original error on on limit error (miminar@redhat.com)
- Fix haproxy config bug. (smitram@gmail.com)
- Fix somee mistakes in script as follow: (wang.yuexiao@zte.com.cn)
- UPSTREAM: 30533: Validate involvedObject.Namespace matches event.Namespace
  (jliggitt@redhat.com)
- support for zero weighted services in a route (rchopra@redhat.com)
- UPSTREAM: 30713: Empty resource type means no defaulting
  (ccoleman@redhat.com)
- Extract should default to current directory (ccoleman@redhat.com)
- deprecate --list option from `volumes` cmd (jvallejo@redhat.com)
- Bug 1330201 - Periodically sync k8s iptables rules (rpenta@redhat.com)
- call out config validation warnings more clearly (deads@redhat.com)
- Show restart count warnings only for latest deployment (mfojtik@redhat.com)
- Fix scrub pod container command (mawong@redhat.com)
- Updated auto generated doc for pod-network CLI commands (rpenta@redhat.com)
- Updated auto generated bash completions for pod-network CLI commands
  (rpenta@redhat.com)
- Added test cases for 'oadm pod-network isolate-projects' (rpenta@redhat.com)
- CLI changes to support project network isolation (rpenta@redhat.com)
- Make pod-network cli command to use ChangePodNetworkAnnotation instead of
  updating VNID directly (rpenta@redhat.com)
- Remove old SDN netid allocator (rpenta@redhat.com)
- Test cases for assign/update/revoke VNIDs (rpenta@redhat.com)
- Handling VNID manipulations (rpenta@redhat.com)
- Test cases for network ID allocator interface (rpenta@redhat.com)
- Added network ID allocator interface (rpenta@redhat.com)
- Test cases for network ID range interface (rpenta@redhat.com)
- Added network ID range interface (rpenta@redhat.com)
- Accessor methods for ChangePodNetworkAnnotation on NetNamespace
  (rpenta@redhat.com)
- have origin.spec use hack scripts to build (tdawson@redhat.com)
- Allow startup to continue even if nodes don't have EgressNetworkPolicy list
  permission (danw@redhat.com)
- add a validateServiceAccount to the creation of ipfailover pods
  (jtanenba@redhat.com)

* Wed Aug 17 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.22
- Return directly if no pods found when evacuating (zhao.xiangpeng@zte.com.cn)
- Bump origin-web-console (5fa2bd9) (jforrest@redhat.com)
- add new-app support for detecting .net apps (bparees@redhat.com)
- Use scheme/host from request for token redirect (jliggitt@redhat.com)
- Modify a error variable (miao.yanqiang@zte.com.cn)
- Revert "add suggestion to use `describe` to obtain container names"
  (ccoleman@redhat.com)
- allow SA oauth clients to list projects (deads@redhat.com)
- namespace scope all our new admission plugins (deads@redhat.com)
- Update to released Go 1.7 (ccoleman@redhat.com)
- Allow registry-admin and registry-editor create serviceaccounts
  (agladkov@redhat.com)
- fail server on bad admission (deads@redhat.com)
- extended: retry on update conflicts in deployment tests (mkargaki@redhat.com)
- test: extend timeout in ICT tests to the IC controller resync interval
  (mkargaki@redhat.com)
- The first letter should be capitalized (yu.peng36@zte.com.cn)
- func getCredentials no longer in use (li.guangxu@zte.com.cn)
- Bug 1365450 - Fix SDN plugin name change (rpenta@redhat.com)
- Bump origin-web-console (bc567c7) (jforrest@redhat.com)
- add suggestion to use `describe` to obtain container names
  (jvallejo@redhat.com)
- bump(github.com/openshift/source-to-image):
  89b96680e451c0fa438446043f967b5660942974 (bparees@redhat.com)
- Moving from enviornment variables to annotaions for healthcheck interval
  (erich@redhat.com)
- record quota errors on image import conditions (pweil@redhat.com)
- oc start-build: display an error when git is not available and --from-repo is
  requested (cewong@redhat.com)
- Adding ENV's for Default Router timeout settings, and adding validation
  (erich@redhat.com)
- UPSTREAM: 30510: Endpoint controller logs errors during normal cluster
  behavior (decarr@redhat.com)
- Improve grant page appearance, allow partial scope grants
  (jliggitt@redhat.com)
- UPSTREAM: 30626: prevent RC hotloop on denied pods (deads@redhat.com)
- remove redundant comment in build-base-images.sh (wang.yuexiao@zte.com.cn)
- if err is nil return nil directly (li.guangxu@zte.com.cn)
- Fix pullthrough serve blob (miminar@redhat.com)
- Ignore negative value of grace-period and add more evacuate examples
  (zhao.xiangpeng@zte.com.cn)
- Pullthrough logging improvements (miminar@redhat.com)
- Allow to pull from insecure registries for unit tests (miminar@redhat.com)
- add func to instead of lengthiness parameters (wang.yuexiao@zte.com.cn)
- Stop using node selector as ipfailover label (marun@redhat.com)
- Changes createLocalGitDirecory to s2i version (rymurphy@redhat.com)
- fix up image policy admission plugin (deads@redhat.com)
- bump(github.com/openshift/source-to-image):
  2878c1ab41784dab0a467e12a200659506174e68 (jupierce@redhat.com)
- Set xff headers for reencrypt[ed] routes. (smitram@gmail.com)
- Delete EgressNetworkPolicy objects on namespace deletion (danw@redhat.com)
- UPSTREAM: 29982: Fix PVC.Status.Capacity and AccessModes after binding
  (jsafrane@redhat.com)

* Mon Aug 15 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.21
- Need to set OS_ROOT for tito (tdawson@redhat.com)
- The first letter should be capitalized (yu.peng36@zte.com.cn)
- bump(github.com/AaronO/go-git-http) 34209cf6cd947cfa52063bcb0f6d43cfa50c5566
  (cewong@redhat.com)
- don't do pod deletion management for pipeline builds (bparees@redhat.com)
- Bump origin-web-console (1e55231) (jforrest@redhat.com)
- Updated bash preamble in test/cmd/run.sh (skuznets@redhat.com)
- Update generated docs and completions (ffranz@redhat.com)
- Fixes required by latest version of Cobra (ffranz@redhat.com)
- bump(github.com/spf13/cobra): f62e98d28ab7ad31d707ba837a966378465c7b57
  (ffranz@redhat.com)
- bump(github.com/spf13/pflag): 1560c1005499d61b80f865c04d39ca7505bf7f0b
  (ffranz@redhat.com)
- Avoid using bsdtar for extraction during build (joesmith@redhat.com)
- Removed executable check from test-cmd test filter (skuznets@redhat.com)
- Remain in the current project at login if possible (jliggitt@redhat.com)
- Moved shared init code into hack/lib/init.sh (skuznets@redhat.com)
- respect scopes in list/watch projects (deads@redhat.com)
- Add zsh compatibility note to `completion` help (jvallejo@redhat.com)
- UPSTREAM: 30460: Add zsh compatibility note `completion` cmd help
  (jvallejo@redhat.com)

* Mon Aug 15 2016 Troy Dawson <tdawson@redhat.com>
- Need to set OS_ROOT for tito (tdawson@redhat.com)
- The first letter should be capitalized (yu.peng36@zte.com.cn)
- bump(github.com/AaronO/go-git-http) 34209cf6cd947cfa52063bcb0f6d43cfa50c5566
  (cewong@redhat.com)
- don't do pod deletion management for pipeline builds (bparees@redhat.com)
- Bump origin-web-console (1e55231) (jforrest@redhat.com)
- Updated bash preamble in test/cmd/run.sh (skuznets@redhat.com)
- Update generated docs and completions (ffranz@redhat.com)
- Fixes required by latest version of Cobra (ffranz@redhat.com)
- bump(github.com/spf13/cobra): f62e98d28ab7ad31d707ba837a966378465c7b57
  (ffranz@redhat.com)
- bump(github.com/spf13/pflag): 1560c1005499d61b80f865c04d39ca7505bf7f0b
  (ffranz@redhat.com)
- Avoid using bsdtar for extraction during build (joesmith@redhat.com)
- Removed executable check from test-cmd test filter (skuznets@redhat.com)
- Remain in the current project at login if possible (jliggitt@redhat.com)
- Moved shared init code into hack/lib/init.sh (skuznets@redhat.com)
- respect scopes in list/watch projects (deads@redhat.com)
- Add zsh compatibility note to `completion` help (jvallejo@redhat.com)
- UPSTREAM: 30460: Add zsh compatibility note `completion` cmd help
  (jvallejo@redhat.com)

* Fri Aug 12 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.19
- Merge remote-tracking branch upstream/master, bump origin-web-console 01e7f6b
  (tdawson@redhat.com)
- mark extended builds experimental (bparees@redhat.com)
- the tmpfile may be not closed when function DownloadFromContainer returns err
  (li.guangxu@zte.com.cn)
- Improving circular dependency checking for new-build (jupierce@redhat.com)
- Adding -o=name to start-build (jupierce@redhat.com)
- add tests; mv printer tests to separate file (jvallejo@redhat.com)
- Add the default image policy to bootstrap bindata (ccoleman@redhat.com)
- Make hack commands more consistent and add policy to bindata
  (ccoleman@redhat.com)
- Enable a simple image policy admission controller (ccoleman@redhat.com)
- Make DefaultRegistryFunc and PodSpec extraction generic (ccoleman@redhat.com)
- Fix govet complains (maszulik@redhat.com)
- Change go version detection logic (maszulik@redhat.com)
- Bump origin-web-console (233c7f3) (jforrest@redhat.com)
- Bug 1363630 - Print shortened parent ID in oadm top images
  (maszulik@redhat.com)
- Revert "Bug 1281735 - remove the internal docker registry information from
  imagestream" (ccoleman@redhat.com)
- fix broken paths and redirects for s2i containers (ipalade@redhat.com)
- show namespace for custom strategy bc (jvallejo@redhat.com)
- remove redundant example from `oc projects` (jvallejo@redhat.com)
- SourceStrategy: assign PullSecret when only RuntimeImage needs it.
  (vsemushi@redhat.com)
- Fix `oc idle` help text (sross@redhat.com)
- Ensure only endpoints are specified in `oc idle` (sross@redhat.com)
- add unit tests for s2iProxyConfig and buildEnvVars (ipalade@redhat.com)
- add test case (jvallejo@redhat.com)
- UPSTREAM: 30162: return err on oc run --image with invalid value
  (jvallejo@redhat.com)
- show project labels (deads@redhat.com)
- add metrics to clusterquota controllers (deads@redhat.com)
- UPSTREAM: 30296: add metrics for workqueues (deads@redhat.com)
- Mark ingress as tech-preview in README (mfojtik@redhat.com)
- go-binddata can not found when GOPATH is a list of directories
  (li.guangxu@zte.com.cn)
- refactor function parameters (li.guangxu@zte.com.cn)
- Addressing the issues identified in BZ 1341312 (erich@redhat.com)
- diagnostics: fix bug 1359771 (lmeyer@redhat.com)
- make db user/password parameter description clearer (bparees@redhat.com)
- Use default scrub pod template for NFS recycler (mawong@redhat.com)
- move cmd tools to common build cmd package (bparees@redhat.com)
- fix some bugs with ROUTER_SLOW_LORIS_TIMEOUT (jtanenba@redhat.com)

* Wed Aug 10 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.18
- Merge remote-tracking branch upstream/master, bump origin-web-console 4a118e0
  (tdawson@redhat.com)
- Fix test compile error (jliggitt@redhat.com)
- Add scope representing full user permissions, default token requests with
  unspecified scope (jliggitt@redhat.com)
- make build API validation compile (deads@redhat.com)
- Fix openshift/origin-release:golang-1.4 image (agoldste@redhat.com)
- Clean up test etcd data in test loops (jliggitt@redhat.com)
- UPSTREAM: 29212: hpa: ignore scale targets whose replica count is 0
  (sross@redhat.com)
- Make HAProxy Router Aware of Idled Services (sross@redhat.com)
- Introduce the Unidler Socket (sross@redhat.com)
- Add the `oc idle` command (sross@redhat.com)
- Introduce Unidling Controller (sross@redhat.com)
- Introduce the Hybrid Proxy (sross@redhat.com)
- Changed the userspace proxy to remove conntrack entries (bbennett@redhat.com)
- Fork Kubernetes Userspace Proxy (sross@redhat.com)
- Recognize gzipped empty layer when marking parents in oadm top images
  (maszulik@redhat.com)
- fix typo in deployment (wang.yuexiao@zte.com.cn)
- Add link for details on controlling Docker options with systemd
  (vichoudh@redhat.com)
- Support network ingress on arbitrary ports (marun@redhat.com)
- Use a consistent process for the official release (ccoleman@redhat.com)
- Add MongoDB connection URL to template message (spadgett@redhat.com)
- The docker-builder url is not found (yu.peng36@zte.com.cn)
- fix rpm spec file to properly build man pages dynamically
  (tdawson@redhat.com)
- Make import image more efficient (miminar@redhat.com)
- Remove allocated IPs from app-scenarios test data (jliggitt@redhat.com)
- Isolate graph test data (jliggitt@redhat.com)
- Bug 1281735 - remove the internal docker registry information from
  imagestream (maszulik@redhat.com)
- Add more info for test case (zhao.xiangpeng@zte.com.cn)
- change "timeout server" to "timeout tunnel" for pass through routes
  (jtanenba@redhat.com)
- Drop the SDN endpoint filter pod watch (danw@redhat.com)
- support for defaulting s2i incremental field at the cluster level
  (bparees@redhat.com)
- display build config dependencies more explicitly in oc status; also note
  input images have scheduled imports (gmontero@redhat.com)

* Mon Aug 08 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.17
- Bump origin-web-console (5ae75bb) (jforrest@redhat.com)
- update help and option suggestions on cmds acting as root cmd
  (jvallejo@redhat.com)
- UPSTREAM: google/cadvisor: 1359: Make ThinPoolWatcher loglevel consistent
  (agoldste@redhat.com)
- UPSTREAM: 29961: Allow PVs to specify supplemental GIDs (agoldste@redhat.com)
- UPSTREAM: 29576: Retry assigning CIDRs in case of failure
  (agoldste@redhat.com)
- UPSTREAM: 29246: Kubelet: Set PruneChildren when removing image
  (agoldste@redhat.com)
- UPSTREAM: 29063: Automated cherry pick of #28604 #29062 (agoldste@redhat.com)
- Update RunNodeController to match upstream change (agoldste@redhat.com)
- UPSTREAM: 28886: Add ForgetPod to SchedulerCache (agoldste@redhat.com)
- UPSTREAM: 28294: kubectl: don't display empty list when trying to get a
  single resource that isn't found (agoldste@redhat.com)
- vendor console script says what commit its vendoring in the output
  (jforrest@redhat.com)
- switch postgres template to latest (9.5) version (bparees@redhat.com)
- new-app: display warning if git not installed (cewong@redhat.com)
- Create petset last in testdata (jliggitt@redhat.com)
- Added unit tests for repository and blobdescriptorservice
  (miminar@redhat.com)
- Allow to mock default registry client (miminar@redhat.com)
- e2e: added tests for cross-repo mounting (miminar@redhat.com)
- e2e: speed-up docker repository pull tests (miminar@redhat.com)
- Configurable blobrepositorycachettl value (miminar@redhat.com)
- Cache blob <-> repository entries in registry with TTL (miminar@redhat.com)
- Fixes docker call in CONTRIBUTING documentation (rymurphy@redhat.com)
- Check for blob existence before serving (miminar@redhat.com)
- Store media type in image (miminar@redhat.com)
- UPSTREAM: docker/distribution: 1857: Provide stat descriptor for Create
  method during cross-repo mount (jliggitt@redhat.com)
- UPSTREAM: docker/distribution: 1757: Export storage.CreateOptions in top-
  level package (miminar@redhat.com)
- Preventing build from inheriting master log level (jupierce@redhat.com)
- Update README URLs based on HTTP redirects (frankensteinbot@gmail.com)
- update the output of quota example (li.guangxu@zte.com.cn)

* Fri Aug 05 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.16
- Buildrequires rsync - due to hack scripts (tdawson@redhat.com)

* Fri Aug 05 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.15
- Use hack/update-generated-docs.sh to update man pages in origin.spec
  (tdawson@redhat.com)
- Set OS_GIT_VERSION in spec file (tdawson@redhat.com)
- UPSTREAM: google/cadvisor: 1411: Ensure minimum kernel version for thin_ls
  (agoldste@redhat.com)
- Remove unsupported DNS calls from tests (ccoleman@redhat.com)
- UPSTREAM: 29655: No PetSet client (ccoleman@redhat.com)
- Show PetSets in oc status (ccoleman@redhat.com)
- DNS should support hostname annotations (ccoleman@redhat.com)
- generate: debug command (ccoleman@redhat.com)
- Debug should be able to skip init containers (ccoleman@redhat.com)
- PetSet examples (ccoleman@redhat.com)
- Enable petsets (ccoleman@redhat.com)
- Bump origin-web-console (ef9f1a1) (jforrest@redhat.com)
- Refactored the dependency between images and images-old-policy
  (skuznets@redhat.com)
- fix s2i config validation (ipalade@redhat.com)
- regen docs and completions (sjenning@redhat.com)
- follow reference values in 'oc env --list' (sjenning@redhat.com)
- Update ose_iamges.sh script to work with 3.3 (tdawson@redhat.com)
- UPSTREAM: 27392: allow watching old resources with kubectl
  (sjenning@redhat.com)
- Dockerfile builder moved to github.com/openshift/imagebuilder
  (ccoleman@redhat.com)
- bump(github.com/openshift/imagebuilder):5a8e7d9be33db899875d7c9effb8c60276188
  67a (ccoleman@redhat.com)
- Switch to depend on docker imagebuilder (ccoleman@redhat.com)
- Copy more kube artifacts (ccoleman@redhat.com)
- bump(kubernetes):507d3a7b242634b131710cfdfd55e3a1531ffb1b
  (ccoleman@redhat.com)
- add istag create (deads@redhat.com)
- Support image forcePull policy for runtime image when do extended build
  (haowang@redhat.com)
- UPSTREAM: 30021: add asserts for RecognizingDecoder and update protobuf
  serializer to implement interface (pweil@redhat.com)
- Print warning next to deployment with restarting pods (mfojtik@redhat.com)
- Fix panic caused by invalid ip range (zhao.xiangpeng@zte.com.cn)
- create app label based on template name, not buildconfig name
  (bparees@redhat.com)
- Added wait for project cache sync before test-cmd buckets start
  (skuznets@redhat.com)
- cluster up: use shared volumes in mac and windows (cewong@redhat.com)
- Avoid test flakes when creating new projects (mkhan@redhat.com)
- UPSTREAM: 29847: Race condition in scheduler predicates test
  (ccoleman@redhat.com)
- update config for unauth pull (aweiteka@redhat.com)
- Bump origin-web-console (781f34f) (jforrest@redhat.com)
- check for correct exiterr type on oc exec failures (bparees@redhat.com)
- UPSTREAM: 29182: Use library code for scheduler predicates test
  (ccoleman@redhat.com)
- Upstream packages for Go 1.4 have been dropped (ccoleman@redhat.com)
- switch to generated password for jenkins service (bparees@redhat.com)
- Always output existing credentials message when reusing credentials on login
  (jliggitt@redhat.com)
- Fix debugging pods with multiple containers (ffranz@redhat.com)
- UPSTREAM: 29952: handle container terminated but pod still running in
  conditions (ffranz@redhat.com)
- properly check for running app pod (bparees@redhat.com)
- handle forbidden server errs on older server version (jvallejo@redhat.com)
- Reorganize directories, test and Makefile (aweiteka@redhat.com)
- add link to Manageing Security Context Contraints (li.guangxu@zte.com.cn)
- Fix BZ1361024 (jtanenba@redhat.com)
- Generate man pages and bash completion during rpm build (tdawson@redhat.com)
- [RPMS] Require device-mapper-persistent-data >= 0.6.2 (sdodson@redhat.com)
- UPSTREAM: 28539: Fix httpclient setup for gcp (decarr@redhat.com)
- UPSTREAM: 28871: Do not use metadata server to know if on GCE
  (decarr@redhat.com)
- Removed namespace assumptions from test-cmd test cases (skuznets@redhat.com)
- deploy: deep-copy only when mutating in the controllers (mkargaki@redhat.com)
- deploy: remove redundant test deployment checks (mkargaki@redhat.com)
- improve checking if systemd needs to be reexec'd (sdodson@redhat.com)
- regen swagger spec (sjenning@redhat.com)
- UPSTREAM: 28263: Allow specifying secret data using strings
  (sjenning@redhat.com)
- [RPMS] Enable CPU and Memory during node installation (sdodson@redhat.com)

* Wed Aug 03 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.14
- Fix doc link (wang.yuexiao@zte.com.cn)
- Bump origin-web-console (b0dc7d5) (jforrest@redhat.com)
- Adds a flag to allow skipping config writes when creating a project
  (ffranz@redhat.com)
- add back missing integration tests (deads@redhat.com)
- clusterquota: prove image stream work with cluster quota (mfojtik@redhat.com)
- add back missing integration tests (deads@redhat.com)
- cluster up: suggest 'cluster down' message (cewong@redhat.com)
- add extended builds functionality (ipalade@redhat.com)
- Display original cmd and stderr in extended test (mfojtik@redhat.com)
- Improve bootstrap policy overwrite error handling (jliggitt@redhat.com)
- Aggregate reconcile errors (jliggitt@redhat.com)
- Make AddRole tolerate races on role additions (jliggitt@redhat.com)
- Improve role/rolebinding virtual storage (jliggitt@redhat.com)
- Fix panic in rolebinding reconcile error message (jliggitt@redhat.com)
- generated code (pweil@redhat.com)
- SCC seccomp admission (pweil@redhat.com)
- UPSTREAM: <carry>: SCC seccomp support (pweil@redhat.com)
- using a unused parameter (Yanqiang Miao)
- Add paused field in deployment config describer (mfojtik@redhat.com)
- mv oc projects tests to separate file (jvallejo@redhat.com)
- Bump origin-web-console (d68bf15) (jforrest@redhat.com)
- new-app: accept template on stdin (cewong@redhat.com)
- UPSTREAM: 29588: Properly apply quota for init containers
  (ccoleman@redhat.com)
- oc rsync: allow multiple include or exclude patterns (cewong@redhat.com)
- Change package names for upstream volume controllers in master.go
  (pmorie@gmail.com)
- UPSTREAM: 29673: Fix mount collision timeout issue (pmorie@gmail.com)
- UPSTREAM: 29641: Fix wrapped volume race (pmorie@gmail.com)
- UPSTREAM: 28939: Allow mounts to run in parallel for non-attachable volumes
  (pmorie@gmail.com)
- UPSTREAM: 28409: Reorganize volume controllers and manager (pmorie@gmail.com)
- UPSTREAM: 28153: Fixed goroutinemap race on Wait() (pmorie@gmail.com)
- UPSTREAM: 24797: add enhanced volume and mount logging for block devices
  (pmorie@gmail.com)
- UPSTREAM: 28584: Add spec.Name to the configmap GetVolumeName
  (pmorie@gmail.com)
- remove accidentally checked in openshift.local.config (bparees@redhat.com)
- UPSTREAM: 29171: Fix order of determineContainerIP args (pmorie@gmail.com)
- handle nil reference to o.Attach.Err (jvallejo@redhat.com)
- UPSTREAM: 29485: Assume volume detached if node doesn't exist
  (pmorie@gmail.com)
- UPSTREAM: 24385: golint fixes for aws cloudprovider (pmorie@gmail.com)
- UPSTREAM: 29240: Pass nodeName to VolumeManager instead of hostName
  (pmorie@gmail.com)
- UPSTREAM: 29031: Wait for PD detach on PD E2E to prevent kernel err
  (pmorie@gmail.com)
- UPSTREAM: 28404: Move ungraceful PD tests out of flaky (pmorie@gmail.com)
- UPSTREAM: 28181: Add two pd tests with default grace period
  (pmorie@gmail.com)
- UPSTREAM: 28090: Mark "RW PD, remove it, then schedule" test flaky
  (pmorie@gmail.com)
- UPSTREAM: 28048: disable flaky PD test (pmorie@gmail.com)
- UPSTREAM: 29077: Fix "PVC Volume not detached if pod deleted via namespace
  deletion" issue (pmorie@gmail.com)
- enable deployments (deads@redhat.com)
- cluster up: fix incorrect directory permissions (cewong@redhat.com)
- Handle local source as binary when git is not available (cewong@redhat.com)
- enable replicasets (deads@redhat.com)
- Move image administrative commands tests to admin.sh (maszulik@redhat.com)
- oadm top command generated changes (maszulik@redhat.com)
- Add oadm top command for analyzing image and imagestream usage
  (maszulik@redhat.com)
- remove unnecessary export (deads@redhat.com)
- Bug 1325069 - Sort status tags according to semver (maszulik@redhat.com)
- Support scheduler config options so custom schedulers can be used
  (ccoleman@redhat.com)
- Check pull access when tagging imagestreams (jliggitt@redhat.com)
- to remove singleton and to fix
  TestSimpleImageChangeBuildTriggerFromImageStreamTagSTI (salvatore-
  dario.minonne@amadeus.com)
- update the URL of git server (li.guangxu@zte.com.cn)
- Kerberos Extended Test Improvements (mkhan@redhat.com)
- update hardcoded "oc" cmd suggestion in cmd output (jvallejo@redhat.com)
- check that dest exists before attempting to extract (jvallejo@redhat.com)
- Support EgressNetworkPolicy in SDN plugin (danw@redhat.com)
- Make SDN plugin track vnid->Namespaces mapping (in addition to the reverse)
  (danw@redhat.com)
- Update generated code (danw@redhat.com)
- Add EgressNetworkPolicy (danw@redhat.com)
- Always set pod name annotation on build (cewong@redhat.com)
- SCC admission with sharedIndexInformer (salvatore-dario.minonne@amadeus.com)
- use legacy restmapper against undiscoverable servers (deads@redhat.com)
- fix bz1355721 (bmeng@redhat.com)
- `env_file` option of docker-compose support (surajssd009005@gmail.com)
- testing (bparees@redhat.com)
- Changing defaultExporter to be public (DefaultExporter) (mdame@redhat.com)
- Don't use "kexec" as both a package name and a variable name
  (danw@redhat.com)
- Use randomness for REGISTRY_HTTP_SECRET in projectatomic/atomic-registry-
  install (schmidt.simon@gmail.com)

* Mon Aug 01 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.13
- rsh: Insert a TERM variable into the command to be run
  (jonh.wendell@redhat.com)
- e2e test: remove PodCheckDns flake (lmeyer@redhat.com)
- oc secrets: rename `add` to `link` and add `unlink` (sgallagh@redhat.com)
- Return nil directly instead of err when err is nil
  (zhao.xiangpeng@zte.com.cn)
- Extended tests for kerberos (mkhan@redhat.com)
- UPSTREAM: 29134: Improve quota controller performance (decarr@redhat.com)

* Fri Jul 29 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.12
- Avoid to create an API client if not needed (ffranz@redhat.com)
- bump(github.com/openshift/source-to-image):
  724c0ddaacec2fad31a05ac2d2175cd9ad7136c6 (bparees@redhat.com)
- UPSTREAM: 29457: Quota was not counting services with multiple nodeports
  properly (agoldste@redhat.com)
- Improved Bash idioms in test/cmd/images.sh (skuznets@redhat.com)
- Improved Bash idioms in test/cmd/builds.sh (skuznets@redhat.com)
- Improved Bash idioms in test/cmd/basicresources.sh (skuznets@redhat.com)
- Ensured that all indentation in hack/lib/cmd.sh used tabs
  (skuznets@redhat.com)
- Use anonyous FIFO instead of pipe in os::cmd output test
  (skuznets@redhat.com)
- Handled working dir changes in Bash relative path util (skuznets@redhat.com)
- Colocated config validation with other diagnostics tests
  (skuznets@redhat.com)
- example: add pulling origin-pod (li.guangxu@zte.com.cn)
- Update generated files (stefan.schimanski@gmail.com)
- UPSTREAM: 28351: Add support for kubectl create quota command
  (stefan.schimanski@gmail.com)
- generated: cmd changes (ccoleman@redhat.com)
- Add a migration framework for mutable resources (ccoleman@redhat.com)
- make oc login kubeconfig permission error clearer (jvallejo@redhat.com)
- Update branding of templates to use OpenShift Platform Container
  (sgoodwin@redhat.com)
- update oc version to display client and server versions (jvallejo@redhat.com)
- Set default image version for 'oc cluster up' (cewong@redhat.com)
- make clusterquota/status endpoint (deads@redhat.com)
- update PSP review APIS (deads@redhat.com)
- handle authorization evaluation error for SAR (deads@redhat.com)
- BuildSource: mark secrets field as omitempty. (vsemushi@redhat.com)
- Moved completions tests from hack/test-cmd.sh into test/cmd/completions.sh
  (skuznets@redhat.com)
- Removed spam from hack/test-cmd.sh output on test success
  (skuznets@redhat.com)
- update the output of Deploying a private docker registry
  (li.guangxu@zte.com.cn)
- document NetworkPolicy status (danw@redhat.com)
- UPSTREAM: 29291: Cherry-picked (jimmidyson@gmail.com)
- Description of (run the unit tests) is incorrect in README.md
  (li.guangxu@zte.com.cn)
- dockerbuild pull on Docker 1.9 is broken (ccoleman@redhat.com)
- Add Dockerfiles.centos7 and job-id files for CentOS image building
  (tdawson@redhat.com)
- Add V(5) logging back to pkg/util/ovs and pkg/util/ipcmd (danw@redhat.com)
- update FAQ in README.md (li.guangxu@zte.com.cn)
- dind: enable overlay storage (marun@redhat.com)
- dind: remove wrapper script (marun@redhat.com)
- Make /var/lib/origin mounted rslave for containerized systemd units
  (sdodson@redhat.com)

* Wed Jul 27 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.11
- README.md: Updates to "What can I run on Origin?" section (jawnsy@redhat.com)
- Return instead of exit on argument mismatch in os::cmd (skuznets@redhat.com)
- Allows filtering templates by namespace in new-app (ffranz@redhat.com)
- tolerate yaml payloads for generic webhooks (bparees@redhat.com)
- Use client interface for DCReaper (matthieu.dalstein@amadeus.com)
- Clarify major Origin features (jawnsy@redhat.com)
- bump(github.com/openshift/source-to-image):
  3632461ab64707aad489203b99889650cdf02647 (jupierce@redhat.com)
- Update Origin admission controllers to handle init containers
  (ccoleman@redhat.com)
- UPSTREAM: 29356: Container limits are not applied to InitContainers
  (ccoleman@redhat.com)
- UPSTREAM: 29356: InitContainers are not checked for hostPort ranges
  (ccoleman@redhat.com)
- InitContainer admission (ccoleman@redhat.com)
- update docs (jvallejo@redhat.com)
- make cli command root dynamic in cmd output (jvallejo@redhat.com)
- Ensure build pod name annotation is set (cewong@redhat.com)
- Secrets: work around race in `oc extract` test (sgallagh@redhat.com)
- Fix test registry client with images of different versions
  (agladkov@redhat.com)
- Parse v2 schema manifest (agladkov@redhat.com)
- add master-config validation that complains about using troublesome admission
  chains (deads@redhat.com)
- Use image.DockerImageLayers for pruning (maszulik@redhat.com)
- implement basic DDOS protections in the HAProxy template router
  (jtanenba@redhat.com)
- generated: For proto and authorization changes (ccoleman@redhat.com)
- Enable protobuf for new configurations, json for old (ccoleman@redhat.com)
- bump(github.com/openshift/source-to-image):
  49dac82c5f1521e95980c71ff0e092b991fc1b09 (bparees@redhat.com)
- Make authorization conversions efficient (ccoleman@redhat.com)
- Rename AuthorizationAttributes -> Action (ccoleman@redhat.com)
- Fix issues running protobuf from clients (ccoleman@redhat.com)
- Verify protobuf during runs (ccoleman@redhat.com)
- Templates should use NestedObject* with Unstructured scheme
  (ccoleman@redhat.com)
- UPSTREAM: 28931: genconversion=false should skip fields during conversion
  generation (ccoleman@redhat.com)
- UPSTREAM: 28934: Unable to have optional message slice (ccoleman@redhat.com)
- UPSTREAM: <drop>: Drop LegacyCodec in unversioned client
  (ccoleman@redhat.com)
- UPSTREAM: 28932: Fail correctly in go-to-protobuf (ccoleman@redhat.com)
- UPSTREAM: 28933: Handle server errors more precisely (ccoleman@redhat.com)
- extended: set failure traps in all deployment tests (mkargaki@redhat.com)
- Adding template instructional message to new-app output (jupierce@redhat.com)
- Override port forwarding if using docker machine (cewong@redhat.com)
- Bump origin-web-console (50d02f7) (jforrest@redhat.com)
- support -f flag for --follow in start-build (bparees@redhat.com)
- Enforce --tty=false flag for 'oc debug' (mdame@localhost.localdomain)

* Mon Jul 25 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.10
- Skip the '-' character in scratch builds for Docker 1.9.1
  (ccoleman@redhat.com)
- Added REST storage for imagesignatures resource (miminar@redhat.com)
- List enabled/disabled features in readme. (ccoleman@redhat.com)
- Match upstream exec API refactoring (agoldste@redhat.com)
- UPSTREAM: 29237: Fix Windows terminal handling (agoldste@redhat.com)
- Example description incorrect (li.guangxu@zte.com.cn)
- add oc annotate --single-resource flag tests (jvallejo@redhat.com)
- UPSTREAM: 29319: update rsrc amnt check to allow use of --resource-version
  flag with single rsrc (jvallejo@redhat.com)
- deploy: validate minReadySeconds against default deployment timeout
  (mkargaki@redhat.com)
- make node auth use tokenreview API (deads@redhat.com)
- UPSTREAM: 28788: add tokenreviews endpoint to implement webhook
  (deads@redhat.com)
- UPSTREAM: 28852: authorize based on user.Info (deads@redhat.com)
- deploy: stop defaulting to ImageStreamTag unconditionally
  (mkargaki@redhat.com)
- deploy: enqueue configs on pod events (mkargaki@redhat.com)
- validate imagestreamtag names in build definitions (bparees@redhat.com)
- dump etcd on test integration failures (deads@redhat.com)
- oc: enable following deployment logs in deploy (mkargaki@redhat.com)
- deploy: update the log registry to poll on notfound deployments
  (mkargaki@redhat.com)
- UPSTREAM: 28964: Reexport term.IsTerminal (agoldste@redhat.com)
- UPSTREAM: <carry>: support pointing oc portforward to old openshift server
  (agoldste@redhat.com)
- UPSTREAM: <carry>: support pointing oc exec to old openshift server
  (agoldste@redhat.com)
- UPSTREAM: 25273: Support terminal resizing for exec/attach/run
  (agoldste@redhat.com)
- UPSTREAM: <carry>: REVERT support pointing oc exec to old openshift server
  (agoldste@redhat.com)
- Implement an `oc extract` command to make it easy to use secrets
  (ccoleman@redhat.com)

* Fri Jul 22 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.9
- Clean up Origin and OSE differences (tdawson@redhat.com)
- Abort deployment when the from version is higher than to version
  (mfojtik@redhat.com)
- run all parallel builds when a serial build finishes (bparees@redhat.com)
- Image progress: recognize additional already pushed status
  (cewong@redhat.com)
- Accept OS_BUILD_IMAGE_ARGS in our image build scripts (ccoleman@redhat.com)
- clean up jenkins master/slave example (bparees@redhat.com)
- add mariadb template and use specific imagestream tag versions, not latest
  (bparees@redhat.com)
- bump(github.com/openshift/source-to-image):
  9350cd1ef8afc9c91dcdc96e1d972cbcc6f36181 (vsemushi@redhat.com)
- Add an experimental --mount flag to oc ex dockerbuild (ccoleman@redhat.com)
- Inherit all our images from our base (ccoleman@redhat.com)
- add default resource requests to registry creation (agladkov@redhat.com)
- Pause the deployment config before deleting (mfojtik@redhat.com)
- Debug should not have 15s timeout (ccoleman@redhat.com)
- Start rsync daemon in the foreground to prevent zombie processes
  (cewong@redhat.com)
- Bump origin-web-console (bad6254) (jforrest@redhat.com)
- Enabling multiline parameter injection for templates (jupierce@redhat.com)
- allow for configurable server side timeouts on routes (jtanenba@redhat.com)
- UPSTREAM: 29133: use a separate queue for initial quota calculation
  (deads@redhat.com)
- Cherry-pick scripts create a .make directory that we should ignore
  (decarr@redhat.com)
- Fix ClusterResourceOverride test that has been broken for a while
  (ccoleman@redhat.com)
- Bump size of file allowed to be uploaded (ccoleman@redhat.com)
- Test must set RequireEtcd (ccoleman@redhat.com)
- Strip build tags from integration, they are not needed (ccoleman@redhat.com)
- update generated docs (jvallejo@redhat.com)
- UPSTREAM: 0000: update timeout flag help to contain correct duration format
  (jvallejo@redhat.com)
- Avoid allocations while checking role bindings (ccoleman@redhat.com)
- Fixed a bug in jUnit output from os::cmd::try_until_text
  (skuznets@redhat.com)
- use pod name instead of id in oc delete example (jvallejo@redhat.com)
- deploy: add source for hook events (mkargaki@redhat.com)
- Added checks in build path for docker-compose import
  (surajssd009005@gmail.com)

* Wed Jul 20 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.8
- Add support for anonymous registry requests (jliggitt@redhat.com)
- Allow anonymous users to check their access (jliggitt@redhat.com)
- gather dump before you kill the process (deads@redhat.com)
- Bug 1357668: update error message on invalid tags in ICTs
  (mkargaki@redhat.com)
- Add goimports to the release image (ccoleman@redhat.com)
- generate: Protobuf types and updates (ccoleman@redhat.com)
- Handle content type correctly in extensions (ccoleman@redhat.com)
- Generator for protobuf (ccoleman@redhat.com)
- Give build fields proto safe names (ccoleman@redhat.com)
- UPSTREAM: 28935: don't double encode runtime.Unknown (ccoleman@redhat.com)
- UPSTREAM: 26044: Additional fixes to protobuf versioning
  (ccoleman@redhat.com)
- UPSTREAM: 28810: Honor protobuf name tag (ccoleman@redhat.com)
- bump(k8s.io/kubernetes/third_party/protobuf):v1.3.0 (ccoleman@redhat.com)
- Enable protoc in the release images (ccoleman@redhat.com)
- Bump origin-web-console (15dc649) (jforrest@redhat.com)
- Copy GSSAPI errors to prevent use-after-free bugs (mkhan@redhat.com)
- oc new-app: add missing single quote. (vsemushi@redhat.com)
- UPSTREAM: 27263: Cherry-picked (stefan.schimanski@gmail.com)
- Remove deployment trigger warning (jhadvig@redhat.com)
- deploy: add minReadySeconds for deploymentconfigs (mkargaki@redhat.com)
- deploy: generated code for minReadySeconds (mkargaki@redhat.com)
- UPSTREAM: 28111: Add MinReadySeconds to rolling updater (mkargaki@redhat.com)
- UPSTREAM: 28966: Describe container volume mounts (mfojtik@redhat.com)
- oc: add --insecure-policy for creating edge routes (mkargaki@redhat.com)
- Bug 1356530: handle 403 in oc rollback (mkargaki@redhat.com)
- Allow Docker for Mac beta to work by using port forwarding
  (cewong@redhat.com)
- expose evaluation errors for RAR (deads@redhat.com)
- allow git_ssl_no_verify env variable in build pods (bparees@redhat.com)
- deploy: move cli-related packages in cmd (mkargaki@redhat.com)

* Mon Jul 18 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.7
- deploy: update revisionHistoryLimit to int32 (mkargaki@redhat.com)
- Added a check for empty template in DeployConfig (rymurphy@redhat.com)
- restore resyncing ability for fifo based controllers (deads@redhat.com)
- UPSTREAM: <carry>: fix fifo resync, remove after FIFO is dead
  (deads@redhat.com)
- Dump deployment logs in case of test failure (nagy.martin@gmail.com)
- Test gssapi library load when selecting available challenge handlers
  (jliggitt@redhat.com)

* Fri Jul 15 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.6
- Cleanup OSE directories no longer in origin, or used (tdawson@redhat.com)
- Add jobs to conformance run (maszulik@redhat.com)
- deploy: set gracePeriod on deployer creation rather than on deletion
  (mkargaki@redhat.com)
- UPSTREAM: 28966: Fix watch cache filtering (jliggitt@redhat.com)
- Fully specify DeleteHostSubnet() rules (danw@redhat.com)
- Ensure the test deployment invariant is maintained (ccoleman@redhat.com)
- Bump origin-web-console (38099da) (jforrest@redhat.com)
- Added debugging information to os::cmd output content test
  (skuznets@redhat.com)
- Fix SA OAuth test flake (jliggitt@redhat.com)
- Load versioned gssapi libs (jliggitt@redhat.com)
- Re-enable new-app integration tests (cewong@redhat.com)
- treat notfound and badrequest instantiate errors as fatal
  (bparees@redhat.com)
- docs: remove openshift_model.md (lmeyer@redhat.com)
- add nodejs 4 imagestream and bump templates to use latest imagestreams
  (bparees@redhat.com)
- Update CONTRIBUTING.adoc (jminter@redhat.com)
- Enable PersistentVolumeLabel admission plugin (agoldste@redhat.com)
- add etcd dump for integration test (deads@redhat.com)
- UPSTREAM: 28626: update resource builder error message to be more clear
  (jvallejo@redhat.com)
- display resource type as part of its name (jvallejo@redhat.com)
- UPSTREAM: 28509: Update HumanResourcePrinter signature w single PrintOptions
  param (jvallejo@redhat.com)
- extended: update deployment test timeout (mkargaki@redhat.com)
- deploy: set gracePeriodSeconds on deployer deletion (mkargaki@redhat.com)
- integration: add multiple ict deployment test (mkargaki@redhat.com)
- Added a clean-up policy for deployments (skuznets@redhat.com)
- add project annotation selectors to cluster quota (deads@redhat.com)
- Refactor stable layer count unit test (cewong@redhat.com)
- Add test for image import and conversion v2 schema to v1 schema
  (agladkov@redhat.com)
- UPSTREAM: 27379: display resouce type as part of resource name
  (jvallejo@redhat.com)
- Added tests for pod-network CLI command (rpenta@redhat.com)
- Some reorg/cleanup for enabling pod-network cli tests (rpenta@redhat.com)
- use maven image for slave pods in sample pipeline (bparees@redhat.com)
- Revert "Allow size of image to be zero when schema1 from Hub"
  (legion@altlinux.org)
- Fix image size calculation in importer (agladkov@redhat.com)

* Wed Jul 13 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.5
- Add an admission controller to block illegal service endpoints
  (danw@redhat.com)
- Limit the amount of output of a faulty new-build (rymurphy@redhat.com)
- Parameter validation for new-app strategy (jupierce@redhat.com)
- Add option to handle OAuth grants at a per-client granularity
  (sgallagh@redhat.com)
- Add mirror env var options and fix required parameters to templates
  (vdinh@redhat.com)
- Add a service account for the endpoints controller (danw@redhat.com)
- Remove unused code (li.guangxu@zte.com.cn)
- no need to build networking pkg in build-tests session
  (li.guangxu@zte.com.cn)
- SDN types should used fixed width integers (ccoleman@redhat.com)
- reject build requests for binary builds if not providing binary inputs
  (bparees@redhat.com)
- dind: update to f24 (and go 1.6) (marun@redhat.com)
- Bump origin-web-console (563e73a) (jforrest@redhat.com)
- return notfound errors w/o wrappering them (bparees@redhat.com)
- No info about deployer pods logged (ccoleman@redhat.com)
- Update godep-restore (ccoleman@redhat.com)
- Adding a field to templates to allow them to deliver a user message with
  parameters substituted (jupierce@redhat.com)
- UPSTREAM: 28500: don't migrate files you can't access (deads@redhat.com)
- Update comment (rhcarvalho@gmail.com)
- Refactored test/cmd/authentication.sh to use proper strings and literals
  (skuznets@redhat.com)

* Mon Jul 11 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.4
- OSE Fix: Revert OS_GIT_VERSION regex back (tdawson@redhat.com)
- Fix a DNS flake by watching for pod succeeded (ccoleman@redhat.com)
- Avoid mutating the cached deployment config in image change controller
  (ccoleman@redhat.com)
- combine quota registries (deads@redhat.com)
- UPSTREAM: 28611: add union registry for quota (deads@redhat.com)
- UPSTREAM: <carry>: make namespace lifecycle access review aware
  (deads@redhat.com)
- collapse admission chains (deads@redhat.com)
- lock clusterquota admission to avoid conflicts (deads@redhat.com)
- add clusterquota admission (deads@redhat.com)
- UPSTREAM: 28504: shared informer cleanup (deads@redhat.com)
- Remove all checkErr() flags and retry conflict on BuildController
  (ccoleman@redhat.com)
- test-local-registry.sh works on *nix as well (miminar@redhat.com)
- deploy: pin retries to a const and forget correctly in the dc loop
  (mkargaki@redhat.com)
- deploy: remove deployer pod controller (mkargaki@redhat.com)
- deploy: collapse deployer into deployments controller (mkargaki@redhat.com)
- Add gssapi build flags (jliggitt@redhat.com)
- Limit gssapi auth to specified principal (jliggitt@redhat.com)
- Add gssapi negotiate support (jliggitt@redhat.com)
- bump(github.com/apcera/gssapi): b28cfdd5220f7ebe15d8372ac81a7f41cc35ab32
  (jliggitt@redhat.com)
- Use recycler service account for the recycler pod (ccoleman@redhat.com)
- modify example in Makefile (li.guangxu@zte.com.cn)
- Failing to update deployment config should requeue on image change
  (ccoleman@redhat.com)
- Print more info in router tests (ccoleman@redhat.com)
- generated: API, completions, man pages, conversion, copy
  (ccoleman@redhat.com)
- Deployment test does not round trip (ccoleman@redhat.com)
- Disable extended tests that won't work on OpenShift (ccoleman@redhat.com)
- Update master startup and admission to reflect changes in upstream
  (ccoleman@redhat.com)
- Alter generation to properly reuse upstream types (ccoleman@redhat.com)
- Make conversion and encoding handle nested runtime.Objects
  (ccoleman@redhat.com)
- Refactors from upstream Kube changes (ccoleman@redhat.com)
- Revert "react to 27341: this does not fix races in our code"
  (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Distribution dependencies
  (ccoleman@redhat.com)
- bump(k8s.io/kubernetes):v1.3.0+ (ccoleman@redhat.com)
- bump(*): rename Godeps/_workspace/src -> vendor/ (ccoleman@redhat.com)
- Track upstream versions more precisely (ccoleman@redhat.com)
- Use vendor instead of Godeps/_workspace in builds (ccoleman@redhat.com)
- Update commit checker to handle vendor (ccoleman@redhat.com)
- Remove v1beta3 (ccoleman@redhat.com)
- bump registry xfs quota; re-enable disk usage diag; add docker info
  (gmontero@redhat.com)
- make sure the db has an endpoint before testing the app (bparees@redhat.com)
- Adding dumplogs pattern to extended build tests (jupierce@redhat.com)
- allow cni as one of the plugins (rchopra@redhat.com)
- fix bz1353489 (rchopra@redhat.com)
- Fix incorrect master leases ttl setting (agoldste@redhat.com)
- Fix race in imageprogress test (cewong@redhat.com)

* Fri Jul 08 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.3
- Bump origin-web-console (e29729b) (jforrest@redhat.com)
- Ensure binary is built before completions are updated (jvallejo@redhat.com)
- Added tagged pull e2e tests (maszulik@redhat.com)
- Ensure client can delete a buildconfig with no associated builds, even if
  user lacks build strategy permission (jupierce@redhat.com)
- Fixes oc convert examples (ffranz@redhat.com)
- Remove v1beta3 from end-to-end tests (maszulik@redhat.com)
- cache: return api errors on not found imagestreams/deploymentconfigs
  (mkargaki@redhat.com)
- add clusterquota reconciliation controller (deads@redhat.com)
- Bump origin-web-console (ccd995f) (jforrest@redhat.com)
- Remove global flags from 'openshift start kubernetes *' (ffranz@redhat.com)
- allow roundrobin algorithm as a choice in passthrough/reencrypt
  (rchopra@redhat.com)
- Fixes oc convert examples (ffranz@redhat.com)
- Add krb5-devel to build environments (jliggitt@redhat.com)
-  Backporting dist-git Dockerfile to Dockerfile.product (tdawson@redhat.com)
- Issue 9702 - consider schema v2 layers when pruning (maszulik@redhat.com)
- UPSTREAM: 28353: refactor quota calculation for re-use (deads@redhat.com)
- extended: refactor deployment test to be more debuggable
  (mkargaki@redhat.com)
- track updated manpages (jvallejo@redhat.com)
- update manpage list parsing (jvallejo@redhat.com)
- Cleaned up test/end-to-end/core script (skuznets@redhat.com)
- Add debug logging to ldap search bind errors (jliggitt@redhat.com)
- align s2i and docker push image error messages (bparees@redhat.com)
- Removed environment variable substitution step from stacktrace
  (skuznets@redhat.com)
- Make former openshift-sdn code pass govet (danw@redhat.com)
- Update for import/reorg of openshift-sdn code (danw@redhat.com)
- Drop imported openshift-sdn code (danw@redhat.com)
- handle multiple names for docker registry (deads@redhat.com)
- UPSTREAM: 28379: allow handler to join after the informer has started
  (deads@redhat.com)
- Generated changes for oadm prune images --prune-over-size-limit
  (maszulik@redhat.com)
- Add --prune-over-size-limit flag for pruning images (maszulik@redhat.com)
- Move files to the right places for the origin tree (danw@redhat.com)
- Drop pkg/exec, port pkg/ipcmd and pkg/ovs to kexec (danw@redhat.com)
- Refactor pruning images to follow one common flow with other pruners
  (maszulik@redhat.com)
- Refactor pruning deployment to follow one common flow with other pruners
  (maszulik@redhat.com)
- Refactor pruning builds to follow one common flow with other pruners
  (maszulik@redhat.com)
- haproxy-router delete service endpoint problem (pcameron@redhat.com)
- Remove build files, Godeps, docs, etc (danw@redhat.com)
- Oops, branch got broken by a last-minute change (danw@redhat.com)
- Unexport names that don't need to be exported. (danw@redhat.com)
- Reorganize and split out node API (danw@redhat.com)
- Simplify and split out master API (danw@redhat.com)
- Split OsdnController into OsdnMaster and OsdnNode (danw@redhat.com)
- Add vnidMap type (danw@redhat.com)
- Merge ovsPlugin into OsdnController (danw@redhat.com)
- Merge plugins/osdn/factory into plugins/odsn (danw@redhat.com)
- Merge plugins/osdn/ovs into plugins/osdn (danw@redhat.com)
- Updated `os::cmd' internals to reflect new script location
  (skuznets@redhat.com)
- Migrated `os::text' utilities into `hack/lib/util' (skuznets@redhat.com)
- Migrated `os::cmd' utilities into `hack/lib' (skuznets@redhat.com)
- Adjust types for rebase, add stub Status method (jliggitt@redhat.com)
- we are already production ready (akostadi@redhat.com)
- Use osapi.ClusterNetworkDefault instead of hard coding to 'default'
  (rpenta@redhat.com)
- Added IsOpenShiftNetworkPlugin and IsOpenShiftMultitenantNetworkPlugin
  methods (rpenta@redhat.com)
- Log created/updated ClusterNetwork resource (rpenta@redhat.com)
- Persist network plugin name as part of cluster network creation
  (rpenta@redhat.com)
- Simplify caching and fetching network information (rpenta@redhat.com)
- debug.sh: Fix fetching config file for service (rpenta@redhat.com)
- Force a rebuild after ./hack/sync-to-origin.sh (danw@redhat.com)
- Fix naming typo in sync-to-origin.sh (danw@redhat.com)
- debug: log master config and master api/controllers journal and service
  (dcbw@redhat.com)
- Enable SDN StartNode() to call node iptables setup (rpenta@redhat.com)
- Periodically sync openshift node iptables rules (rpenta@redhat.com)
- Fail more cleanly on script errors (danw@redhat.com)
- Add logging for SDN watch events (rpenta@redhat.com)
- Update subnet when host IP changes instead of delete-old + create-new subnet.
  (rpenta@redhat.com)
- Remove prompts from commented examples (rhcarvalho@gmail.com)
- Add tc state to the debug tar (bbennett@redhat.com)
- Split plugin creation into NewMasterPlugin and NewNodePlugin
  (dcbw@redhat.com)
- Split proxy endpoint filtering out into separate object (dcbw@redhat.com)
- Ignore malformed IP in Node object (danw@redhat.com)
- SDN watch will try to fix transient errors (rpenta@redhat.com)
- Return errors from plugin hooks (rpenta@redhat.com)
- Run Watch resources forever! (rpenta@redhat.com)
- Simplify Watch resource in SDN (rpenta@redhat.com)
- Refer resource names by constants (rpenta@redhat.com)
- Refer SDN plugin names and annotations by constants (rpenta@redhat.com)
- Update for origin kubernetes rebase (dcbw@redhat.com)
- Add more HostSubnet logging (dcbw@redhat.com)
- Synchronize access to vnid map (rpenta@redhat.com)
- Controlled access to vnid map (rpenta@redhat.com)
- Delete NetID from VNID map before deleting NetNamespace object
  (rpenta@redhat.com)
- Fix updating VNID map (rpenta@redhat.com)
- debug.sh: don't do DNS check if node "name" is an IP address
  (danw@redhat.com)
- debug.sh: fix incoming remote test packet traces (danw@redhat.com)
- debug.sh: comment fix (danw@redhat.com)
- debug.sh: check for systemd unit files in /etc too (danw@redhat.com)
- Return better errors from SetUpPod/TearDownPod (danw@redhat.com)
- Fix AddHostSubnetRules() and DeleteHostSubnetRules() (rpenta@redhat.com)
- Removed the cookies from the remote node rules. (bbennett@redhat.com)
- Add a mutex to registry.podsByIP (danw@redhat.com)
- Prepopulate pod info map so that proxy can filter endpoints
  (rpenta@redhat.com)
- Remove unused GetServicesNetwork() and GetHostSubnetLength()
  (rpenta@redhat.com)
- Existing items in the event queue will be reported as Modified event, so
  handle approriately during watching resources(services,namespaces,etc.)
  (rpenta@redhat.com)
- OsdnController.services is only needed in watch services, make it local
  (rpenta@redhat.com)
- Minor cleanup in watchServices() (rpenta@redhat.com)
- Prepopulate VNIDMap for VnidStartMaster/VnidStartNode methods
  (rpenta@redhat.com)
- Do not need to pre-populate nodeAddressMap in WatchNodes (rpenta@redhat.com)
- Don't return resource version for methods GetSubnets, GetNetNamespaces and
  GetServices. No longer needed. (rpenta@redhat.com)
- Remove unused stop channels in sdn (rpenta@redhat.com)
- Remove Get and Watch resource behavior in sdn (rpenta@redhat.com)
- Don't resync/repopulate the event queue for sdn events (rpenta@redhat.com)
- Use NewListWatchFromClient() instead of ListFunc/WatchFunc
  (rpenta@redhat.com)
- Add more logging, to help debug problems (danw@redhat.com)
- Fixes the ip address to namespace cache when pods reuse addresses
  (bbennett@redhat.com)
- Changed the debug script to capture the arp cache (bbennett@redhat.com)
- Added 'docker ps -a' to the node debug items (bbennett@redhat.com)
- Do not throw spurious error msgs in watch network namespaces
  (rpenta@redhat.com)
- Fix watch services for multitenant plugin (rpenta@redhat.com)
- Fix broken VnidStartMaster() (rpenta@redhat.com)
- Fix watch services for multitenant plugin (rpenta@redhat.com)
- Implement "pod.network.openshift.io/assign-macvlan" annotation
  (danw@redhat.com)
- Fix network pod teardown (rpenta@redhat.com)
- Added endpoints to the list of things we add to the debug tar.
  (bbennett@redhat.com)
- Make plugin event-handling methods take objects rather than multiple args
  (danw@redhat.com)
- Move remaining internal-only types from osdn/api to osdn (danw@redhat.com)
- Drop kubernetes/OpenShift wrapper types (danw@redhat.com)
- Merge FlowController into PluginHooks (danw@redhat.com)
- Rename OvsController to OsdnController (danw@redhat.com)
- Fix qos check in del_ovs_flows() (rpenta@redhat.com)
- Improved the debug script to grab the docker unit file. (bbennett@redhat.com)
- Remove unused MTU stuff from bandwidth code (danw@redhat.com)
- Implement better OVS flow rule versioning (dcbw@redhat.com)
- Enforce ingress/egress bandwidth limits (dcbw@redhat.com)
- Validate cluster/service network only when there is a config change
  (rpenta@redhat.com)
- Add pod annotations to osdn Pod API object (dcbw@redhat.com)
- Ensure NodeIP doesn't overlap with the cluster network (dcbw@redhat.com)
- Simplify passing ClusterNetwork through master start functions
  (dcbw@redhat.com)
- Consolidate cluster network validation (dcbw@redhat.com)
- Cache ClusterNetwork details in the Registry (dcbw@redhat.com)
- Fixes to danwinship/arp-fixes (danw@redhat.com)
- Add explicit "actions=drop" OVS rules (danw@redhat.com)
- Add some more OVS anti-spoofing checks (danw@redhat.com)
- Redo OVS rules to avoid ARP caching problems (danw@redhat.com)
- Re-fix reconnect-pods-on-restart code (danw@redhat.com)
- Sync more rebase-related changes back from origin (danw@redhat.com)
- Set the right MTU on tun0 too (dcbw@redhat.com)
- debug.sh: fix typo that messed up our journal output (danw@redhat.com)
- debug.sh: make this work even if renamed (danw@redhat.com)
- Only UpdatePod on startup if the network actually changed (danw@redhat.com)
- Call UpdatePod on all running pods at startup (danw@redhat.com)
- Make "openshift-sdn-ovs update" fix up network connectivity (danw@redhat.com)
- openshift-sdn-ovs: reorganize, deduplicate (danw@redhat.com)
- openshift-sdn-ovs: remove unused Init and Status methods (danw@redhat.com)
- Set MTU on vovsbr/vlinuxbr (danw@redhat.com)
- Resync from origin for kubernetes rebase (danw@redhat.com)
- If NodeIP and NodeName are unsuitable, fallback to default interface IP
  address (dcbw@redhat.com)
- Update to new ListOptions client APIs - missing piece of puzzle
  (maszulik@redhat.com)
- Update to new ListOptions client APIs (jliggitt@redhat.com)
- Minor update to previous commit; use log.Fatalf() (danw@redhat.com)
- Put the network-already-set-up check in the right place (danw@redhat.com)
- Don't set registry.clusterNetwork/.serviceNetwork until needed
  (danw@redhat.com)
- Revert "Fail early on a node if we can't fetch the ClusterNetwork record"
  (danw@redhat.com)
- Fix alreadySetUp() check by using correct address string (dcbw@redhat.com)
- Add setup debug log output (dcbw@redhat.com)
- Fail early on a node if we can't fetch the ClusterNetwork record
  (danw@redhat.com)
- Revert "Don't crash on endpoints with invalid/empty IP addresses"
  (danw@redhat.com)
- Don't crash on endpoints with invalid/empty IP addresses (danw@redhat.com)
- debug.sh: add "sysctl -a" output (danw@redhat.com)
- debug.sh: don't try to run "oc" on the nodes (danw@redhat.com)
- debug.sh: fix up master-as-node case (danw@redhat.com)
- debug.sh: bail out correctly if "run_self_via_ssh --master" fails
  (danw@redhat.com)
- debug.sh: remove some unused code (danw@redhat.com)
- debug.sh: belatedly update for merged OVS rules (danw@redhat.com)
- debug.sh: belatedly update environment filtering for json->yaml change
  (danw@redhat.com)
- fix connectivity from docker containers (me@ibotty.net)
- Add a missing 'ovs-ofctl del-flows' on pod teardown (danw@redhat.com)
- Add a route to fix up service IP usage from the node. (danw@redhat.com)
- Fix a bug with services in multitenant (danw@redhat.com)
- Make pkg/ipcmd and pkg/ovs not panic if their binaries are missing
  (danw@redhat.com)
- Tweak SubnetAllocator behavior when hostBits%%8 != 0 (danw@redhat.com)
- Add some caching to SubnetAllocator (danw@redhat.com)
- Extend TestAllocateReleaseSubnet() to test subnet exhaustion
  (danw@redhat.com)
- Improve subnet_allocator_test debugging (danw@redhat.com)
- trivial: rename SubnetAllocator.capacity to .hostBits (danw@redhat.com)
- Minor: move 'ovsPluginName' to project_options.go, used by join-projects
  /isolate-projects/make-projects-global        rename adminVNID to globalVNID
  (rpenta@redhat.com)
- Admin commands should not depend on OVS code (ccoleman@redhat.com)
- Update kube and multitenant to use pkg/ovs and pkg/ipcmp (danw@redhat.com)
- Move config.env handling to go and rename the file (danw@redhat.com)
- Port setup_required check to go (danw@redhat.com)
- Move docker network configuration into its own setup script (danw@redhat.com)
- Move setup sysctl calls to controller.go (danw@redhat.com)
- Remove locking from openshift-sdn-ovs-setup.sh (danw@redhat.com)
- Add pkg/ipcmd, a wrapper for /sbin/ip (danw@redhat.com)
- Add pkg/ovs, a wrapper for ovs-ofctl and ovs-vsctl (danw@redhat.com)
- Bump node subnet wait to 30 seconds (from 10) (dcbw@redhat.com)
- Add an os.exec wrapper that can be used for test programs (danw@redhat.com)
- Don't redundantly validate the ClusterNetwork (danw@redhat.com)
- Move FilteringEndpointsConfigHandler into openshift-sdn (dcbw@redhat.com)
- Remove unused openshift-sdn-ovs script parameters (dcbw@redhat.com)
- Consolidate pod setup/teardown and update operations (dcbw@redhat.com)
- Log message when pod is waiting for sdn initialization (rpenta@redhat.com)
- Move pod network readiness check to pod setup instead of network plugin init
  (rpenta@redhat.com)
- Filter VXLAN packets from outside the cluster (danw@redhat.com)
- trivial: renumber ovs tables (danw@redhat.com)
- Use the multitenant OVS flows even for the single tenant case
  (danw@redhat.com)
- Properly prioritize all multitenant openflow rules (danw@redhat.com)
- Fix sdn GetHostIPNetworks() and GetNodeIP() (rpenta@redhat.com)
- Make kubelet to block on network plugin initialization until OpenShift is
  done with it's SDN setup. (rpenta@redhat.com)
- Recognize go v1.5 in hack/verify-gofmt.sh (dcbw@redhat.com)
- Merge flatsdn and multitenant plugins (danw@redhat.com)
- Make openshift-ovs-subnet and openshift-ovs-multitenant identical
  (danw@redhat.com)
- Make openshift-sdn-kube-subnet-setup.sh and openshift-sdn-multitenant-
  setup.sh identical (danw@redhat.com)
- Move kube controller base OVS flow setup into setup.sh (danw@redhat.com)
- osdn: Fix out of range panic (mkargaki@redhat.com)
- Trivial build fix (dcbw@redhat.com)
- Don't return an error if the plugin is unknown (dcbw@redhat.com)
- Remove unused/stale files from sdn repo (rpenta@redhat.com)
- Allowing running single tests via "WHAT=pkg/foo ./hack/test.sh"
  (danw@redhat.com)
- Shuffle things around in node start to fix a hang (danw@redhat.com)
- Simplify Origin/openshift-sdn interfaces (dcbw@redhat.com)
- Export oc.Registry for plugins (dcbw@redhat.com)
- Consolidate plugins/osdn and pkg/ovssubnet (dcbw@redhat.com)
- Split subnet-specific startup into separate file (dcbw@redhat.com)
- Split multitenant-specific startup into separate file (dcbw@redhat.com)
- Genericize watchAndGetResource() (dcbw@redhat.com)
- Consolidate hostname lookup (dcbw@redhat.com)
- Trivial multitenant cleanups (dcbw@redhat.com)
- Drop the SubnetRegistry type (danw@redhat.com)
- Move GetNodeIP() to netutils (danw@redhat.com)
- Rename osdn.go to registry.go, OsdnRegistryInterface to Registry
  (danw@redhat.com)
- Remove unused lookup of VNID for TearDownPod (dcbw@redhat.com)
- Fixed comment typo (bbennett@redhat.com)
- Updated with the review comments:  - Fixed stupid errors  - Moved the ssh
  options into the ssh function  - Changed the nsenter commands to make them
  use -- before the subsidiary command  - Changed the output format to yaml
  (bbennett@redhat.com)
- Copy in kubernetes/pkg/util/errors for netutils tests (danw@redhat.com)
- Detect host network conflict with sdn cluster/service network
  (rpenta@redhat.com)
- Made a few small changes:  - Got the 'oc routes' output  - Supressed the host
  check in ssh  - Unified common code between host and node  - Cleaned up the
  formatting to make the commands being run a little more obvious
  (bbennett@redhat.com)
- Tolerate missing routes for lbr0 too (sdodson@redhat.com)
- Isolate restarting docker (sdodson@redhat.com)
- Fix for go-fmt (danw@redhat.com)
- Don't loop forever if subnet not found for the node (rpenta@redhat.com)
- Track Service modifications and update OVS rules (danw@redhat.com)
- Change representation of multi-port services (danw@redhat.com)
- Fix a few log.Error()s that should have been log.Errorf() (danw@redhat.com)
- debug.sh: skip ready=false pods, which won't have podIP set (danw@redhat.com)
- debug.sh: protect a bit against unexpected command output (danw@redhat.com)
- debug.sh: fix a hack (danw@redhat.com)
- Rename pod-network subcommand unisolate-projects to make-projects-global
  (rpenta@redhat.com)
- Allow traffic from VNID 0 pods to all services (danw@redhat.com)
- Tolerate error for tun0 redundant route but not for lbr0 route
  (rpenta@redhat.com)
- Remove support for supervisord-managed docker (marun@redhat.com)
- debug.sh: comment and error message fixes (danw@redhat.com)
- debug.sh: filter out some potentially-sensitive data (danw@redhat.com)
- debug.sh: ignore --net=host containers (danw@redhat.com)
- debug.sh: try to find node config file from 'ps' output (danw@redhat.com)
- debug.sh: include resolv.conf in output (danw@redhat.com)
- debug.sh: explicitly run remote script via bash (danw@redhat.com)
- Validate SDN cluster/service network (rpenta@redhat.com)
- Fix GetClusterNetworkCIDR() (rpenta@redhat.com)
- Minor nit: change valid service IP check to use kapi.IsServiceIPSet
  (rpenta@redhat.com)
- Remove redundant tun0 route (rpenta@redhat.com)
- Fix multitenant flow cleanup again (danw@redhat.com)
- Fix cleanup of service OpenFlow rules (danw@redhat.com)
- Unconditionally use kubernetes iptables pkg to add iptables rules
  (danw@redhat.com)
- Backport changes from origin for kubernetes rebase (danw@redhat.com)
- Clean up stale OVS flows correctly / don't error out of TearDown early
  (danw@redhat.com)
- Fix non-multitenant pod routing (danw@redhat.com)
- Misc debug.sh fixes (danw@redhat.com)
- Detect SDN setup requirement when user switches between flat and multitenant
  plugins (rpenta@redhat.com)
- Minor: pod-network CLI examples to use # for comments (rpenta@redhat.com)
- Track PodIP->Namespace mappings, implement endpoint filtering
  (danw@redhat.com)
- Make plugins take osdn.OsdnRegistryInterface rather than creating it
  themselves (danw@redhat.com)
- Admin command to manage project network (rpenta@redhat.com)
- Fix SDN GetServices() (rpenta@redhat.com)
- Fix SDN status hook (rpenta@redhat.com)
- Use kubernetes/pkg/util/iptables for iptables manipulation (danw@redhat.com)
- Move firewall/iptables setup to common.go (danw@redhat.com)
- Updated debug.sh to handle origin vs OSE vs atomic naming conventions
  (danw@redhat.com)
- Change deprecated "oc get -t" to "oc get --template" in debug.sh
  (danw@redhat.com)
- Indicate if debug.sh fails because KUBECONFIG isn't set. (danw@redhat.com)
- Simplify service tracking by using kapi.NamespaceAll (danw@redhat.com)
- let Status call not do docker inspect, as an empty output defaults to that
  only within kubelet (rajatchopra@gmail.com)
- Replace hostname -f with unmae -n (nakayamakenjiro@gmail.com)
- Remove unused import/method (rpenta@redhat.com)
- plugins: Update Kube client imports (mkargaki@redhat.com)
- More code cleanup (no new changes) (rpenta@redhat.com)
- Fix gofmt in previous commit (rpenta@redhat.com)
- Fix imports in openshift-sdn plugins (rpenta@redhat.com)
- Sync openshift-sdn/plugins to origin/Godeps/.../openshift-sdn/plugins
  (rpenta@redhat.com)
- IP/network-related variable naming consistency/clarify (danw@redhat.com)
- Fix up setup.sh args (danw@redhat.com)
- Remove dead code from kube plugin (that was already removed from multitenant)
  (danw@redhat.com)
- Update setup_required to do the correct check for docker-in-docker
  (danw@redhat.com)
- Fix hack/test.sh (rpenta@redhat.com)
- Remove hack/build.sh from travis.yml (rpenta@redhat.com)
- Update README.md (rpenta@redhat.com)
- sync-to-origin.sh - Program to sync openshift-sdn changes to origin
  repository (rpenta@redhat.com)
- Remove Makefile (rpenta@redhat.com)
- Flat and Multitenant plugins for openshift sdn (rpenta@redhat.com)
- Move ovssubnet to pkg dir (rpenta@redhat.com)
- Remove standalone openshift-sdn binary (rpenta@redhat.com)
- Remove etcd registry (rpenta@redhat.com)
- cleanup lbr (rpenta@redhat.com)
- debug.sh: improve progress output a bit (danw@redhat.com)
- debug.sh: test services (danw@redhat.com)
- debug.sh: test default namespace in connectivity check (danw@redhat.com)
- Move vovsbr to br0 port 3 rather than 9 (danw@redhat.com)
- debug.sh: add some missing quotes (danw@redhat.com)
- Add a tool for gathering data to debug networking problems (danw@redhat.com)
- Cleanup docker0 interface before restarting docker, since it won't do so
  itself. (eric.mountain@amadeus.com)
- Fix SDN race conditions during master/node setup (rpenta@redhat.com)
- Use firewalld D-Bus API to configure firewall rules (danw@redhat.com)
- Add godbus dep (danw@redhat.com)
- Fix race condition in pkg/netutils/server/server_test.go (danw@redhat.com)
- Fix error messages in server_test.go (danw@redhat.com)
- Status hook for multitenant plugin (rpenta@redhat.com)
- Remove some testing cruft from the multitenant plugin (danw@redhat.com)
- Implement kubernetes status hook in kube plugin (danw@redhat.com)
- Update README.md (ccoleman@redhat.com)
- Make SDN MTU configurable (rpenta@redhat.com)
- Expose method for getting node IP in osdn api (rpenta@redhat.com)
- Revoke VNID for admin namespaces (rpenta@redhat.com)
- For multitenant plugin, handle both existing and admin namespaces
  (rpenta@redhat.com)
- Add GOPATH setup to hack/test.sh (danw@redhat.com)
- Improve error messages in pkg/netutils/server/server_test (danw@redhat.com)
- Make Get/Watch interfaces for nodes/subnets/namespaces consistent
  (rpenta@redhat.com)
- Consistent error logging in standalone registry/IP allocator
  (rpenta@redhat.com)
- Multi-tenant service isolation (danw@redhat.com)
- Get node IP from GetNodes() instead of net.Lookup() (rpenta@redhat.com)
- Renamed some field names to remove confusion (rpenta@redhat.com)
- VNID fixes (rpenta@redhat.com)
- Revert "Merge pull request #115 from danwinship/service-isolation"
  (danw@redhat.com)
- Add support for dind in multitenant plugin (marun@redhat.com)
- Fix veth host discovery for multitenant plugin (marun@redhat.com)
- In openshift-sdn standalone mode, trigger node event when node ip changes
  (rpenta@redhat.com)
- Pass updated node IP in the NodeEvent to avoid cache/stale lookup by
  net.LookupIP() (rpenta@redhat.com)
- Replace 'minion' with 'node' or 'nodeIP' based on context (rpenta@redhat.com)
- Update subnet openflow rules in case of host ip change (rpenta@redhat.com)
- Add install-dev target to makefile. (marun@redhat.com)
- Add support for docker-in-docker deployment (marun@redhat.com)
- Enable IP forwarding during openshift sdn setup (rpenta@redhat.com)
- Fix veth lookup for kernels with interface suffix (marun@redhat.com)
- service isolation support (danw@redhat.com)
- Add basic plugin migration script (dcbw@redhat.com)
- Remove some dead code (dcbw@redhat.com)
- Add some documentation about our isolation implementation (dcbw@redhat.com)
- add a few more comments to the multitenant OVS setup (danw@redhat.com)
- simplify the incoming-from-docker-bridge rule (danw@redhat.com)
- drop an unnecessary multitenant ovs rule (danw@redhat.com)
- fix a comment in openshift-sdn-multitenant-setup.sh (danw@redhat.com)
- make cross node services work (make the gateway a special exit); add priority
  to overlapping rules (rchopra@redhat.com)
- Let docker-only container global traffic through the vSwitch to tun0
  (dcbw@redhat.com)
- Allow vnid=x to send to vnid=0 on the same host (dcbw@redhat.com)
- distribute vnids from 10 (rchopra@redhat.com)
- Initialise VnidMap (rchopra@redhat.com)
- Don't install multitenant plugin for now (dcbw@redhat.com)
- Fix shell syntax error in openshift-ovs-multitenant (dcbw@redhat.com)
- Make the multitenant controller work (danw@redhat.com)
- multitenancy: logic to create and manage netIds for new namespaces
  (rchopra@redhat.com)
- multitenant mode call from 'main'; its just a copy of the kube functionality
  - no real multitenancy feature added (rchopra@redhat.com)
- move api,types,bin files to respective controllers (rchopra@redhat.com)
- fix error in bash script (rchopra@redhat.com)
- ignore the SDN setup/teardown if the NetworkMode of the container is 'host'
  (rchopra@redhat.com)
- fix issue#88 - generate unique cookie based on vtep (rchopra@redhat.com)
- install docker systemd conf file (rchopra@redhat.com)
- sdn should not re-configure docker/ovs if it appears to be a harmless restart
  (rchopra@redhat.com)
- Make docker-network configuration only happen when sdn is running
  (sdodson@redhat.com)
- Signal when the SDN is ready (ccoleman@redhat.com)
- Fix origin issue#2834; submit existing subnets as in-use (rchopra@redhat.com)
- doc: fix spelling error in example (thaller@redhat.com)
- kube-subnet-setup: fix setup iptables FORWARD rule (thaller@redhat.com)
- add sysctl flag to kube-setup (rchopra@redhat.com)
- Update the comments that go into /etc/sysconfig/docker-network
  (sdodson@redhat.com)
- lockwrap shell utils; add support for internet connectivity to containers
  spun directly through docker (rchopra@redhat.com)
- Forward packages to/from cluster_network (sdodson@redhat.com)
- Update /etc/sysconfig/docker-network rather than /etc/sysconfig/docker
  (sdodson@redhat.com)
- disallow 127.0.0.1 as vtep resolution (rchopra@redhat.com)
- state file no more with docker 1.6, use ethtool instead. Credit: Dan Winship
  (rchopra@redhat.com)
- fixed the default to kubernetes minion path (rchopra@redhat.com)
- Install files under %%{_buildroot}, fixes rpm build (sdodson@redhat.com)
- add option for etcd path to watched for minions (rajatchopra@gmail.com)
- remove revision arg from api as it is an etcd abstraction
  (rchopra@redhat.com)
- first set of reorg for api-fication of openshift-sdn (rchopra@redhat.com)
- fix bz1215107. Use the lbr gateway as the tun device address.
  (rchopra@redhat.com)
- kube plugin (rchopra@redhat.com)
- Adds network diagram. (mrunalp@gmail.com)
- Set DOCKER_NETWORK_OPTIONS via /etc/sysconfig/docker-network
  (sdodson@redhat.com)
- Add ip route description. (mrunal@me.com)
- Add router instructions. (mrunal@me.com)
- Add node instructions (mrunal@me.com)
- Add introduction (mrunal@me.com)
- Add README for native container networking. (mrunalp@gmail.com)
- Add SyslogIdentifier=openshift-sdn-{master,node} respectively
  (sdodson@redhat.com)
- add error checking to netutils/server; move ipam interface up
  (rchopra@redhat.com)
- netutils server (rchopra@redhat.com)
- Add service dependency triggering on enablement (sdodson@redhat.com)
- Adds a test for IP release. (mrunalp@gmail.com)
- Fix typo. (mrunalp@gmail.com)
- Add tests for IP Allocator. (mrunalp@gmail.com)
- Add IP Allocator. (mrunalp@gmail.com)
- Make SDN service dependencies more strict (sdodson@redhat.com)
- Document -container-network & -container-subnet-length in master sysconfig
  (sdodson@redhat.com)
- return if error is a stop by user event (rchopra@redhat.com)
- error handling, cleanup, formatting and bugfix in generating default gateway
  for a subnet (rchopra@redhat.com)
- store network config in etcd (rchopra@redhat.com)
- Makes subnets configurable. (mrunalp@gmail.com)
- Improve output formatting (miciah.masters@gmail.com)
- remove unnecessary for loop and handle client create error
  (rchopra@redhat.com)
- Add Restart=on-failure to unit files (miciah.masters@gmail.com)
- Remove unncessary print (nakayamakenjiro@gmail.com)
- Fixed error handling to catch error from newNetworkManager
  (nakayamakenjiro@gmail.com)
- WatchMinions: Correctly handle errors (miciah.masters@gmail.com)
- Add travis build file. (mrunalp@gmail.com)
- Add gofmt verifier script from Openshift. (mrunalp@gmail.com)
- Add starter test script. (mrunalp@gmail.com)
- Revert to hostname -f as go builtin doesn't return FQDN. (mrunalp@gmail.com)
- Always treat -etcd-path as a directory (miciah.masters@gmail.com)
- openshift-sdn-master does not require openvswitch or bridge-utils
  (sdodson@redhat.com)
- Update commit to a commit that exists upstream (sdodson@redhat.com)
- Require systemd in subpackages, update setup for tarball format
  (sdodson@redhat.com)
- Remove DOCKER_OPTIONS references from master sysconfig (sdodson@redhat.com)
- Push packaging work into the upstream repo (sdodson@redhat.com)
- Fix comment. (mrunalp@gmail.com)
- Rename refactor in the controller. (mrunalp@gmail.com)
- Use library function to get hostname. (mrunalp@gmail.com)
- Add a LICENSE file. (mrunalp@gmail.com)
- Just use Error when no formatting is needed (rchopra@redhat.com)
- fix issue#12. Give some slack for etcd to come alive. (rchopra@redhat.com)
- fix iptables rules for traffic from docker (rchopra@redhat.com)
- Fixed misspellings in output (jolamb@redhat.com)
- Update README.md (rchopra@redhat.com)
- Document DOCKER_OPTIONS variable (jolamb@redhat.com)
- Add helpful comment at start of docker sysconfig (jolamb@redhat.com)
- Use $DOCKER_OPTIONS env var for docker settings if present
  (jolamb@redhat.com)
- iptables modifications for vxlan (rchopra@redhat.com)
- non-permanence for linux bridge, so that we do not need restart network
  service (rchopra@redhat.com)
- init minion registry for sync mode, updated README for sync mode
  (rchopra@redhat.com)
- Sync mode for running independently of PaaS to register nodes
  (rchopra@redhat.com)
- burnish that spot (rchopra@redhat.com)
- README enhancements, specified requirements (rchopra@redhat.com)
- GetMinions fix on key (rchopra@redhat.com)
- gofmt fixes. (mrunalp@gmail.com)
- Update README.md (rchopra@redhat.com)
- initial commit (rchopra@redhat.com)

* Tue Jul 05 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.2
- clarify makefile (li.guangxu@zte.com.cn)
- Remove deprecated man pages (ffranz@redhat.com)
- Fixes usage for oc set probe and oc debug (ffranz@redhat.com)
- Add test for DockerImageConfig parsing and counting size of image layers
  (agladkov@redhat.com)
- Fix parsing DockerImageConfig (agladkov@redhat.com)
- disable stack tracing logic for curl operations, it breaks them.
  (bparees@redhat.com)
- bump(github.com/openshift/source-to-image):
  ebd41a8b24bd95716b90a05fe0fda540c69fc119 (bparees@redhat.com)
- better warning for users when the buildconfig is of type binary
  (bparees@redhat.com)
- sort projects alphabetically (jvallejo@redhat.com)
- UPSTREAM: 28179: dedup workqueue requeing (deads@redhat.com)
- Add test for invalid client cert (jliggitt@redhat.com)
- add clusterquota projection for associated projects (deads@redhat.com)
- integration: wait for synced config before testing autoscaling
  (mkargaki@redhat.com)
- Unify counting of image layers in our helper and registry
  (agladkov@redhat.com)
- extended: test for iterative deployments (mkargaki@redhat.com)
- Limit the number of events and deployments displayed in dc describer
  (mfojtik@redhat.com)
- remove extra deploy invocation for sample repos tests (bparees@redhat.com)
- bump(github.com/openshift/source-to-image):
  ec23cd36484d1c32d3cfdbbc160d30d06bac9db4 (jupierce@redhat.com)
- Clean up references to NETWORKING_DEBUG (marun@redhat.com)
- Bump vagrant memory to allow go builds (marun@redhat.com)
- UPSTREAM: 27435: Fix bugs in DeltaFIFO (deads@redhat.com)
- update completion help example (jvallejo@redhat.com)
- update generated docs (jvallejo@redhat.com)
- update completion help to use root command in examples (jvallejo@redhat.com)
- set env from secrets and configmaps (sjenning@redhat.com)
- Note that --env flag doesn't apply to templates (rhcarvalho@gmail.com)
- Simplify calls to FixturePath (rhcarvalho@gmail.com)
- oc: support rollout undo (mkargaki@redhat.com)
- oc: tests for rollout {pause,resume,history} (mkargaki@redhat.com)
- oc: support rollout history (mkargaki@redhat.com)
- UPSTREAM: 27267: kubectl: refactor rollout history (mkargaki@redhat.com)
- oc: support rollout {pause,resume} (mkargaki@redhat.com)
- oc: generated completions/docs for rollout (mkargaki@redhat.com)
- oc: make route utility reusable across packages (mkargaki@redhat.com)
- oc: port rollout command from kubectl (mkargaki@redhat.com)
- add new SCL version imagestreams (bparees@redhat.com)
- remove source repo from pipeline buildconfig (bparees@redhat.com)
- deploy: update tests in the deployment controller (mkargaki@redhat.com)
- deploy: use shared caches in the deployment controller (mkargaki@redhat.com)
- Fix --show-events=false for build configs and deployment configs
  (mfojtik@redhat.com)
- Update for openshift-sdn changes (danw@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  e2106a29fc38f962af64332e6e1d9f94e421c7aa (danw@redhat.com)
- Add 'MissingSecret' reason for pending builds (mfojtik@redhat.com)
- Don't fallback to cert when given invalid token (mkhan@redhat.com)
- commonize bc input across strategies; better insure bc ouput is set
  (gmontero@redhat.com)
- Do not hard code build name (rhcarvalho@gmail.com)
- e2e: honor $USE_IMAGES during registry's deployment (miminar@redhat.com)

* Thu Jun 30 2016 Troy Dawson <tdawson@redhat.com> 3.3.0.1
- add OSE 3.3 build target (tdawson@redhat.com)
- add oc create clusterquota (deads@redhat.com)
- Enable certain test debug with delve using DLV_DEBUG envvar.
  (maszulik@redhat.com)
- Registry auth cleanup (jliggitt@redhat.com)
- dind: avoid being targeted by oci-systemd-hooks (marun@redhat.com)
- dind: minor cleanup in systemd masking (marun@redhat.com)
- dind: stop systemd containers with RTMIN+3 (marun@redhat.com)
- Update hack/ose_image build scripts (tdawson@redhat.com)
- add clusterresourcequota/namespace reverse index (deads@redhat.com)
- Add master lease endpoint reconciler (agoldste@redhat.com)
- Send lifecycle hook status events from deployer pod (mfojtik@redhat.com)
- remove underscores from function names (deads@redhat.com)
- ab testing (rchopra@redhat.com)
- react to 27341: this does not fix races in our code (deads@redhat.com)
- UPSTREAM: 27341: Fix race in informer (deads@redhat.com)
- UPSTREAM: 28025: Add EndpointReconciler to master Config
  (agoldste@redhat.com)
- UPSTREAM: 26915: Extract interface for master endpoints reconciler
  (agoldste@redhat.com)

* Tue Jun 28 2016 Scott Dodson <sdodson@redhat.com> 3.3.0.0
- Godeps did not properly remove old registry godeps (ccoleman@redhat.com)
- track generated manpages (jvallejo@redhat.com)
- add manpage generation utils (jvallejo@redhat.com)
- UPSTREAM: 6744: add upstream genutils helpers (jvallejo@redhat.com)
- Fix sti build e2e test (miminar@redhat.com)
- ignore build version on bc update if version is older than existing value
  (bparees@redhat.com)
- force a db deployment in extended template tests (bparees@redhat.com)
- use an image with a valid docker1.9 manifest (bparees@redhat.com)
- Use docker/distribution v2.4.0+ (agladkov@redhat.com)
- Fixes oc set env --overwrite=false with multiple resources
  (ffranz@redhat.com)
- Changing logging statments to Clayton's spec (jupierce@redhat.com)
- oc: cleanup expose; make --port configurable for routes (mkargaki@redhat.com)
- add configchange trigger so deploy happens on imagechange
  (bparees@redhat.com)
- restructure run policy test and add logging (bparees@redhat.com)
- controller: move shared informers in separate package (mkargaki@redhat.com)
- deploy: fix initial image change deployments (mkargaki@redhat.com)
- oc: restore legacy behavior for deploy --latest (mkargaki@redhat.com)
- use valid commit for binary build test (bparees@redhat.com)
- Revert "use cache for referential integrity and escalation checks"
  (deads@redhat.com)
- tweak sample pipeline template and add to oc cluster up (bparees@redhat.com)
- cache: add an image stream lister and a reference index (mkargaki@redhat.com)
- deploy: add shared caches in the trigger controller (mkargaki@redhat.com)
- move _tools to _output/tools (sjenning@redhat.com)
- godep scripts (sjenning@redhat.com)
- Add a simple script for starting a local docker registry
  (ccoleman@redhat.com)
- UPSTREAM: 25913: daemonset handle DeletedFinalStateUnknown (deads@redhat.com)
- use cache for referential integrity and escalation checks (deads@redhat.com)
- add cluster quota APIs (deads@redhat.com)
- oc set env must respect --overwrite=false (ffranz@redhat.com)
- display cache mutation errors in stdout (deads@redhat.com)
- dc controller was mutating cache objects (deads@redhat.com)
- increase watch timeout for build controller test (bparees@redhat.com)
- use shared informer for authorization (deads@redhat.com)
- UPSTREAM: 27784: add optional mutation checks for shared informer cache
  (deads@redhat.com)
- UPSTREAM: 27786: add lastsyncresourceversion to sharedinformer
  (deads@redhat.com)
- deploy: enhance status for deploymentconfigs (mkargaki@redhat.com)
- deploy: generated code for deploymentconfig status enhancements
  (mkargaki@redhat.com)
- extended: test deploymentconfig rollback (mkargaki@redhat.com)
- oc: make rollback use both paths for rolling back (mkargaki@redhat.com)
- deploy: add new endpoint for rolling back deploymentconfigs
  (mkargaki@redhat.com)
- deploy: generated code for rollback (mkargaki@redhat.com)
- Add missed fast path conversions (will go upstream eventually)
  (ccoleman@redhat.com)
- Don't state crashlooping container when pod only has one container
  (mkhan@redhat.com)
- hide authorization resource groups to prevent further usage
  (deads@redhat.com)
- handle cyclic dependencies in build-chain (gmontero@redhat.com)
- highlight current project with asterisk (jvallejo@redhat.com)
- Fixes examples of oc proxy (ffranz@redhat.com)
- Delete obsolete Dockerfile (jawnsy@redhat.com)
- Expose a way for clients to discover OpenShift API resources
  (ccoleman@redhat.com)
- update cluster-reader tests to use discovery (deads@redhat.com)
- UPSTREAM: 26355: refactor quota evaluation to cleanly abstract the quota
  access (deads@redhat.com)
- Use include flags with build (agladkov@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- bump(github.com/docker/distribution):
  596ca8b86acd3feebedae6bc08abf2a48d403a14 (agladkov@redhat.com)
- refactor openshift for quota changes (deads@redhat.com)
- UPSTREAM: 25091: reduce conflict retries (deads@redhat.com)
- UPSTREAM: 25414: Improve quota integration test to not use events
  (deads@redhat.com)
- deploy: reread hook logs on Retry (mkargaki@redhat.com)
- Unmarshal ports given in integer format (surajssd009005@gmail.com)
- Changing secrets.go to avoid unreliable docker build output capture. Also
  changing test cases so that BuildConfig resources are local to the tree and
  do not need to be current in origin/master HEAD. (jupierce@redhat.com)
- bump(github.com/openshift/source-to-image):
  69a0d96663b775c5e8fa942401c7bb9ca495e8f0 (bparees@redhat.com)
- add oc completion cmd wrapper (jvallejo@redhat.com)
- Modify oadm print statements to use stderr (mkhan@redhat.com)
- bump(coreos/etcd): fix etcd hash in Godeps.json (agoldste@redhat.com)
- add /version/openshift (deads@redhat.com)
- deploy: use shared caches in the dc controller (mkargaki@redhat.com)
- deploy: caches obsolete dc update in the deployerpod controller
  (mkargaki@redhat.com)
- cache: add a deploymentconfig lister (mkargaki@redhat.com)
- Revert "fixup coreos dep bump" (ccoleman@redhat.com)
- Made output of error more specific to bad parameter in a template file
  Updated tests for my change (rymurphy@redhat.com)
- Bump origin-web-console (cf5a74d) (jforrest@redhat.com)
- Bump origin-web-console (cf5a74d) (jforrest@redhat.com)
- bump(coreos/etcd):8b320e7c550067b1dfb37bd1682e8067023e0751 fixup coreos dep
  bump (sjenning@redhat.com)
- bump(github.com/AaronO/go-git-http/auth) but not really because mfojtik was
  just too tired to type those last 2 character (eparis@redhat.com)
- Clean up unused token secret (jliggitt@redhat.com)
- Update MySQL replication tests to reflect new template
  (nagy.martin@gmail.com)
- Add missing fuzzers for AnonymousConfig test (mfojtik@redhat.com)
- revise pipeline instructions for current sample state (bparees@redhat.com)
- Clean up unused token secret (jliggitt@redhat.com)
- Add a make perform-official-release target (ccoleman@redhat.com)
- Don't enforce quota in registry by default (miminar@redhat.com)
- oc describe for JenkinsBuildStrategy (gmontero@redhat.com)
- DSL openShift -> openshift in Jenkinsfile (gmontero@redhat.com)
- add mutation cache (deads@redhat.com)
- UPSTREAM: 25091: partial - reduce conflict retries (deads@redhat.com)
- convert dockercfg secret generator to a work queue (deads@redhat.com)
- return status error from build admission (deads@redhat.com)
- Fix docker tag command in hack/build-images.sh (mkumatag@in.ibm.com)
- Deleteing buildconfig with wildcard results in wrong output not related to
  action taken (skramaja@redhat.com)
- GIF version of asciicast (ccoleman@redhat.com)
- Ignore attempt to empty route spec.host field (jliggitt@redhat.com)
- Add screen cap of oc cluster up to README (ccoleman@redhat.com)
- Add error detection and UT to image progress (cewong@redhat.com)
- UPSTREAM: 27644: Use preferred group version when discovery fails due to 403
  (mkhan@redhat.com)
- Disable ResourceQuota while it is being investigated (ccoleman@redhat.com)
- UPSTREAM: <carry>: Limit affinity but handle error (ccoleman@redhat.com)
- UPSTREAM: <drop>: Remove string trimming (ccoleman@redhat.com)
- Reenable upstream e2e, disable PodAffinity (ccoleman@redhat.com)
- When updating policies and roles, only update last modified on change
  (ccoleman@redhat.com)
- DiscoveryRESTMapper was not caching the calculated mapper
  (ccoleman@redhat.com)
- UPSTREAM: 27243: Don't alter error type from server (ccoleman@redhat.com)
- UPSTREAM: 27242: Make discovery client parameterizable to legacy prefix
  (ccoleman@redhat.com)
- Fixes router printer line breaks (ffranz@redhat.com)
- Update test data to not reference v1beta3 (jforrest@redhat.com)
- cluster up: prevent start without a writeable KUBECONFIG (cewong@redhat.com)
- udpate jenkinsfile to use new DSL (gmontero@redhat.com)
- Added newline print to warning when individual rsync strategies fail Changed
  the return error to say that all strategies have failed, as opposed to just
  the final strategy error. (rymurphy@redhat.com)
- UPSTREAM: 23801: update Godeps completion support (jvallejo@redhat.com)
- Don't enforce quota in registry by default (miminar@redhat.com)
- deploy: deep-copy rcs before mutating them (mkargaki@redhat.com)
- UPSTREAM: <drop>: Disable timeouts on kubelet pull and logs
  (ccoleman@redhat.com)
- Add immutable updates on access and authorize tokens (ccoleman@redhat.com)
- Build LastVersion should be int64 (ccoleman@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  c3721f675f5474b717e6eb6aff9b837e095b6840 (rpenta@redhat.com)
- Auto generated conversions and swagger spec/descriptions for ClusterNetwork
  resource and node networkPluginName changes (rpenta@redhat.com)
- Make node to auto detect openshift network plugin from master
  (rpenta@redhat.com)
- Use ClusterNetworkDefault for referring to 'default' cluster network
  (rpenta@redhat.com)
- Added pluginName field to ClusterNetwork resource (rpenta@redhat.com)
- Fixes deprecated oc exec examples (ffranz@redhat.com)
- add clusterquota types (deads@redhat.com)
- React to int -> intXX changes in the code (ccoleman@redhat.com)
- start using shared informer (deads@redhat.com)
- UPSTREAM: 26276: make quota validation re-useable (deads@redhat.com)
- add cluster rules to namespace powers (deads@redhat.com)
- import docker-compose env var substitution (surajssd009005@gmail.com)
- oc: don't print anything specific on create yet (mkargaki@redhat.com)
- UPSTREAM: 26161: kubectl: move printObjectSpecificMessage in factory
  (mkargaki@redhat.com)
- deploy: use new sorting logic for deployment logs (mkargaki@redhat.com)
- UPSTREAM: 26771: kubectl: fix sort logic for logs (mkargaki@redhat.com)
- UPSTREAM: 27048: kubectl: return more meaningful timeout errors
  (mkargaki@redhat.com)
- UPSTREAM: 27048: kubectl: ignore only update conflicts in the scaler
  (mkargaki@redhat.com)
- Convert all int types to int32 or int64 (ccoleman@redhat.com)
- Add fast path conversions for Origin resources (ccoleman@redhat.com)
- Generated files (mfojtik@redhat.com)
- Interesting changes (mfojtik@redhat.com)
- defaulting changes (mfojtik@redhat.com)
- boring changes (mfojtik@redhat.com)
- UPSTREAM: 27412: Allow specifying base location for test etcd data
  (jliggitt@redhat.com)
- UPSTREAM: 26078: Fix panic when the namespace flag is not present
  (mfojtik@redhat.com)
- UPSTREAM: openshift/openshift-sdn: 321: Fix integers and add missing methods
  to OVS plugin (mfojtik@redhat.com)
- UPSTREAM: coreos/etcd: 5617: fileutil: avoid double preallocation
  (jliggitt@redhat.com)
- UPSTREAM: coreos/etcd: 5572: fall back to truncate() if fallocate is
  interrupted (mfojtik@redhat.com)
- UPSTREAM: skynetservices/skydns: <carry>: Allow listen only ipv4
  (ccoleman@redhat.com)
- UPSTREAM: skynetservices/skydns: <carry>: Disable systemd activation for DNS
  (ccoleman@redhat.com)
- UPSTREAM: google/cadvisor: <carry>: Disable container_hints flag that is set
  twice (mfojtik@redhat.com)
- UPSTREAM: emicklei/go-restful: <carry>: Add "Info" to go-restful ApiDecl
  (ccoleman@redhat.com)
- bump(denverdino/aliyungo): 554da7ebe31b6172a8f15a0b1cf8c628145bed6a
  (mfojtik@redhat.com)
- bump(github.com/docker/engine-api): 3d72d392d07bece8d7d7b2a3b6b2e57c2df376a2
  (mfojtik@redhat.com)
- bump(github.com/google/cadvisor): 750f18e5eac3f6193b354fc14c03d92d4318a0ec
  (mfojtik@redhat.com)
- bump(github.com/spf13/cobra): 4c05eb1145f16d0e6bb4a3e1b6d769f4713cb41f
  (mfojtik@redhat.com)
- bump(github.com/onsi/ginkgo): 2c2e9bb47b4e44067024f29339588cac8b34dd12
  (mfojtik@redhat.com)
- bump(github.com/coreos): 8b320e7c550067b1dfb37bd1682e8067023e0751
  (mfojtik@redhat.com)
- bump(github.com/emicklei/go-restful):
  496d495156da218b9912f03dfa7df7f80fbd8cc3 (mfojtik@redhat.com)
- bump(github.com/fsouza/go-dockerclient):
  bf97c77db7c945cbcdbf09d56c6f87a66f54537b (mfojtik@redhat.com)
- bump(github.com/onsi/gomega): 7ce781ea776b2fd506491011353bded2e40c8467
  (mfojtik@redhat.com)
- bump(github.com/skynetservices/skydns):
  1be70b5b8aa07acccd972146d84011b670af88b4 (mfojtik@redhat.com)
- bump(gopkg.in/yaml.v2) a83829b6f1293c91addabc89d0571c246397bbf4
  (mfojtik@redhat.com)
- bump(k8s.io/kubernetes): 686fe3889ed652b3907579c9e46f247484f52e8d
  (mfojtik@redhat.com)
- Refactored binary build shell invocation (skuznets@redhat.com)
- Refactored stacktrace implementation for bash scripts (skuznets@redhat.com)
- Failure in extended deployment test (ccoleman@redhat.com)
- Fixes examples of port-forward (ffranz@redhat.com)
- Added quote func to make all DOT ID valids (mkhan@redhat.com)
- Lock release build to specific versions (ccoleman@redhat.com)
- Bump origin-web-console (7840198) (jforrest@redhat.com)
- atomic-registry via systemd (aweiteka@redhat.com)
- Typo fix - oc dockerbuild example (mkumatag@in.ibm.com)
- cluster up: optionally install metrics components (cewong@redhat.com)
- refactor to Complete-Validate-Run (pweil@redhat.com)
- Bug 1343681 - Fix tagsChanged logic (maszulik@redhat.com)
- tolerate multiple bcs pointing to same istag for oc status
  (gmontero@redhat.com)
- use defined constant (pweil@redhat.com)
- deploy: switch config change to a generic trigger controller
  (mkargaki@redhat.com)
- deploy: move config change controller to new location (mkargaki@redhat.com)
- deploy: stop instantiating from the image change controller
  (mkargaki@redhat.com)
- Changed glog V(1) and V(2) to V(0) (jupierce@redhat.com)
- Add make to all release images and add a Golang 1.7 image
  (ccoleman@redhat.com)
- Refactored os::text library to use `[[' tests instead of `[' tests
  (skuznets@redhat.com)
- Implemented `os::text::clear_string' to remove a string from TTY output
  (skuznets@redhat.com)
- Escaped special characters in regex (skuznets@redhat.com)
- Refactored `source' statements that drifted since the original PR
  (skuznets@redhat.com)
- Added `findutils' dependency for DIND image (skuznets@redhat.com)
- Renamed $ORIGIN_ROOT to $OS_ROOT for consistency (skuznets@redhat.com)
- Revert "Revert "Implemented a single-directive library import for Origin Bash
  scripts"" (skuznets@redhat.com)
- Update pipeline templates (spadgett@redhat.com)
- update "oc projects" to display all projects (jvallejo@redhat.com)
- Cleanup some dead test code (ccoleman@redhat.com)
- Increase the lease renewal fraction to 1/3 interval (ccoleman@redhat.com)
- Return the error from losing a lease to log output (ccoleman@redhat.com)
- Prevent route theft by removing the ability to update spec.host
  (ccoleman@redhat.com)
- allow map keys to be templatized (deads@redhat.com)
- deploy: remove deployer pods on cancellation (mkargaki@redhat.com)
- bypass k8s container image validation when ict's defined (allow for empty
  string) (gmontero@redhat.com)
- Update CONTRIBUTING and HACKING docs (mkargaki@redhat.com)
- Bug 1327108 - Updated error information when trying to import ImageStream
  pointing to a different one (maszulik@redhat.com)
- Revert "remove lastModified from policy and policybinding"
  (deads2k@users.noreply.github.com)
- Check logs after we verify the deployment passed (ccoleman@redhat.com)
- When tagging an image across image streams, rewrite the tag ref
  (ccoleman@redhat.com)
- Bug in oc describe imagestream (ccoleman@redhat.com)
- ImageStreamMappings should default DockerImageReference (ccoleman@redhat.com)
- Allow mutation of image dockerImageReference (ccoleman@redhat.com)
- Add 'oc create imagestream' (ccoleman@redhat.com)
- Allow --reference to be passed to oc tag (ccoleman@redhat.com)
- remove lastModified from policy and policybinding (deads@redhat.com)
- setup_tmpdir_vars(): TPMDIR -> TMPDIR (miciah.masters@gmail.com)
- respect scopes for rules review (deads@redhat.com)
- Updated Dockerfiles (ccoleman@redhat.com)
- Move directories appropriately (ccoleman@redhat.com)
- Change code to use /testdata/ instead of /fixtures directories
  (ccoleman@redhat.com)
- dind: enable intra pod test (marun@redhat.com)
- dind: Fix EmptyDir support (marun@redhat.com)
- dind: rm docker volumes natively (added in 1.9) (marun@redhat.com)
- bump(github.com/evanphx/json-patch):465937c80b3c07a7c7ad20cc934898646a91c1de
  (jimmidyson@gmail.com)
- remove multiple template example, it's a lie (bparees@redhat.com)
- Move fixtures to testdata directories (ccoleman@redhat.com)
- New release images locked to Go versions (ccoleman@redhat.com)
- Be more selective about what we generate for build names
  (ccoleman@redhat.com)
- Add instructions for downloading oc and using hosted template
  (sspeiche@redhat.com)
- don't specify sync project (bparees@redhat.com)
- cluster up: print last 10 lines of logs on errors after container started
  (cewong@redhat.com)
- Command to set deployment hooks on deployment configs (cewong@redhat.com)
- integration: bump timeout for non-automatic test (mkargaki@redhat.com)
- builders: simplified image progress reporting (cewong@redhat.com)
- Update bindata (mkargaki@redhat.com)
- Retry service account update on conflict (jliggitt@redhat.com)
- fix django typo (bparees@users.noreply.github.com)
- Add liveness probe for the ipfailover dc. (smitram@gmail.com)
- Allow import all tags when .spec.tags are specified as well
  (maszulik@redhat.com)
- allocate route host on update if host is empty in spec (pweil@redhat.com)
- add service dependency and infrastructure annotations (bparees@redhat.com)
- Added Signatures field to Image object (miminar@redhat.com)
- fix some ineffassign issues (v.behar@free.fr)
- Trap sigterm and cleanup - remove any assigned VIPs. (smitram@gmail.com)
- Add a GCS test to verify it is compiled in (ccoleman@redhat.com)
- mark docker compose as experimental (bparees@redhat.com)
- Build dockerregistry with tag `include_gcs` (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Distribution dependencies
  (ccoleman@redhat.com)
- debug / workarounds for image extended tests wrt current ICT based deploy
  behavior (gmontero@redhat.com)
- Update go version in contributing doc (jliggitt@redhat.com)
- Fix job e2e test (maszulik@redhat.com)
- make project watch work for namespace deletion (deads@redhat.com)
- set build-hook command (cewong@redhat.com)
- Guard against deleting entire filesystem in hack/test-go.sh
  (skuznets@redhat.com)
- Detect data races in `go test` log output (skuznets@redhat.com)
- Bug 1343426: list deployments correctly in oc deploy (mkargaki@redhat.com)
- auto-create a jenkins service account (bparees@redhat.com)
- Refactor image-import command into managable methods (maszulik@redhat.com)
- Increase the tests coverage for import-image command (maszulik@redhat.com)
- extended: refactor deployment tests (mkargaki@redhat.com)
- deploy: add pausing for deploymentconfigs (mkargaki@redhat.com)
- deploy: generated code for pause (mkargaki@redhat.com)
- bypass oc.Run() --follow --wait output flakes (gmontero@redhat.com)
- stop mutating cache (deads@redhat.com)
- add openshift ex config patch to modify master-config.yaml (deads@redhat.com)
- Improve escalation error message (jliggitt@redhat.com)
- prevent the build controller from escalating users (deads@redhat.com)
- Remove template fields with default values (rhcarvalho@gmail.com)
- fix scope character validation (jliggitt@redhat.com)
- UPSTREAM: 26554: kubectl: make --container-port actually work for expose
  (mkargaki@redhat.com)
- deploy: ignore set lastTriggeredImage on create (mkargaki@redhat.com)
- Fixes --show-labels when printing OpenShift resources (ffranz@redhat.com)
- UPSTREAM: 26793: expose printer utils (ffranz@redhat.com)
- Add the both ROUTER_SERVICE*SNI_PORT (cw-aleks@users.noreply.github.com)
- SCC check API: add addmission in podnodeconstraints (salvatore-
  dario.minonne@amadeus.com)
- Extended tests: added quota and limit range related tests
  (miminar@redhat.com)
- Cache limit range objects in registry (miminar@redhat.com)
- Allow to toggle quota enforcement in the registry (miminar@redhat.com)
- Limit imagestreamimages and imagestreamtags (miminar@redhat.com)
- Limit ImageStreams per project using quota (miminar@redhat.com)
- Use helpers to parse and make imagestream tag names (miminar@redhat.com)
- Image size admission with limitrange (pweil@redhat.com)
- Removed image quota evaluators (miminar@redhat.com)
- Serve DNS from the nodes (ccoleman@redhat.com)
- UPSTREAM: 24118: Proxy can be initialized directly (ccoleman@redhat.com)
- UPSTREAM: 24119: Use default in AddFlags (ccoleman@redhat.com)
- fix contextdir path (bparees@redhat.com)
- Improve validation message on build spec update (cewong@redhat.com)
- add service serving cert signer to token controller initialization
  (deads@redhat.com)
- UPSTREAM: <carry>: add service serving cert signer to token controller
  (deads@redhat.com)
- address typecast panic (gmontero@redhat.com)
- deflake second level timing issue in TestDescribeBuildDuration
  (gmontero@redhat.com)
- add validation to prevent filters on dn lookups (deads@redhat.com)
- BasicAuthIdentityProvider: Do not follow redirection (sgallagh@redhat.com)
- router: Add name and namespace templates params (miciah.masters@gmail.com)
- api: fix serialization test for deploymentconfigs (mkargaki@redhat.com)
- SCC check: API and validation (salvatore-dario.minonne@amadeus.com)
- builder: increase git check timeout exponentially (cewong@redhat.com)
- Add a test to validate ipfailover starts (ccoleman@redhat.com)
- oc set probe/triggers: set output version (cewong@redhat.com)
- Default container name on execPod lifecycle hooks (ccoleman@redhat.com)
- Add temporary HTTP failures to image retry (ccoleman@redhat.com)
- Import app.json to OpenShift applications (ccoleman@redhat.com)
- UPSTREAM: 25487: pod constraints func for quota validates resources
  (decarr@redhat.com)
- UPSTREAM: 24514: Quota ignores pod compute resources on updates
  (decarr@redhat.com)
- UPSTREAM: 25161: Sort resources in quota errors to avoid duplicate events
  (decarr@redhat.com)
- gitserver: allow anonymous access when using uid/pwd auth (cewong@redhat.com)
- Skip registry client v1 and v2 tests until we push with 1.10
  (ccoleman@redhat.com)
- deploy: restore ict behavior for deploymentconfig with no cc triggers
  (mkargaki@redhat.com)
- unbreak e2e after DNS and oc cluster up (deads@redhat.com)
- Update jenkins-master-template.json context (xiuwang)
- bump(github.com/openshift/source-to-image):
  2abf650e2e65008e5837266991ff4f2aed14bf39 (cewong@redhat.com)
- let builders create new imagestreams for pushes (deads@redhat.com)
- Allow size of image to be zero when schema1 from Hub (ccoleman@redhat.com)
- IPFailover was broken for alpha.1 (ccoleman@redhat.com)
- Add watch caching to origin types (jliggitt@redhat.com)
- test-integration etcd verbosity controlled by OSTEST_VERBOSE_ETCD
  (deads@redhat.com)
- fix oc describe panic (ipalade@redhat.com)
- Integration test for automatic=false ICTs (mkargaki@redhat.com)
- Bug 1340735: update dc image at most once on automatic=false
  (mkargaki@redhat.com)
- If --public-hostname is an IP, use it for server IP in cluster up
  (ccoleman@redhat.com)
- dockerbuild pulls all images rather than :latest (ccoleman@redhat.com)
- Bug 1340344 - Add additional interfaces implementation for handling watches,
  rsh, etc. (maszulik@redhat.com)
- Add test for suppression of router reload (marun@redhat.com)
- Printing duplicate messages during startbuild (ccoleman@redhat.com)
- Enable integration test debug with delve (marun@redhat.com)
- Optionally skip building integration test binary (marun@redhat.com)
- Fix verbose output for hack/test-integration.sh (marun@redhat.com)
- Avoid router reload during sync (marun@redhat.com)
- add impersonate-group (deads@redhat.com)
- Increased timeout in TestTimedCommand integration test (skuznets@redhat.com)
- add ROUTER_SLOWLORIS_TIMEOUT enviroment variable to default router image
  (jtanenba@redhat.com)
- Fix misspells reported by goreportcard.com (v.behar@free.fr)
- make the jenkins service last so that you can retry on failures
  (deads@redhat.com)
- cleanup bootstrap policy to allow future changes (deads@redhat.com)
- hack/util: wait for registry readiness (miminar@redhat.com)
- Updated auto generated docs for pod-network CLI cmd (rpenta@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  b74a40614df17a99b8f77ac0dbd9aaa3ffae3218 (rpenta@redhat.com)
- Pass iptablesSyncPeriod config to SDN node plugin (rpenta@redhat.com)
- Change default value of iptablesSyncPeriod from 5s to 30s (rpenta@redhat.com)
- Add goreportcard badge in README (mkargaki@redhat.com)
- add cli describer for oauthtokens (jvallejo@redhat.com)
- Document things, remove /usr/bin/docker mount from contrib systemd unit
  (sdodson@redhat.com)
- Fix git clone tests (jliggitt@redhat.com)
- chroot docker to the rootfs (sdodson@redhat.com)
- bump(github.com/openshift/source-to-image):
  6373e0ab0016dd013573f55eaa037c667f8e4a92 (vsemushi@redhat.com)
- UPSTREAM: 25907: Fix detection of docker cgroup on RHEL (agoldste@redhat.com)
- Updated allowed levels of `glog` info levels in CLI doc (skuznets@redhat.com)
- Tolerate partial successes on reconciling cluter role bindings
  (skuznets@redhat.com)
- add build trigger information and separate buildconfig data
  (ipalade@redhat.com)
- Warn if a docker compose service has no ports (ccoleman@redhat.com)
- bump(github.com/openshift/source-to-image):
  65db940221d282c2f7f1d21998bbd4af2f8a1c16 (gmontero@redhat.com)
- git server readme: fix references to gitserver (cewong@redhat.com)
- More pretty readme (ccoleman@redhat.com)
- gitserver: allow specifying build strategy (cewong@redhat.com)
- remove resource groups and divide by API group (deads@redhat.com)
- Truncate labels of builds generated from a BuildConfig (cewong@redhat.com)
- Refactored builder remote URI check to used timed ListRemote
  (skuznets@redhat.com)
- Added a timeout to `git ls-remote` invocation in `oc new-app`
  (skuznets@redhat.com)
- Refactored command functions not to accept a Writer (skuznets@redhat.com)
- make the pipeline readme pretty (bparees@redhat.com)
- Update egress-router package dependencies (danw@redhat.com)
- make the empty API groups to "" for policy rules standard (deads@redhat.com)
- Separated export from declaration for environment variables
  (skuznets@redhat.com)
- Added make update to simplify updating all generated artifacts.
  (maszulik@redhat.com)
- bump(github.com/Microsoft/go-winio):4f1a71750d95a5a8a46c40a67ffbed8129c2f138
  (sdodson@redhat.com)
- Service account token controller startup change (jliggitt@redhat.com)
- Disable sticky sessions for tcp routes based on env value. Updated to use env
  variable value directly as per @smarterclayton comments. (smitram@gmail.com)
- Update the jsonpath template link in oc get -h (ripcurld.github@gmail.com)
- UPSTREAM: 23858: Convert service account token controller to use a work queue
  (jliggitt@redhat.com)
- [RPMS] bump BuildRequires: golang to 1.6.2 (#19) (sdodson@redhat.com)
- cluster up: fix insecure registry argument message (cewong@redhat.com)
- add jenkins pipeline example files (bparees@redhat.com)
- UPSTREAM: 24537: Add locks in HPA test (deads@redhat.com)
- make admission plugins enablable or disabable based on config
  (deads@redhat.com)
- UPSTREAM: 25898: make admission plugins configurable based on external
  criteria (deads@redhat.com)
- respect scopes during authz delegation (deads@redhat.com)
- discovery rest mapper (deads@redhat.com)
- Update glogging to always set level, and to respect glog.level
  (ccoleman@redhat.com)
- Mount only host dir to copy files from (cewong@redhat.com)
- Enhance jenkins-persistent-template and jenkins-ephemeral-template
  (tnozicka@gmail.com)
- add scope options to oc policy can-i (deads@redhat.com)
- allow SAR to specify scopes (deads@redhat.com)
- cluster up: test bootstrap asset locations (cewong@redhat.com)
- Fix govet errors in oc cluster up for 1.6 (cewong@redhat.com)
- oc: inform about switch to the same project (mkargaki@redhat.com)
- Bug 1338679: emit events on failure to create a deployer pod
  (mkargaki@redhat.com)
- Drop Go 1.4 conditions, switch to 1.6 by default (ccoleman@redhat.com)
- Docker image cannot have - or _ in succession (ccoleman@redhat.com)
- Allow a user to specify --as-user or --as-root=false (ccoleman@redhat.com)
- Update extensionProperties to single array of structs (jforrest@redhat.com)
- eliminate need for SA token as client secret annotation (deads@redhat.com)
- Longer timeout on test fixture DC (ccoleman@redhat.com)
- Switch gitserver to use origin-base (cewong@redhat.com)
- Bump origin-web-console (5cfe43625c3885835f2b23b674e527ceabf17502)
  (jliggitt@redhat.com)
- policy unsafe proxy requests separately (jliggitt@redhat.com)
- Limit queryparam auth to websockets (jliggitt@redhat.com)
- cluster up: allow specifying a full image template (cewong@redhat.com)
- retry SA annotation updates in the test in case of conflicts or other
  weirdness (deads@redhat.com)
- bump(github.com/AaronO/go-git-http) 34209cf6cd947cfa52063bcb0f6d43cfa50c5566
  (cewong@redhat.com)
- deployapi: fix automatic description (mkargaki@redhat.com)
- Fix tests for the trigger controllers (mkargaki@redhat.com)
- Resolve image for all initial deployments (mkargaki@redhat.com)
- Deploymentconfig controller should update dc status on successful exit
  (mkargaki@redhat.com)
- Stop using the deploymentconfig generator (mkargaki@redhat.com)
- fix flaky configapi fuzzing for SA grant method (deads@redhat.com)
- Support arbitrary key:value properties for console extensions
  (jforrest@redhat.com)
- default to letting oauth clients request all scopes from users
  (deads@redhat.com)
- allow configuration of the SA oauth client grant flows (deads@redhat.com)
- cluster up: use alternate port for DNS when 53 is not available
  (cewong@redhat.com)
- use service account credentials as oauth token (deads@redhat.com)
- add AdditionalSecrets to oauthclients (deads@redhat.com)
- UPSTREAM: RangelReale/osin: <carry>: only request secret validation
  (deads@redhat.com)
- Update atomic registry quickstart artifacts (aweiteka@redhat.com)
- Restrict the scratch name to lowercase in the dockerbuilder
  (ccoleman@redhat.com)
- run registry as daemonset (pweil@redhat.com)
- cluster up: ensure you can login as administrator (cewong@redhat.com)
- F5: Cleanup mockF5.close() calls in tests (miciah.masters@gmail.com)
- add user:list-projects scope (deads@redhat.com)
- refactor whatcanido to can-i --list (deads@redhat.com)
- oc: separate test for the deploymentconfig describer (mkargaki@redhat.com)
- oc: enhance the deploymentconfig describer (mkargaki@redhat.com)
- print proper image name during root user warning (bparees@redhat.com)
- README for 'oc cluster up/down' (cewong@redhat.com)
- allow who-can to reference resource names (deads@redhat.com)
- extend timeout for SSCS tests running in parallel (deads@redhat.com)
- Allow update on hostsubnets (rpenta@redhat.com)
- Cleanup all asset related files in origin (jforrest@redhat.com)
- make scopes forbidden message friendlier to read (deads@redhat.com)
- Client command to start OpenShift with reasonable defaults
  (cewong@redhat.com)
- Include option in console vendoring script to generate the branch and commit
  automatically (jforrest@redhat.com)
- Bump origin-web-console (140a1f1) (jforrest@redhat.com)
- UPSTREAM: 25690: Fixes panic on round tripper when TLS under a proxy
  (ffranz@redhat.com)
- o Add locking around ops that change state. Its better to go   lockless
  eventually and use a "shadow" (cloned) copy of the   config when we do a
  reload. o Fixes as per @smarterclayton review comments. (smitram@gmail.com)
- create client code for oauth types and tests (deads@redhat.com)
- add scope restrictions to oauth clients (deads@redhat.com)
- container image field values irrelevant with ict's in automatic mode, don't
  have setting that confuses users; add updated quickstarts
  (gmontero@redhat.com)
- localsubjectaccessreview test-cmd demonstration with scoped token
  (deads@redhat.com)
-   o Add basic validation for route TLS configuration - checks that     input
  is "syntactically" valid.   o Checkpoint initial code.   o Add support for
  validating route tls config.   o Add option for validating route tls config.
  o Validation fixes.   o Check private key + cert mismatches.   o Add tests.
  o Record route rejection.   o Hook into add route processing + store invalid
  service alias configs     in another place - easy to check prior errors on
  readmission.   o Remove entry from invalid service alias configs upon route
  removal.   o Add generated completions.   o Bug fixes.   o Recording
  rejecting routes is not working completely.   o Fix status update problem -
  we should set the status to admitted     only if we had no errors handling a
  route.   o Rework to use a new controller - extended_validator as per
  @smarterclayton comments.   o Cleanup validation as per @liggitt comments.
  o Update bash completions.   o Fixup older validation unit tests.   o Changes
  as per @liggitt review comments + cleanup tests.   o Fix failing test.
  (smitram@gmail.com)
- Cleanup output to builds, force validation, better errors
  (ccoleman@redhat.com)
- Convert builder glog use to straight output (ccoleman@redhat.com)
- Add a simple glog replacement stub (ccoleman@redhat.com)
- Print hooks for custom deployments and multiline commands
  (ccoleman@redhat.com)
- Recreate deployment should always run the acceptor (ccoleman@redhat.com)
- Updated generated deepcopy/conversion (ccoleman@redhat.com)
- Enable custom deployments to have rolling/recreate as well
  (ccoleman@redhat.com)
- Add incremental conditional completion to deployments (ccoleman@redhat.com)
- UPSTREAM: 25617: Rolling updater should indicate progress
  (ccoleman@redhat.com)
- If desired conditions match, don't go into a wait loop (ccoleman@redhat.com)
- Add an annotation that skips creating deployer pods (ccoleman@redhat.com)
- Update deployment logs and surge on recreate (ccoleman@redhat.com)
- return status errors from build clone API (deads@redhat.com)
- Fix go vet errors (rhcarvalho@gmail.com)
- Ignore errors on debug when a template is returned (ccoleman@redhat.com)
- bump(github.com/openshift/source-to-image):
  64f909fd86e302e5e22a7dd2854278a97ed44cf4 (rhcarvalho@gmail.com)
- Deployment config pipeline must mark the pods it covers (ffranz@redhat.com)
- Revert "Revert "oc status must show monopods"" (ffranz@redhat.com)
- clarify 'oc delete' help info (somalley@redhat.com)
- Add script for vendoring the console repo into bindata.go files
  (jforrest@redhat.com)
- UPSTREAM: 25077: PLEG: reinspect pods that failed prior inspections
  (agoldste@redhat.com)
- Support schema2 of compose, set env vars properly (ccoleman@redhat.com)
- UPSTREAM: 25537: e2e make ForEach fail if filter is empty, fix no-op tests
  (decarr@redhat.com)
- add bash script demonstrating scoped tokens (deads@redhat.com)
- platformmanagement_public_704: Added basic auditing capabilities
  (maszulik@redhat.com)
- Remove dind hack from kubelet (marun@redhat.com)
- more spots to dump on error deployment logs in extended tests
  (gmontero@redhat.com)
- Truncate build pod label to allowed size (cewong@redhat.com)
- disable gitauth tests (gmontero@redhat.com)
- change BC ICT to automatic=true (more deterministic) (gmontero@redhat.com)
- Add NetworkManager dnsmasq configuration dispatcher (sdodson@redhat.com)
- Remove prompts from commented examples (rhcarvalho@gmail.com)
- db-templates: mongodb ephemeral dc should automatically resolve
  (mkargaki@redhat.com)
- UPSTREAM: 25501: SplitHostPort is needed since Request.RemoteAddr has the
  host:port format (maszulik@redhat.com)
- workaround to infinite wait on service in k8s utils; better deployment
  failure test diag (gmontero@redhat.com)
- provide a way to request escalating scopes (deads@redhat.com)
- prevent escalating resource access by default (deads@redhat.com)
- Fixing test flake for docker builds (bleanhar@redhat.com)
- Delete registry interfaces for clusternetwork/hostsubnet/netnamespace
  resources (rpenta@redhat.com)
- Refactored Bash scripts to use new preable for imports (skuznets@redhat.com)
- Declared all Bash library functions read-only. (skuznets@redhat.com)
- Implemented a single-directive library import for Origin Bash scripts
  (skuznets@redhat.com)
- Updated the function declaration syle in existing Bash libararies.
  (skuznets@redhat.com)
- migrate use of s.DefaultConvert to autoConvert (gmontero@redhat.com)
- UPSTREAM: 25472: tolerate nil error in HandleError (deads@redhat.com)
- Check for pod creation in scc exec admission (jliggitt@redhat.com)
- enforce scc during pod updates (deads@redhat.com)
- update jenkins example readme to leverage -z option on oc policy
  (gmontero@redhat.com)
- fix nil ImageChange conversion (gmontero@redhat.com)
- Interrupting a dockerbuild command should clean up leftovers
  (ccoleman@redhat.com)
- When building oc from build-*-images, use found path (ccoleman@redhat.com)
- Bug 1329138: stop emitting events on update conflicts (mkargaki@redhat.com)
- add istag to oc debug (deads@redhat.com)
- add scoped impersonation (deads@redhat.com)
- add service serving cert controller (deads@redhat.com)
- Show/Edit runPolicy (jhadvig@redhat.com)
- add descriptions for scopes (deads@redhat.com)
- add scope validation to tokens (deads@redhat.com)
- Pre-skip some GCE/AWS-only extended networking tests (danw@redhat.com)
- Reorganize extended network tests to save a bit of time (danw@redhat.com)
- bump(k8s.io/kubernetes): c2cc7fd2eafe14472098737b2c255ce4ca06d987
  (mfojtik@redhat.com)
- Add a Docker compose import and command (ccoleman@redhat.com)
- Ignore govet errors on ./third_party (ccoleman@redhat.com)
- UPSTREAM: 25018: Don't convert objects in the destination version
  (ccoleman@redhat.com)
- UPSTREAM: 24390: []byte conversion is not correct (ccoleman@redhat.com)
- bump(third_party/github.com/docker/libcompose):3ca15215f36154fbf64f15bfa305bf
  b0cebb6ca7 (ccoleman@redhat.com)
- bump(github.com/flynn/go-shlex):3f9db97f856818214da2e1057f8ad84803971cff
  (ccoleman@redhat.com)
- Add defensive if statements to valuesIn and valuesNotIn filters
  (admin@benjaminapetersen.me)
- UPSTREAM: 25369: Return 'too old' errors from watch cache via watch stream
  (jliggitt@redhat.com)
- add scope authorizer (deads@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  0fbae3c75c6802a5f5d7fe48d562856443457f6a (dcbw@redhat.com)
- Update for split-out openshift-sdn proxy plugin (dcbw@redhat.com)
- oc: update rollback example (mkargaki@redhat.com)
- Remove gaps from build chart for deleted builds (spadgett@redhat.com)
- Fix jshint warnings (spadgett@redhat.com)
- Changes to markup and the logo so that it isn't clipped at certain dimensions
  - Fixes bug 1328016 - CSS changes to enable logos of varing dimensions within
  the following sizes - Logo max width 230px / height 36px
  (sgoodwin@redhat.com)
- UPSTREAM: 23574: add user.Info.GetExtra (deads@redhat.com)
- Test path-based routing via HTTPS and WebSockets (elyscape@gmail.com)
- cluster quota options (deads@redhat.com)
- Include ext-searchbox.js for Ace editor search (spadgett@redhat.com)
- deployapi: add separate call for updating DC status (mkargaki@redhat.com)
- deployapi: add generation numbers (mkargaki@redhat.com)
- Bug 1333300 - add/update resource labels and annotations (jhadvig@redhat.com)
- Bug 1333669 - error msg when unknown apiVersion or kind (jhadvig@redhat.com)
- Calculate effective limit min in UI for request/limit ratio
  (spadgett@redhat.com)
- Bug 1333651 - set object group and version (jhadvig@redhat.com)
- haproxy Cookie id leaks information about software (pcameron@redhat.com)
- Project should not fetch client immediately (ccoleman@redhat.com)
- put new quota size in extended test (gmontero@redhat.com)
- Remove deployments resource from roles (jliggitt@redhat.com)
- dockerbuild: stop container before committing it (cewong@redhat.com)
- refactor webhook code (bparees@redhat.com)
- Add reconcile protection for roles (jliggitt@redhat.com)
- more catching of build errors, dump of logs, etc.; add registry pod disk
  usage analysis (gmontero@redhat.com)
- Remove sudo call from hack/test-cmd.sh (jliggitt@redhat.com)
- Omit needless reconciles (jliggitt@redhat.com)
- allow project request limits on system users and service accounts
  (deads@redhat.com)
- Allow pulling base image by default in dockerbuild (mfojtik@redhat.com)
- Store author when using MAINTAINER instruction (mfojtik@redhat.com)
- Show newlines and links in template descriptions (spadgett@redhat.com)
- Display FROM instruction (mfojtik@redhat.com)
- Disable UI scaling for in progress deployment (spadgett@redhat.com)
- Bug fix to remove empty <tbody> results in Firefox border rendering bug
  (rhamilto@redhat.com)
- refactor webhooks to account for multiple sources (ipalade@redhat.com)
- point users to the jenkins tutorial from the template (bparees@redhat.com)
- add oc create dc (deads@redhat.com)
- put the git ls-remote "--head" argument in the right order (before the url)
  (bparees@redhat.com)
- Exclude type fields from conversion (ccoleman@redhat.com)
- Regenerate image api deep copies (ccoleman@redhat.com)
- Don't use external types in our serialized structs (ccoleman@redhat.com)
- Regenerated copies and conversions (ccoleman@redhat.com)
- Switch to upstream generators (ccoleman@redhat.com)
- UPSTREAM: 25033: Make conversion gen downstream consumable
  (ccoleman@redhat.com)
- Add support for resource owner password grant (jliggitt@redhat.com)
- Fix timing issue scaling deployments (spadgett@redhat.com)
- UPSTREAM: 24924: fix PrepareForUpdate bug for HPA (jliggitt@redhat.com)
- UPSTREAM: 24924: fix PrepareForUpdate bug for PV and PVC
  (jliggitt@redhat.com)
- Use copy-to-clipboard for next steps webhook URL (spadgett@redhat.com)
- BZ 1332876 - Break long Git source URL (jhadvig@redhat.com)
- stop resending modifies with same project resourceversion for watch
  (deads@redhat.com)
- Allow network test runner to use kubeconfig (marun@redhat.com)
- Replace mongostat with mongo in mongodb example template (mfojtik@redhat.com)
- Fix a problem with ignored Dockerfiles in db conformance
  (ccoleman@redhat.com)
- Add tests and properly handle directory merging (ccoleman@redhat.com)
- Return error from exec (ccoleman@redhat.com)
- update to match s2i type renames (bparees@redhat.com)
- bump(github.com/RangelReale/osincli):
  05659f31e8b694f522f44226839a66bd8b7c08cct (jliggitt@redhat.com)
- support project watch resourceversion=0 (deads@redhat.com)
- bump(github.com/openshift/source-to-image):
  c99ef8b29e94bdaf62d5d5aefa74d12af6d37e3c (bparees@redhat.com)
- Increase initialDelay for Jenkins livenessprobe (mfojtik@redhat.com)
- add configmap to oc set volume (deads@redhat.com)
- Add extended network tests for default namespace non-isolation
  (danw@redhat.com)
- bump(github.com/openshift/source-to-image):
  528d0e97ac38354621520890878d7ea34451384b (bparees@redhat.com)
- Update data.js to support fieldSelector & labelSelector
  (admin@benjaminapetersen.me)
- BZ 1332787 Cannot create resources defined in List through From File option
  on create page (jhadvig@redhat.com)
- Skip pulp test in integration (ccoleman@redhat.com)
- Add output logging to docker build and correctly pull (ccoleman@redhat.com)
- Enable recursion by default for start commands (ccoleman@redhat.com)
- DNS resolution must return etcd name errors (ccoleman@redhat.com)
- Add DNS to the conformance suite (ccoleman@redhat.com)
- Add expanded DNS e2e tests for Origin (ccoleman@redhat.com)
- UPSTREAM: 24128: Allow cluster DNS verification to be overriden
  (ccoleman@redhat.com)
- Additional web console updates for JenkinsPipeline strategy
  (spadgett@redhat.com)
- Reverting mountPath validation for add to allow overwriting volume without
  specifying mountPath (ewolinet@redhat.com)
- Remove moment timezone (jliggitt@redhat.com)
- Add the ability to configure an HAPROXY router to send log messages to a
  syslog address (jtanenba@redhat.com)
- [RPMS] Improve default permissions (sdodson@redhat.com)
- show pod IP during debug (deads@redhat.com)
- Add the service load balancer controller to Origin (ccoleman@redhat.com)
- Add create helpers for users, identities, and mappings from the CLI
  (jliggitt@redhat.com)
- Update editor mode before checking error annotations (spadgett@redhat.com)
- default the volume type (deads@redhat.com)
- Accessibility: make enter key work when confirming project name
  (spadgett@redhat.com)
- let registry-admin delete his project (deads@redhat.com)
- Upload component during the create flow (jhadvig@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  c709df994a5bbecfa80fae62c20131cd25337b79 (rpenta@redhat.com)
- Improve oc cancel-build command (mfojtik@redhat.com)
- dind: stop attempting to cache images during build (marun@redhat.com)
- added oc status check for hpa missing cpu request (skuznets@redhat.com)
- Fix for issue 8578 involving non-breaking strings. (sgoodwin@redhat.com)
- Update yaml.js to latest, official version (spadgett@redhat.com)
- add what-can-i-do endpoint (deads@redhat.com)
- Allow to specify the run policy for builds (mfojtik@redhat.com)
- oc: remove dead factory code (mkargaki@redhat.com)
- oc: dont warn for missing istag in case of in-flight build
  (mkargaki@redhat.com)
- Fix openshift-start empty hostname error message (ripcurld.github@gmail.com)
- oc: support multiple resources in rsh (mkargaki@redhat.com)
- UPSTREAM: 23590: kubectl: more sophisticated pod selection for logs and
  attach (mkargaki@redhat.com)
- UPSTREAM: <drop>: watch-based wait utility (mkargaki@redhat.com)
- Fix broken tests, cleanup config (ccoleman@redhat.com)
- UPSTREAM: <carry>: force import ordering for stable codegen
  (deads@redhat.com)
- Instantiate Jenkins when pipeline strategy is created (jimmidyson@gmail.com)
- Add JenkinsPipeline build strategy (jimmidyson@gmail.com)
- Review 1 (ccoleman@redhat.com)
- Change hack to use direct docker build (ccoleman@redhat.com)
- Flexible dockerfile builder that uses client calls (ccoleman@redhat.com)
- Increase default web console line limit to 5000 (spadgett@redhat.com)
- Allow parameters on generic webhook build trigger call (gmontero@redhat.com)
- prevent s2i from running onbuild images (bparees@redhat.com)
- bump(github.com/openshift/source-to-image):
  849ad2f6204f7bc0c3522fc59ec4d3c6d653db7c (bparees@redhat.com)
- Enable the gcs, oss, and inmemory storage drivers in registry
  (ccoleman@redhat.com)
- Fix bootstrap package name spelling (cewong@redhat.com)
- make project cache watchable (deads@redhat.com)
- setup default impersonation rules (deads@redhat.com)
- blacklist the ThirdPartyResource kind from other resources page
  (jforrest@redhat.com)
- Do not stop tests (agladkov@redhat.com)
- Generate bindata for bootstrap streams, templates and quickstarts
  (cewong@redhat.com)
- Template parameters broken in new-app (ccoleman@redhat.com)
- integration tests: wait for admission control cache to allow pod creation
  (cewong@redhat.com)
- [RPMS] Get rid of openshift-recycle subpackage (#18) (sdodson@redhat.com)
- [RPMS] Add openshift-recycle (#17) (sdodson@redhat.com)
- bump(k8s.io/kubernetes): 61d1935e4fbaf5b9dd43ebd491dd33fa0aeb48a0
  (deads@redhat.com)
- UPSTREAM: 24839: Validate deletion timestamp doesn't change on update
  (jliggitt@redhat.com)
- add acting-as request handling (deads@redhat.com)
- Handle null items array from bad api responses (jforrest@redhat.com)
- UPSTREAM: 23549: add act-as powers (deads@redhat.com)
- Use git shallow clone when not using ref (mfojtik@redhat.com)
- Correctly use netcat (ccoleman@redhat.com)
- Add lsof (ccoleman@redhat.com)
- Label and streamline the remaining images (ccoleman@redhat.com)
- Simplify the router and ipfailover images (ccoleman@redhat.com)
- Move recycler into origin binary (ccoleman@redhat.com)
- UPSTREAM: 24052: Rate limiting requeue (deads@redhat.com)
- UPSTREAM: 23444: add a delayed queueing option to the workqueue
  (deads@redhat.com)
- add default resource requests to router creation (jtanenba@redhat.com)
- bump(github.com/openshift/source-to-image):
  caa5fed4d6c38041ec1396661b66faff4eff358d (bparees@redhat.com)
- Increase deployment test fixture timeout (ironcladlou@gmail.com)
- remove admission plugins from chain when they have an nil config
  (deads@redhat.com)
- UPSTREAM: 24421: remove admission plugins from chain (deads@redhat.com)
- Refactors to generation in preparation for compose (ccoleman@redhat.com)
- Add openshift.io/deployer-pod.type label (agladkov@redhat.com)
- fix godeps (bparees@redhat.com)
- remove describe user function (ipalade@redhat.com)
- Update integration tests to confirm project name when deleting
  (spadgett@redhat.com)
- remove old double escape validation (pweil@redhat.com)
- Make --insecure flag take precedence over insecure annotation
  (maszulik@redhat.com)
- introduce --watch capability for oc rsync (bparees@redhat.com)
- bump(github.com/fsnotify/fsnotify): 3c39c22b2c7b0516d5f2553f1608e5d13cb19053
  (bparees@redhat.com)
- Read insecure annotation on an image stream when import-image is invoked.
  (maszulik@redhat.com)
- Remove port from request's Host header (elyscape@gmail.com)
- Fix failing application generator test (spadgett@redhat.com)
- warn if image is not a builder, even if it came from source detection
  (bparees@redhat.com)
- Bindata for the console 1.3 changes to date and fixes to spec tests
  (jforrest@redhat.com)
- Make bulk output more centralized (ccoleman@redhat.com)
- Refer SDN plugin names by constants (rpenta@redhat.com)
- fix bad tabbing in template (bparees@redhat.com)
- Fixes to bower after rebase (jforrest@redhat.com)
- Project list add user to role message shouldnt hardcode admin role
  (jforrest@redhat.com)
- Don't validate limit against request limit range default
  (spadgett@redhat.com)
- Trigger login flow if api discovery failed because of possible cert errors
  (jforrest@redhat.com)
- Changing notifications to alerts (rhamilto@redhat.com)
- Lock matchHeight and temporarily add resolution to fix jenkins issue
  (jforrest@redhat.com)
- Tolarate no input buildConfig in console (jhadvig@redhat.com)
- Web console support for JenkinsPipeline builds (spadgett@redhat.com)
- Enabling WebHooks from console will add Generic webhook (jhadvig@redhat.com)
- Add an 'Other resources' page to view, raw edit, and delete other stuff in
  your project (jforrest@redhat.com)
- Changing copy to clipboard button to an input + button (rhamilto@redhat.com)
- Confirm project delete by typing project name (spadgett@redhat.com)
- Update humanizeKind filter to use lowercase by default (spadgett@redhat.com)
- Edit routes in web console (spadgett@redhat.com)
- API discovery in the console (jforrest@redhat.com)
- Reintroduce :empty selector to hide online extensions but adds as a util
  class in _util.scss (admin@benjaminapetersen.me)
- Handle multiline command arguments in web console (spadgett@redhat.com)
- Update nav-mobile-dropdown to use new extension-point, eliminate hawtio-
  extension (admin@benjaminapetersen.me)
- Metrics updates (spadgett@redhat.com)
- Fix usage rate calculation for network metrics in pod page
  (ffranz@redhat.com)
- Fix httpGet path in pod template (spadgett@redhat.com)
- Warn users when navigating away with the terminal window open
  (spadgett@redhat.com)
- Add network metrics to pod page on console (ffranz@redhat.com)
- Fix UI e2e test failure (spadgett@redhat.com)
- Add _spacers.less to generate pad/mar utility classes
  (admin@benjaminapetersen.me)
- Add container entrypoint to debug dialog (spadgett@redhat.com)
- Web console: debug terminal for crashing pods (spadgett@redhat.com)
- Exclude google-code-prettify (spadgett@redhat.com)
- Left align tabs when they wrap to second line (spadgett@redhat.com)
- DataService._urlForResource should return a string (spadgett@redhat.com)
- Fix jshint warnings (spadgett@redhat.com)
- Update grunt-contrib-jshintrc to 1.0.0 (spadgett@redhat.com)
- Fix failing web console unit tests (spadgett@redhat.com)
- Web console support for health checks (spadgett@redhat.com)
- Update online extensions, avoid rendering emtpy DOM nodes
  (admin@benjaminapetersen.me)
- Use hawtioPluginLoader to load javaLinkExtension (admin@benjaminapetersen.me)
- Support autoscaling in the web console (spadgett@redhat.com)
- Update extension points from hawtio manager to angular-extension-registry
  (admin@benjaminapetersen.me)
- Don't enable editor save button until after a change (spadgett@redhat.com)
- Improving pod-template visualization (rhamilto@redhat.com)
- Use @dl-horizontal-breakpoint (spadgett@redhat.com)
- Update Patternfly utilization chart class name (spadgett@redhat.com)
- Updating PatternFly and Angular-PatternFly to v3.3.0 (rhamilto@redhat.com)
- Remove angular-patternfly utilization card dependency (spadgett@redhat.com)
- tests: Bind DNS only to API_HOST address (sgallagh@redhat.com)
- Add mount debug information for cpu quota test flake (mrunalp@gmail.com)
- Handle client-side errors on debug pod creation (ffranz@redhat.com)
- make new project wait for rolebinding cache before returning
  (deads@redhat.com)
- cite potential builds for oc status missing input streams
  (gmontero@redhat.com)
- allow for no build config source input (defer assemble script or custom
  builder) (gmontero@redhat.com)
- add oc create policybinding (deads@redhat.com)
- wait for authorization cache to update before pruning (deads@redhat.com)
- UPSTREAM: <carry>: add scc to pod describer (screeley@redhat.com)
- add option to reverse buildchains (deads@redhat.com)
- switch new project example repo to one that doesn't need a db
  (bparees@redhat.com)
- fully qualify admission attribute records (deads@redhat.com)
- UPSTREAM: 24601: fully qualify admission resources and kinds
  (deads@redhat.com)
- Update generated public functions (ccoleman@redhat.com)
- Make hand written conversions public (ccoleman@redhat.com)
- do not force drop KILL cap on anyuid SCC (pweil@redhat.com)
- UPSTREAM: 24382: RateLimitedQueue TestTryOrdering could fail under load
  (ccoleman@redhat.com)
- generated files (deads@redhat.com)
- important: etcd initialization change (deads@redhat.com)
- important: previously bugged: authorization adapter (deads@redhat.com)
- fix rebase of limitranger for clusterresourceoverride (lmeyer@redhat.com)
- important: resource enablement (deads@redhat.com)
- disabled kubectl features: third-party resources and recursive directories
  (deads@redhat.com)
- rebase interesting enough to keep diff-able (deads@redhat.com)
- <drop>: disable etcd3 unit tests (deads@redhat.com)
- boring changes (deads@redhat.com)
- UPSTREAM: <drop>: keep old deep copy generator for now (deads@redhat.com)
- UPSTREAM: 24208: Honor starting resourceVersion in watch cache
  (agoldste@redhat.com)
- UPSTREAM: 24153: make optional generic etcd fields optional
  (deads@redhat.com)
- UPSTREAM: 23894: Should not fail containers on OOM score adjust
  (ccoleman@redhat.com)
- UPSTREAM: 24048: Use correct defaults when binding apiserver flags
  (jliggitt@redhat.com)
- UPSTREAM: 24008: Make watch cache behave like uncached watch
  (jliggitt@redhat.com)
- UPSTREAM: openshift/openshift-sdn: <drop>: sig changes (deads@redhat.com)
- bump(hashicorp/golang-lru): a0d98a5f288019575c6d1f4bb1573fef2d1fcdc4
  (deads@redhat.com)
- bump(davecgh/go-spew): 5215b55f46b2b919f50a1df0eaa5886afe4e3b3d
  (deads@redhat.com)
- bump(rackspace/gophercloud): 8992d7483a06748dea706e4716d042a4a9e73918
  (deads@redhat.com)
- bump(coreos/etcd): 5e6eb7e19d6385adfabb1f1caea03e732f9348ad
  (deads@redhat.com)
- bump(k8s.io/kubernetes): 61d1935e4fbaf5b9dd43ebd491dd33fa0aeb48a0
  (deads@redhat.com)
- pass args to bump describer (deads@redhat.com)
- UPSTREAM: revert: 1f9b8e5: 24008: Make watch cache behave like uncached watch
  (deads@redhat.com)
- UPSTREAM: revert: c3d1d37: 24048: Use correct defaults when binding apiserver
  flags (deads@redhat.com)
- UPSTREAM: revert: 9ec799d: 23894: Should not fail containers on OOM score
  adjust (deads@redhat.com)
- UPSTREAM: revert: 66d1fdd: 24208: Honor starting resourceVersion in watch
  cache (deads@redhat.com)
- diagnostics: update cmd tests (lmeyer@redhat.com)
- Debug command should default to pods (ccoleman@redhat.com)
- Support --logspec (glog -vmodule) for finegrained logging
  (ccoleman@redhat.com)
- diagnostics: introduce cluster ServiceExternalIPs (lmeyer@redhat.com)
- diagnostics: introduce cluster MetricsApiProxy (lmeyer@redhat.com)
- Output logs on network e2e deployment failure (marun@redhat.com)
- Import from repository doesn't panic (miminar@redhat.com)
- Default role reconciliation to additive-only (jliggitt@redhat.com)
- made commitchecker error statements more verbose (skuznets@redhat.com)
- Remove deprecated --nodes flag (jliggitt@redhat.com)
- Improve logging of missing authorization codes (jliggitt@redhat.com)
- git server: automatically launch builds on push (cewong@redhat.com)
- fix typo in error message (pweil@redhat.com)
- update registry config with deprecated storage.cache.layerinfo, now
  blobdescriptor (aweiteka@redhat.com)
- implemented glog wrapper for use in delegated commands (skuznets@redhat.com)
- UPSTREAM: golang/glog: <carry>: add 'InfoDepth' to 'V' guard
  (skuznets@redhat.com)
- update failure conditions for oc process (skuznets@redhat.com)
- remove unused constant that has been migrated to bootstrappolicy
  (pweil@redhat.com)
- dind: output rc reminder if bin path is not set (marun@redhat.com)
- dind: clean up disabling of sdn node (marun@redhat.com)
- dind: ensure quoting of substitutions (marun@redhat.com)
- dind: use os::build funcs to find binary path (marun@redhat.com)
- dind: ensure all conditionals use [[ (marun@redhat.com)
- least change to stop starting reflector in clusterresourceoverride
  (deads@redhat.com)
- UPSTREAM: 24403: kubectl: use platform-agnostic helper in edit
  (mkargaki@redhat.com)
- Allow new-app params to be templatized (ccoleman@redhat.com)
- Change the context being used when Stat-ing remote registry
  (maszulik@redhat.com)
- Revert SDN bridge-nf-call-iptables=0 hack (dcbw@redhat.com)
- Update README.md (brian.christner@gmail.com)
- add labels and annotations from buildrequest to resulting build
  (bparees@redhat.com)
- Don't close the "lost" channel more than once (ccoleman@redhat.com)
- Expose Watch in the Users client API (v.behar@free.fr)
- Expose Watch in the Groups client API (v.behar@free.fr)
- oc: fix help message for deploy (mkargaki@redhat.com)
- Disable serial image pulls by default (avagarwa@redhat.com)
- docs: add irc badge (jsvgoncalves@gmail.com)
- improve systemd unit ordering (jdetiber@redhat.com)
- F5: handle HTML responses gracefully (miciah.masters@gmail.com)
- correct precision in the no config file message (jay@apache.org)

* Mon Jun 20 2016 Scott Dodson <sdodson@redhat.com> 3.2.1.3
- add mutation cache (deads@redhat.com)
- convert dockercfg secret generator to a work queue (deads@redhat.com)
- UPSTREAM: 25091: partial - reduce conflict retries (deads@redhat.com)
- Add bindata.go to pick up changes merged in
  https://github.com/openshift/ose/pull/213 which addressed bug 1328016.
  (sgoodwin@redhat.com)

* Tue Jun 14 2016 Scott Dodson <sdodson@redhat.com> 3.2.1.2
- UPSTREAM: 27227: Counting pod volume towards PV limit even if PV/PVC is
  missing (abhgupta@redhat.com)
- UPSTREAM 22568: Considering all nodes for the scheduler cache to allow
  lookups (abhgupta@redhat.com)
- Use our golang-1.4 build image in OSE (ccoleman@redhat.com)
- Add a GCS test to verify it is compiled in (ccoleman@redhat.com)
- Build dockerregistry with tag `include_gcs` (ccoleman@redhat.com)
- UPSTREAM: docker/distribution: <carry>: Distribution dependencies
  (ccoleman@redhat.com)
- Upstream PR8615 - haproxy Cookie id leaks info (pcameron@redhat.com)

* Sat Jun 04 2016 Scott Dodson <sdodson@redhat.com> 3.2.1.1
- UPSTREAM 8287 Suppress router reload during sync (marun@redhat.com)
- UPSTREAM 8893 Sync access to router state (smitram@gmail.com)
- UPSTREAM: 25487: pod constraints func for quota validates resources
  (decarr@redhat.com)
- UPSTREAM: 24514: Quota ignores pod compute resources on updates
  (decarr@redhat.com)
- Skip registry client v1 and v2 tests until we push with 1.10
  (ccoleman@redhat.com)
- Allow size of image to be zero when schema1 from Hub (ccoleman@redhat.com)
- UPSTREAM: 25161: Sort resources in quota errors to avoid duplicate events
  (decarr@redhat.com)
- Bug 1342091 - Add additional interfaces implementation for handling watches,
  rsh, etc. (maszulik@redhat.com)
- Add watch caching to origin types (jliggitt@redhat.com)
- Fix login redirect (spadgett@redhat.com)
- UPSTREAM: 24403: kubectl: use platform-agnostic helper in edit
  (mkargaki@redhat.com)
- Backport bug #1333118: adding labels and envs is tricky
  (admin@benjaminapetersen.me)
- Document things, remove /usr/bin/docker mount from contrib systemd unit
  (sdodson@redhat.com)
- chroot docker to the rootfs (sdodson@redhat.com)
- Truncate labels of builds generated from a BuildConfig (cewong@redhat.com)
- Truncate build pod label to allowed size (cewong@redhat.com)
- Service account token controller startup change (jliggitt@redhat.com)
- [RPMs] Refactor golang BuildRequires (sdodson@redhat.com)
- UPSTREAM: 23858: Convert service account token controller to use a work queue
  (jliggitt@redhat.com)
- UPSTREAM: 24052: add built-in ratelimiter to workqueue (deads@redhat.com)
- UPSTREAM: 23444: fake util.clock tick (jliggitt@redhat.com)
- UPSTREAM: 23444: make delayed workqueue use channels with single writer
  (jliggitt@redhat.com)
- UPSTREAM: 23444: add a delayed queueing option to the workqueue
  (deads@redhat.com)
- Fix extended validation test expected error count. Probably needs to do error
  check rather than exact error count match. (smitram@gmail.com)
-   o Add basic validation for route TLS configuration - checks that     input
  is "syntactically" valid.   o Checkpoint initial code.   o Add support for
  validating route tls config.   o Add option for validating route tls config.
  o Validation fixes.   o Check private key + cert mismatches.   o Add tests.
  o Record route rejection.   o Hook into add route processing + store invalid
  service alias configs     in another place - easy to check prior errors on
  readmission.   o Remove entry from invalid service alias configs upon route
  removal.   o Add generated completions.   o Bug fixes.   o Recording
  rejecting routes is not working completely.   o Fix status update problem -
  we should set the status to admitted     only if we had no errors handling a
  route.   o Rework to use a new controller - extended_validator as per
  @smarterclayton comments.   o Cleanup validation as per @liggitt comments.
  o Update bash completions.   o Fixup older validation unit tests.   o Changes
  as per @liggitt review comments + cleanup tests.   o Fix failing test.
  (smitram@gmail.com)
- Bug 1334485 - Empty overview page (jhadvig@redhat.com)
- UPSTREAM: 25907: Fix detection of docker cgroup on RHEL (agoldste@redhat.com)
- UPSTREAM: 25472: tolerate nil error in HandleError (deads@redhat.com)
- UPSTREAM: 25690: Fixes panic on round tripper when TLS under a proxy
  (ffranz@redhat.com)
- platformmanagement_public_704: Added basic auditing (maszulik@redhat.com)
- Bug 1333118 - Make for CLI tools a standalone help page (jhadvig@redhat.com)
- allow project request limits on system users and service accounts
  (deads@redhat.com)
- Bug 1333172 - differ between route hostname and navigation within the console
  (jhadvig@redhat.com)
- Updating Help > Documentation links to use helpLink filter
  (rhamilto@redhat.com)
- UPSTREAM: 25501: SplitHostPort is needed since Request.RemoteAddr has the
  host:port format (maszulik@redhat.com)
- Remove gaps from build chart for deleted builds (spadgett@redhat.com)
- Check for pod creation in scc exec admission (jliggitt@redhat.com)
- Include ext-searchbox.js for Ace editor search (spadgett@redhat.com)
- UPSTREAM: 25369: Return 'too old' errors from watch cache via watch stream
  (jliggitt@redhat.com)
- Update logo and support css structure to enable a more flexible dimensions
  (sgoodwin@redhat.com)
- Calculate effective limit min in UI for request/limit ratio
  (spadgett@redhat.com)
- Disable UI scaling for in progress deployment (spadgett@redhat.com)
- Fix timing issue scaling deployments (spadgett@redhat.com)
- Show newlines and links in template descriptions (spadgett@redhat.com)
- UPSTREAM: 25077: PLEG: reinspect pods that failed prior inspections
  (agoldste@redhat.com)
- Update spec to 4.2.1 (tdawson@redhat.com)

* Thu May 05 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.42
- UPSTREAM: 24924: fix PrepareForUpdate bug for HPA (jliggitt@redhat.com)
- UPSTREAM: 24924: fix PrepareForUpdate bug for PV and PVC
  (jliggitt@redhat.com)
- enforce scc during pod updates (deads@redhat.com)
- bump(github.com/openshift/source-to-image):
  528d0e97ac38354621520890878d7ea34451384b (bparees@redhat.com)
- bump(github.com/openshift/source-to-image):
  849ad2f6204f7bc0c3522fc59ec4d3c6d653db7c (bparees@redhat.com)
- prevent s2i from running onbuild images (bparees@redhat.com)

* Tue May 03 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.41
- UPSTREAM: 24933: Validate deletion timestamp doesn't change on update
  (jliggitt@redhat.com)
- policy unsafe proxy requests separately (jliggitt@redhat.com)
- Add login csrf prompts (jliggitt@redhat.com)
- Enable the gcs, oss, and inmemory storage drivers in registry
  (ccoleman@redhat.com)
- Limit queryparam auth to websockets (jliggitt@redhat.com)
- Web console: don't validate limit against request default
  (spadgett@redhat.com)
- Bug 1330364 - add user to role message has 'admin' role hardcoded, should
  just have a placeholder (jforrest@redhat.com)

* Tue Apr 26 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.40
- Update version for first hotfix (tdawson@redhat.com)
- Merge pull request #184 from ironcladlou/deployment-quota-fix
   Prevent deployer pod creation conflicts

* Mon Apr 25 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.20
- bump(github.com/openshift/openshift-sdn)
  ba3087afd66cce7c7d918af10ad91197f8dfd74f (rpenta@redhat.com)
- Open() is called in the pullthrough code path and needs retry
  (ccoleman@redhat.com)
- Improve parsing of semantic version for git tag (ccoleman@redhat.com)
- Prevent deployer pod creation conflicts (ironcladlou@gmail.com)
- Wait until user has access to project in extended (ccoleman@redhat.com)
- Retry 401 unauthorized responses from registries within a window
  (ccoleman@redhat.com)

* Fri Apr 22 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.19
- Support extracting release binaries on other platforms (ccoleman@redhat.com)
- When Kube and Origin version are the same, skip test (ccoleman@redhat.com)
- All image references should be using full semantic version
  (ccoleman@redhat.com)
- Simplified and extended jobs tests (maszulik@redhat.com)
- debug for ext test failures on jenkins (gmontero@redhat.com)
- Fix branding in html title for oauth pages (jforrest@redhat.com)
- force pull fixes / debug (gmontero@redhat.com)
- Make /etc/origin /etc/origin/master /etc/origin/node 0700
  (sdodson@redhat.com)

* Wed Apr 20 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.18
- Revert "Retry import to the docker hub on 401" (ccoleman@redhat.com)
- debug for failures on jenkins (gmontero@redhat.com)
- Change the context being used when Stat-ing remote registry
  (maszulik@redhat.com)
- Update 1 - run to the entire interval (ccoleman@redhat.com)
- Retry 401 unauthorized responses from registries within a window
  (ccoleman@redhat.com)
- Exclude deployment tests and set a perf test to serial (ccoleman@redhat.com)
- Add client debugging for legacy dockerregistry code (ccoleman@redhat.com)
- moved jUnit report for test-cmd to be consistent (skuznets@redhat.com)
- remove trailing = from set trigger example (bparees@redhat.com)
- haproxy obfuscated internal IP in routing cookie (pcameron@redhat.com)
- Example CLI command (ccoleman@redhat.com)
- Add summary status message to network test runner (marun@redhat.com)
- dind: Clean up check for ready nodes (marun@redhat.com)
- Fix precision of cpu to millicore and memory to bytes (decarr@redhat.com)
- UPSTREAM: 23435: Fixed mounting with containerized kubelet
  (jsafrane@redhat.com)

* Mon Apr 18 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.17
- Revert "oc status must show monopods" (ccoleman@redhat.com)
- Refactor build strategy permissions into distinct roles (jliggitt@redhat.com)
- Cleanup network test runner's use of conditionals (marun@redhat.com)
- Copy config to dev cluster master's local storage (marun@redhat.com)
- Cleanup network test runner error handling (marun@redhat.com)
- Minimize image builds by the network test runner (marun@redhat.com)
- Improve network test runner error handling (marun@redhat.com)
- Reduce log verbosity of network test runner (marun@redhat.com)
- Remove redundant extended networking sanity tests (marun@redhat.com)
- [RPMS] Switch requires from docker-io to docker (sdodson@redhat.com)
- extended: Allow to focus on particular tests (miminar@redhat.com)
- Ensure stable route admission (marun@redhat.com)
- Added provisioning flag and refactored VolumeConfig struct names
  (mturansk@redhat.com)
- UPSTREAM: 23793: Make ConfigMap volume readable as non-root
  (pmorie@gmail.com)
- added jUnit XML publishing step to exit trap (skuznets@redhat.com)
- added support for os::cmd parsing to junitreport (skuznets@redhat.com)
- refactored nested suites builder (skuznets@redhat.com)
- configure test-cmd to emit parsable output for jUnit (skuznets@redhat.com)

* Fri Apr 15 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.16
- UPSTREAM: 24208: Honor starting resourceVersion in watch cache
  (agoldste@redhat.com)
- Abuse deployments with extended test (ccoleman@redhat.com)
- dind: only wait for Ready non-sdn nodes (marun@redhat.com)
- UPSTREAM: 23769: Ensure volume GetCloudProvider code uses cloud config
  (jliggitt@redhat.com)
- UPSTREAM: 21140: e2e test for dynamic provisioning. (jliggitt@redhat.com)
- UPSTREAM: 23463: add an event for when a daemonset can't place a pod due to
  insufficent resource or port conflict (jliggitt@redhat.com)
- UPSTREAM: 23929: only include running and pending pods in daemonset should
  place calculation (jliggitt@redhat.com)
- Update font-awesome bower dependency (spadgett@redhat.com)
- Fix unit tests (ironcladlou@gmail.com)
- Prevent concurrent deployer pod creation (ironcladlou@gmail.com)
- Enforce overview scaling rules for deployments without a service
  (spadgett@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  9f1f60258fcef6f0ef647a75a8754bc80779c065 (rpenta@redhat.com)

* Wed Apr 13 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.15
- Update atomic registry quickstart artifacts (aweiteka@redhat.com)
- UPSTREAM: 23746: A pod never terminated if a container image registry was
  unavailable (agoldste@redhat.com)
- fix push path in log message (bparees@redhat.com)
- Bump fontawesome to 4.6.1 (jforrest@redhat.com)
- Fix bindata diff for new font-awesome release (spadgett@redhat.com)
- Fix e2e scc race (agoldste@redhat.com)
- UPSTREAM: 23894: Should not fail containers on OOM score adjust
  (ccoleman@redhat.com)
- Add ability to specify allowed CNs for RequestHeader proxy client cert
  (jliggitt@redhat.com)
- bump(k8s.io/kubernetes): 114a51dfbc43a8bcf07db1774a20e05d560e34b0
  (deads@redhat.com)
- Update ose image build scripts (tdawson@redhat.com)
- Modify pullthrough import-image use case to use our cli framework
  (maszulik@redhat.com)
- add system:image-auditor (deads@redhat.com)
- Improve wording in sample-app README (rhcarvalho@gmail.com)
- Wrap login requests to clear in-memory session (jliggitt@redhat.com)
- Grant use of the NFS plugin in hostmount-anyuid SCC, to enable NFS recycler
  pods to run (pmorie@gmail.com)
- made hack/test-go handle compilation errors better (skuznets@redhat.com)
- Enable etcd cache for k8s resources (jliggitt@redhat.com)
- UPSTREAM: 24048: Use correct defaults when binding apiserver flags
  (jliggitt@redhat.com)
- UPSTREAM: 24008: Make watch cache behave like uncached watch
  (jliggitt@redhat.com)

* Mon Apr 11 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.14
- deployer controller: ensure phase direction (mkargaki@redhat.com)
- Getting docker logs always to debug issue 8399 (maszulik@redhat.com)
- deployment controller: cancel deployers on new cancelled deployments
  (mkargaki@redhat.com)
- Bug 1324437: show cancel as subordinate to non-terminating phases
  (mkargaki@redhat.com)

* Fri Apr 08 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.13
- Travis can't install go vet anymore (ccoleman@redhat.com)
- Add debugs for CPU Quota flake (mrunalp@gmail.com)
- Getting docker logs always to debug issue 8399 (maszulik@redhat.com)
- ProjectRequestLimit plugin: ignore projects in terminating state
  (cewong@redhat.com)
- Support focus arguments on both conformance and core (ccoleman@redhat.com)
- BZ_1324273: Make BC edit form dirty when deleting Key-Value pair
  (jhadvig@redhat.com)
- Add a conformance test for extended (ccoleman@redhat.com)
- Always set the Cmd for source Docker image (mfojtik@redhat.com)
- stylistic/refactoring changes for test/extended/cmd to use os::cmd
  (skuznets@redhat.com)
- UPSTREAM: 23445: Update port forward e2e for go 1.6 (agoldste@redhat.com)
- Naming inconsistancy fix (jhadvig@redhat.com)
- install iproute since the origin-base image dose not contain ip command
  (bmeng@redhat.com)
- Change RunOnce duration plugin to act as a limit instead of override
  (cewong@redhat.com)
- Error on node startup using perFSGroup quota, but fs mounted with noquota.
  (dgoodwin@redhat.com)
- remove running containers in case of a sigterm (bparees@redhat.com)
- Add mounting volumes to privileged pods template (jcope@redhat.com)
- oc describe build: improve output. (vsemushi@redhat.com)

* Wed Apr 06 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.12
- Make infra and shared resource namespaces immortal (jliggitt@redhat.com)
- UPSTREAM: 23883: Externalize immortal namespaces (jliggitt@redhat.com)
- add conformance tag to some extended build tests (bparees@redhat.com)
- add slow tag to jenkins plugin extended test (gmontero@redhat.com)
- Bug 1323710: fix deploy --cancel message on subsequent calls
  (mkargaki@redhat.com)
- UPSTREAM: 23548: Check claimRef UID when processing a recycled PV, take 2
  (swagiaal@redhat.com)
- Updates to the 503 application unavailable page. (sgoodwin@redhat.com)
- Improve display of logs in web console (spadgett@redhat.com)
- bump(github.com/openshift/source-to-image):
  641b22d0a5e7a77f7dab2b1e75f563ba59a4ec96 (rhcarvalho@gmail.com)
- elevate privilegs when removing etcd binary store (skuznets@redhat.com)
- UPSTREAM: 23078: Check claimRef UID when processing a recycled PV
  (swagiaal@redhat.com)

* Mon Apr 04 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.11
- Show pulling / terminated status in pods donut (spadgett@redhat.com)
- Update the postCommit hook godoc to reflect API (ccoleman@redhat.com)
- do not error on adding app label to objects if it exists (bparees@redhat.com)
- Add the openshift/origin-egress-router image (danw@redhat.com)
- remove credentials arg (aweiteka@redhat.com)
- remove atomic registry quickstart from images dir, also hack test script
  (aweiteka@redhat.com)
- Retry when receiving an imagestreamtag not found error (ccoleman@redhat.com)

* Fri Apr 01 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.10
- Allow multiple routers to update route status (sross@redhat.com)
- change default volume size (gmontero@redhat.com)
- Bug 1322587 - NotFound error (404) when deleting layers is logged but we'll
  be continuing the execution. (maszulik@redhat.com)
- Separate out new-app code for reuse (ccoleman@redhat.com)
- Refactor new app and new build to use options struct (ccoleman@redhat.com)
- use restart sec to avoid default rate limit (pweil@redhat.com)
- Refactor start-build to use options style (ccoleman@redhat.com)
- Tolerate local Git repositories without an origin set (ccoleman@redhat.com)
- Add unique suffix to build post-hook containers (rhcarvalho@gmail.com)
- The router command should keep support for hostPort (ccoleman@redhat.com)
- UPSTREAM: 23586: don't sync deployment when pod selector is empty
  (jliggitt@redhat.com)
- UPSTREAM: 23586: validate that daemonsets don't have empty selectors on
  creation (jliggitt@redhat.com)
- no commit id in img name and add openshift org to image name for openshift-
  pipeline plugin extended test (gmontero@redhat.com)
- Set service account correctly in oadm registry, deprecate --credentials
  (jliggitt@redhat.com)
- Add tests for multiple IDPs (sgallagh@redhat.com)
- Encode provider name when redirecting to login page (jliggitt@redhat.com)
- Allow multiple web login methods (sgallagh@redhat.com)
- allow pvc by default (pweil@redhat.com)
- UPSTREAM: 23007: Kubectl shouldn't print throttling debug output
  (jliggitt@redhat.com)
- Resolve api groups in resolveresource (jliggitt@redhat.com)
- Improve provider selection page (jliggitt@redhat.com)
- fix a forgotten modification : 'sti->s2i' (qilin.wang@huawei.com)
- sort volumes for reconciliation (pweil@redhat.com)
- Set charset with content type (jliggitt@redhat.com)
- Bug 1318920: emit events for failed cancellations (mkargaki@redhat.com)
- UPSTREAM: 22525: Add e2e for remaining quota resources (decarr@redhat.com)
- remove test file cruft (skuznets@redhat.com)

* Wed Mar 30 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.9
- Fixed string formatting for glog.Infof in image prunning
  (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image):
  48e62fd57bebba14e1d0f7a40a15b65dafa5458c (cewong@redhat.com)
- Fix 8162: project settings layout issues (admin@benjaminapetersen.me)
- UPSTREAM: 23456: don't sync daemonsets or controllers with selectors that
  match all pods (jliggitt@redhat.com)
- UPSTREAM: 23457: Do not track resource usage for host path volumes. They can
  contain loops. (jliggitt@redhat.com)
- UPSTREAM: 23325: Fix hairpin mode (jliggitt@redhat.com)
- UPSTREAM: 23019: Add a rate limiter to the GCE cloudprovider
  (jliggitt@redhat.com)
- UPSTREAM: 23141: kubelet: send all recevied pods in one update
  (jliggitt@redhat.com)
- UPSTREAM: 23143: Make kubelet default to 10ms for CPU quota if limit < 10m
  (jliggitt@redhat.com)
- UPSTREAM: 23034: Fix controller-manager race condition issue which cause
  endpoints flush during restart (jliggitt@redhat.com)
- bump inotify watches (jeder@redhat.com)
- Use scale subresource for DC scaling in web console (spadgett@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- Disambiguate origin generators (jliggitt@redhat.com)
- Add explicit emptyDir volumes where possible (ironcladlou@gmail.com)
- Resource discovery integration test (jliggitt@redhat.com)
- UPSTREAM: <carry>: v1beta3: ensure only v1 appears in discovery for legacy
  API group (jliggitt@redhat.com)
- Use strategy proxy setting for script download (cewong@redhat.com)
- bump(github.com/openshift/source-to-image):
  2c0fc8ae6150b27396dc00907cac128eeda99b09 (cewong@redhat.com)
- Deployment tests should really be disabled in e2e (ccoleman@redhat.com)
- fix useragent for SA (deads@redhat.com)
- Fix e2e test's check for determining that the router is up - wait for the
  healthz port to respond with success - HTTP status code 200. Still need to
  check for router pod to be born. (smitram@gmail.com)
- tweak jenkins job to test unrelased versions of the plugin
  (gmontero@redhat.com)
- Fix oadm diagnostic (master-node check for ovs plugin) to retrieve the list
  of nodes running on the same machine as master. (avagarwa@redhat.com)
- scc volumes support (pweil@redhat.com)
- UPSTREAM: <carry>: scc volumes support (pweil@redhat.com)
- UPSTREAM: <carry>: v1beta3 scc volumes support (pweil@redhat.com)
- add volume prereq to db template descriptions (bparees@redhat.com)
- Fix typo (tdawson@redhat.com)
- Verify yum installed rpms (tdawson@redhat.com)

* Mon Mar 28 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.8
- E2e deployments filter is incorrect (ccoleman@redhat.com)
- Reorder the debug pod name (ccoleman@redhat.com)
- use a first class field definition to identify scratch images
  (bparees@redhat.com)
- Remove project admin/edit ability to create daemonsets (jliggitt@redhat.com)
- Bug 1314270: force dc reconcilation on canceled deployments
  (mkargaki@redhat.com)
- controller: refactor deployer controller interfaces (mkargaki@redhat.com)
- cli: oc process should print errors to stderr (stefw@redhat.com)
- use emptydir for sample-app volumes (bparees@redhat.com)
- fix extended cmd.sh to handle faster importer (deads@redhat.com)
- #7976 : Initialize Binary source to an empty default state if type but no
  value set (for API v1) (roland@jolokia.org)
- Atomic registry quickstart image (aweiteka@redhat.com)
- Fix bug where router reload fails to run lsof - insufficient permissions with
  the hostnetwork scc. Reduce the lsof requirement since we now check for error
  codes [non zero means bind errors] and have a healthz check as a sanity
  check. Plus fixes as per @smarterclayton review comments. (smitram@gmail.com)
- Include branded header within <noscript> message. (sgoodwin@redhat.com)
- Better error message when JavaScript is disabled (jawnsy@redhat.com)
- Simplify synthetic skips so that no special chars are needed Isolate the
  package skipping into a single function. (jay@apache.org)
- Fix new-app template search with multiple matches (cewong@redhat.com)
- UPSTREAM: <carry>: Suppress aggressive output of warning
  (ccoleman@redhat.com)
- hardcode build name to expect instead of getting it from start-build output
  (bparees@redhat.com)
- New skips in extended tests (ccoleman@redhat.com)
- removed binary etcd store from test-cmd artfacts (skuznets@redhat.com)
- Fix resolver used for --image-stream param, annotation searcher output
  (cewong@redhat.com)
- UPSTREAM: 23065: Remove gce provider requirements from garbage collector test
  (tiwillia@redhat.com)
- Bindata change for error with quotes on project 404 (jforrest@redhat.com)
- Fixed error with quotes (jlam@snaplogic.com)
- Escape ANSI color codes in web console logs (spadgett@redhat.com)
- refactor to not use dot imports for heredoc (skuznets@redhat.com)
- Bug 1320335: Fix quoting for mysql probes (mfojtik@redhat.com)
- Add client utilities for iSCSI and Ceph. (jsafrane@redhat.com)
- loosen exec to allow SA checks for privileges (deads@redhat.com)
- Allow perFSGroup local quota in config on first node start.
  (dgoodwin@redhat.com)
- use a max value of 92233720368547 for cgroup values (bparees@redhat.com)
- Revert "temporarily disable cgroup limits on builds" (bparees@redhat.com)
- update generated code and docs (pweil@redhat.com)
- UPSTREAM: 22857: partial - ensure DetermineEffectiveSC retains the container
  setting for readonlyrootfs (pweil@redhat.com)
- UPSTREAM: <carry>: v1beta3 scc - read only root file system support
  (pweil@redhat.com)
- UPSTREAM: <carry>: scc - read only root file system support
  (pweil@redhat.com)
- UPSTREAM: 23279: kubectl: enhance podtemplate describer (mkargaki@redhat.com)
- oc: add volume info on the dc describer (mkargaki@redhat.com)
- remove dead cancel code (bparees@redhat.com)
- Enable the pod garbage collector (tiwillia@redhat.com)
- pkg: cmd: cli: cmd: startbuild: close response body (runcom@redhat.com)

* Wed Mar 23 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.7
- Ensure ingress host matches route host (marun@redhat.com)
- Enable extensions storage for batch/autoscaling (jliggitt@redhat.com)
- Add navbar-utility-mobile to error.html Fixes
  https://github.com/openshift/origin/issues/8198 (sgoodwin@redhat.com)
- Add /dev to node volumes (sdodson@redhat.com)
- Install e2fsprogs and xfsprogs into base image (sdodson@redhat.com)
- oc debug is not defaulting to TTY (ccoleman@redhat.com)
- UPSTREAM: revert: d54ed4e: 21373: kubelet: reading cloudinfo from cadvisor
  (deads@redhat.com)
- temporarily disable cgroup limits on builds (bparees@redhat.com)
- test/extended/images/mongodb_replica: add tests for mongodb replication
  (vsemushi@redhat.com)
- oc status must show monopods (ffranz@redhat.com)
- Integration tests should use docker.ClientFromEnv() (ccoleman@redhat.com)
- Move upstream (ccoleman@redhat.com)
- hack/test-cmd.sh races against deployment controller (ccoleman@redhat.com)
- make who-can use resource arg format (deads@redhat.com)
- Bug in Kube API version group ordering (ccoleman@redhat.com)
- Fix precision displaying percentages in quota chart tooltip
  (spadgett@redhat.com)
- updated artifacts to contain docker log and exlucde etcd data dir
  (skuznets@redhat.com)
- Mount /var/log into node container (sdodson@redhat.com)
- Hide extra close buttons for task lists (spadgett@redhat.com)
- Test refactor (ccoleman@redhat.com)
- Disable failing upstream test (ccoleman@redhat.com)
- In the release target, only build linux/amd64 (ccoleman@redhat.com)
- Pod diagnostic check is not correct in go 1.6 (ccoleman@redhat.com)
- Update Dockerfile for origin-release to use Go 1.6 (ccoleman@redhat.com)
- Update build-go.sh to deal with Go 1.6 (ccoleman@redhat.com)
- Suppress Go 1.6 error on -X flag (ccoleman@redhat.com)
- Add RunOnceDuration and ProjectRequestLimit plugins to default plugin chains
  (cewong@redhat.com)
- Add kube component config tests, disable /logs on master, update kube-proxy
  init (jliggitt@redhat.com)
- Adjust -webkit-scrollbar width and log-scroll-top affixed position. Fixes
  https://github.com/openshift/origin/issues/7963 (sgoodwin@redhat.com)

* Mon Mar 21 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.6
- Bug 1318537 - Add warning when trying to import non-existing tag
  (maszulik@redhat.com)
- Bug 1310062 - Fallback to http if status code is not 2xx/3xx when deleting
  layers. (maszulik@redhat.com)
- Log the reload output for admins in the router logs (ccoleman@redhat.com)
- Set terminal max-width to 100%% for mobile (spadgett@redhat.com)
- Hide the java link if the container is not ready (slewis@fusesource.com)
- Support limit quotas and scopes in UI (spadgett@redhat.com)
- Add test for patch+conflicts (jliggitt@redhat.com)
- Removed the stray line that unconditionally forced on the SYN eater.
  (bbennett@redhat.com)
- Remove large, unnecessary margin from bottom of create forms
  (spadgett@redhat.com)
- Use smaller log font size for mobile (spadgett@redhat.com)
- UPSTREAM: 23145: Use versioned object when computing patch
  (jliggitt@redhat.com)
- Handle new volume source types on web console (ffranz@redhat.com)
- Show consistent pod status in web console as CLI (spadgett@redhat.com)
- PVCs should not be editable once bound (ffranz@redhat.com)
- bump(github.com/openshift/source-to-image):
  625b58aa422549df9338fdaced1b9444d2313a15 (rhcarvalho@gmail.com)
- bump(github.com/openshift/openshift-sdn):
  72d9ab84f4bf650d1922174e6a90bd06018003b4 (dcbw@redhat.com)
- Reworked image quota (miminar@redhat.com)
- Fix certificate display on mobile (spadgett@redhat.com)
- Include container ID in glog message (rhcarvalho@gmail.com)
- Ignore default security context constraints when running on kube
  (decarr@redhat.com)
- use transport defaults (deads@redhat.com)
- UPSTREAM: 23003: support CIDRs in NO_PROXY (deads@redhat.com)
- UPSTREAM: 22852: Set a missing namespace on objects to admit
  (miminar@redhat.com)
- Handle fallback to docker.io for 1.9 docker, which uses docker.io in
  .docker/config.json (maszulik@redhat.com)
- Revert "platformmanagement_public_425 - add quota information to oc describe
  is" (miminar@redhat.com)

* Fri Mar 18 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.5
- Don't show chromeless log link if log not available (spadgett@redhat.com)
- UPSTREAM: 21373: kubelet: reading cloudinfo from cadvisor (deads@redhat.com)
- fix typo in db template readme (bparees@redhat.com)
- oc: more status fixes (mkargaki@redhat.com)
- Don't autofocus catalog filter input (spadgett@redhat.com)
- Add e2fsprogs to base image (sdodson@redhat.com)
- Bump kubernetes-container-terminal to 0.0.11 (spadgett@redhat.com)
- oc: plumb error writer in oc edit (mkargaki@redhat.com)
- UPSTREAM: 22634: kubectl: print errors that wont be reloaded in the editor
  (mkargaki@redhat.com)
- Include all extended tests in a single binary (marun@redhat.com)
- Update swagger spec (jliggitt@redhat.com)
- bump(k8s.io/kubernetes): 4a3f9c5b19c7ff804cbc1bf37a15c044ca5d2353
  (jliggitt@redhat.com)
- bump(github.com/google/cadvisor): 546a3771589bdb356777c646c6eca24914fdd48b
  (jliggitt@redhat.com)
- add debug when extended build tests fail (bparees@redhat.com)
- clean up jenkins master/slave parameters (bparees@redhat.com)
- Web console: fix problem balancing create flow columns (spadgett@redhat.com)
- Tooltip for multiple ImageSources in BC editor (jhadvig@redhat.com)
- fix two broken extended tests (bparees@redhat.com)
- Bug fix so that table-mobile will word-wrap: break-word (rhamilto@redhat.com)
- Add preliminary quota support for emptyDir volumes on XFS.
  (dgoodwin@redhat.com)
- Bump unit test timeout (jliggitt@redhat.com)
- updated tmpdir for e2e-docker (skuznets@redhat.com)
- Load environment files in containerized systemd units (sdodson@redhat.com)
- Interesting changes for rebase (jliggitt@redhat.com)
- Extended test namespace creation fixes (jliggitt@redhat.com)
- Mechanical changes for rebase (jliggitt@redhat.com)
- fix credential lookup for authenticated image stream import
  (jliggitt@redhat.com)
- Generated docs, conversions, copies, completions (jliggitt@redhat.com)
- UPSTREAM: <carry>: Allow overriding default generators for run
  (jliggitt@redhat.com)
- UPSTREAM: 22921: Fix job selector validation and tests (jliggitt@redhat.com)
- UPSTREAM: 22919: Allow starting test etcd with http (jliggitt@redhat.com)
- Stack definition lists only at narrower widths (spadgett@redhat.com)
- Disable externalIP by default (ccoleman@redhat.com)
- oc: warn about missing stream when deleting a tag (mkargaki@redhat.com)
- implemented miscellaneous iprovements for test-cmd (skuznets@redhat.com)
- Handle env vars that use valueFrom (jhadvig@redhat.com)
- UPSTREAM: 22917: Decrease verbosity of namespace controller trace logging
  (jliggitt@redhat.com)
- UPSTREAM: 22916: Correctly identify namespace subresources in GetRequestInfo
  (jliggitt@redhat.com)
- UPSTREAM: 22914: Move TestRuntimeCache into runtime_cache.go file
  (jliggitt@redhat.com)
- UPSTREAM: 22913: register internal types with scheme for reference unit test
  (jliggitt@redhat.com)
- UPSTREAM: 22910: Decrease parallelism in deletecollection test, lengthen test
  etcd certs (jliggitt@redhat.com)
- UPSTREAM: 22875: Tolerate multiple registered versions in a single group
  (jliggitt@redhat.com)
- UPSTREAM: 22877: mark filename flags for completions (ffranz@redhat.com)
- UPSTREAM: 22929: Test relative timestamps using UTC (jliggitt@redhat.com)
- UPSTREAM: 22746: add user-agent defaulting for discovery (deads@redhat.com)
- bump(github.com/Sirupsen/logrus): aaf92c95712104318fc35409745f1533aa5ff327
  (jliggitt@redhat.com)
- bump(github.com/hashicorp/golang-lru):
  a0d98a5f288019575c6d1f4bb1573fef2d1fcdc4 (jliggitt@redhat.com)
- bump(bitbucket.org/ww/goautoneg): 75cd24fc2f2c2a2088577d12123ddee5f54e0675
  (jliggitt@redhat.com)
- bump(k8s.io/kubernetes): 148dd34ab0e7daeb82582d6ea8e840c15a24e745
  (jliggitt@redhat.com)
- Update copy-kube-artifacts script (jliggitt@redhat.com)
- Allow recursive unit testing packages under godeps (jliggitt@redhat.com)
- Update godepchecker to print commit dates, allow checking out commits
  (jliggitt@redhat.com)
- Ensure errors are reported back in the container logs. (smitram@gmail.com)
- UPSTREAM: 22999: Display a better login message (ccoleman@redhat.com)
- oc: better new-app suggestions (mkargaki@redhat.com)
- parameterize IS namespace (gmontero@redhat.com)
- place tmp secret files in tmpdir (skuznets@redhat.com)

* Wed Mar 16 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.4
- Update ose build scripts (tdawson@redhat.com)
- move-upstream should use UPSTREAM_REPO_LOCATION like cherry-pick
  (ccoleman@redhat.com)
- Update javaLink extension (admin@benjaminapetersen.me)
- add parameter to start OS server with latest images (skuznets@redhat.com)
- added test to decode and validate ldap sync config fixtures
  (skuznets@redhat.com)
- Bug 1317783: avoid shadowing errors in the deployment controller
  (mkargaki@redhat.com)
- Add a test of services/service isolation to tests/e2e/networking/
  (danw@redhat.com)
- Run the isolation extended networking tests under both plugins
  (danw@redhat.com)
- Make sanity and isolation network tests pass in a single-node environment
  (danw@redhat.com)
- Update extended networking tests to use k8s e2e utilities (danw@redhat.com)
- UPSTREAM: 22303: Make net e2e helpers public for 3rd party reuse
  (danw@gnome.org)
- Handle parametrized content types for build triggers (jimmidyson@gmail.com)
- Fix for bugz https://bugzilla.redhat.com/show_bug.cgi?id=1316698 and issue
  #7444   o Fixes as per @pweil- and @marun review comments.   o Fixes as per
  @smarterclayton review comments. (smitram@gmail.com)
- Ensure we are clean to docker.io/* images during hack/release.sh
  (ccoleman@redhat.com)
- make userAgentMatching take a set of required and deny regexes
  (deads@redhat.com)
- UPSTREAM: 22746: add user-agent defaulting for discovery (deads@redhat.com)
- fine tune which template parameter error types are returned
  (gmontero@redhat.com)
- Add ConfigMap permissions (pmorie@gmail.com)
- Slim down issue template appearance (jliggitt@redhat.com)
- Bug 1316749: prompt warning when scaling test deployments
  (mkargaki@redhat.com)
- Remove description field from types (mfojtik@redhat.com)
- [RPMS] Add extended.test to /usr/libexec/origin/extended.test
  (sdodson@redhat.com)
- made edge language less ambiguous (skuznets@redhat.com)
- Move hack/test-cmd_util.sh to test-util.sh, messing with script-fu
  (ccoleman@redhat.com)

* Mon Mar 14 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.3
- UPSTREAM: 22929: Test relative timestamps using UTC (jliggitt@redhat.com)
- DETECT_RACES doesn't work (ccoleman@redhat.com)
- Add policy constraints for node targeting (jolamb@redhat.com)
- Mark filename flags for completions (ffranz@redhat.com)
- UPSTREAM: 22877: mark filename flags for completions (ffranz@redhat.com)
- Send graceful shutdown signal to all haproxy processes + wait for process to
  start listening, fixes as per @smarterclayton review comments and for
  integration tests. (smitram@gmail.com)
- always flush glog before returning from build logic (bparees@redhat.com)
- Improving markup semantics and appearance of display of Volumes data
  (rhamilto@redhat.com)
- Bumping openshift-object-describer to v1.1.2 (rhamilto@redhat.com)
- Bump grunt-contrib-uglify to 0.6.0 (spadgett@redhat.com)
- Export OS_OUTPUT_GOPATH=1 in Makefile (stefw@redhat.com)
- Bug fix for long, unbroken words that don't wrap in pod template
  (rhamilto@redhat.com)
- Fix test with build reference cycle (rhcarvalho@gmail.com)
- updated issue template (skuznets@redhat.com)
- Rename misleading util function (rhcarvalho@gmail.com)
- Only check circular references for oc new-build (rhcarvalho@gmail.com)
- Extract TestBuildOutputCycleDetection (rhcarvalho@gmail.com)
- Fixes rsh usage (ffranz@redhat.com)
- Increase sdn node provisioning timeout (marun@redhat.com)
- Set default template router reload interval to 5 seconds. (smitram@gmail.com)
- Update and additions to web console screenshots (sgoodwin@redhat.com)

* Fri Mar 11 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.2
- Remove left over after move to test/integration (rhcarvalho@gmail.com)
- Improved next steps pages for bcs using sample repos (ffranz@redhat.com)
- Remove dead code (rhcarvalho@gmail.com)
- oc new-app/new-build: handle the case when an imagestream matches but has no
  tags (cewong@redhat.com)
- Add the ability to install iptables rules to eat SYN packets targeted to
  haproxy while the haproxy reload happens.  This prevents traffic to haproxy
  getting dropped if it connects while the reload is in progess.
  (bbennett@redhat.com)
- sample-app: update docs (mkargaki@redhat.com)
- bump(github.com/openshift/source-to-image):
  fb7794026064c5a7b83905674a5244916a07fef9 (rhcarvalho@gmail.com)
- Fixing BZ1291521 where long project name spills out of modal
  (rhamilto@redhat.com)
- Moving overflow:hidden to specifically target  long replication controller or
  deployment name instead of deployment-block when caused another issue.  -
  Fixes https://github.com/openshift/origin/issues/7887 (sgoodwin@redhat.com)
- changed find behavior for OSX compatibility (skuznets@redhat.com)
- add debug statements for test-go (skuznets@redhat.com)
- Improve log text highlighting in Firefox (spadgett@redhat.com)
- Fixes [options] in usage (ffranz@redhat.com)
- Prevent last catalog tile from stretching to 100%% width
  (spadgett@redhat.com)
- Fix default cert for edge route not being used - fixes #7904
  (smitram@gmail.com)
- Initial addition of issue template (mfojtik@redhat.com)
- Fix deployment page layout problems (spadgett@redhat.com)
- allow different cert serial number generators (deads@redhat.com)
- Enabling LessCSS source maps for development (rhamilto@redhat.com)
- Prevent fieldset from expanding due to content (jawnsy@redhat.com)
- Add placement and container to popover and tooltip into popover.js so that
  messages aren't hidden when spanning multiple scrollable areas.  - Fixes
  https://github.com/openshift/origin/issues/7723 (sgoodwin@redhat.com)
- bump(k8s.io/kubernetes): 91d3e753a4eca4e87462b7c9e5391ec94bb792d9
  (jliggitt@redhat.com)
- Add liveness and readiness probe for Jenkins (mfojtik@redhat.com)
- Fix word-break in Firefox (spadgett@redhat.com)
- Add table-bordered styles to service port table (spadgett@redhat.com)
- nocache should be noCache (haowang@redhat.com)
- Drop capabilities when running s2i build container (cewong@redhat.com)
- bump(github.com/openshift/source-to-image)
  0278ed91e641158fbbf1de08808a12d5719322d8 (cewong@redhat.com)
- Fixed races in ratelimiter tests on go1.5 (maszulik@redhat.com)

* Wed Mar 09 2016 Troy Dawson <tdawson@redhat.com> 3.2.0.1
- Change version numbering from 3.1.1.9xx to 3.2.0.x to avoid confusion.
  (tdawson@redhat.com)
- oc: update route warnings for oc status (mkargaki@redhat.com)
- Allow extra trusted bundles when generating master certs, node config, or
  kubeconfig (jliggitt@redhat.com)
- Update README.md (ccoleman@redhat.com)
- Update README.md (ccoleman@redhat.com)
- Update README (ccoleman@redhat.com)
- Break words when wrapping values in environment table (spadgett@redhat.com)
- Improve deployment name wrapping on overview page (spadgett@redhat.com)
- Bug 1315595: Use in-container env vars for liveness/readiness probes
  (mfojtik@redhat.com)
- deploy: more informative cancellation event on dc (mkargaki@redhat.com)
- Fixing typo (jhadvig@redhat.com)
- Show kind in editor modal (spadgett@redhat.com)
- rsync must validate if pod exists (ffranz@redhat.com)
- Minor fixes to Jenkins kubernetes readme (mfojtik@redhat.com)
- Breadcrumbs unification (jhadvig@redhat.com)
- test-cmd: mktemp --suffix is not supported in Mac (mkargaki@redhat.com)
- Fix hardcoded f5 username (admin). (smitram@gmail.com)
- Add "quickstart" to web console browse menu (spadgett@redhat.com)
- Add active deadline to browse pod page (spadgett@redhat.com)
- Unconfuse web console about resource and kind (spadgett@redhat.com)
- Fix role addition for kube e2e tests (marun@redhat.com)
- UPSTREAM: 22516: kubectl: set maxUnavailable to 1 if both fenceposts resolve
  to zero (mkargaki@redhat.com)
- Add pods donut to deployment page (spadgett@redhat.com)
- deploy: emit events on the dc instead of its rcs (mkargaki@redhat.com)
- prevent skewed client updates (deads@redhat.com)
- oc new-build: add --image-stream flag (cewong@redhat.com)
- UPSTREAM: 22526: kubectl: bring the rolling updater on par with the
  deployments (mkargaki@redhat.com)
- Set source of incremental build artifacts (rhcarvalho@gmail.com)
- bump(github.com/openshift/source-to-image):
  2e889d092f8f3fd0266610fa6b4d92db999ef68f (rhcarvalho@gmail.com)
- Use conventional profiler setup code (dmace@redhat.com)
- Support HTTP pprof server in registry (dmace@redhat.com)
- Bump docker minimum version to 1.9.1 in preparation for v1.2
  (sdodson@redhat.com)
- shorten dc caused by annotations (deads@redhat.com)

* Mon Mar 07 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.911
- add oc status warnings for missing is/istag/dockref/isimg for bc
  (gmontero@redhat.com)
- Update skip tags for gluster and ceph. (jay@apache.org)
- Skip quota check when cluster roles are outdated (miminar@redhat.com)
- Put dev cluster unit files in /etc/systemd/system (marun@redhat.com)
- dind: skip building etcd (marun@redhat.com)
- dind: disable sdn node at the end of provisioning (marun@redhat.com)
- Simplify vagrant/dind host provisioning (marun@redhat.com)
- Remove / fix dead code (ccoleman@redhat.com)
- UPSTREAM: <carry>: fix casting errors in case of obj nil
  (jawed.khelil@amadeus.com)
- Show log output in conversion generation (ccoleman@redhat.com)
- Support in-cluster-config for registry (ccoleman@redhat.com)
- Upgrade the registry to create secrets and service accounts
  (ccoleman@redhat.com)
- Support overriding the hostname in the router (ccoleman@redhat.com)
- Remove invisible browse option from web console catalog (spadgett@redhat.com)
- fixes bug 1312218 (bugzilla), fixes #7646 (github)
  (admin@benjaminapetersen.me)
- add discovery cache (deads@redhat.com)
- Making margin consistent around alerts inside.modal-resource-edit
  (rhamilto@redhat.com)
- platformmanagement_public_425 - add quota information to oc describe is
  (maszulik@redhat.com)
- oc: hide markers from different projects on oc status (mkargaki@redhat.com)
- Removing .page-header from About as the visuals aren't "right" for the page
  (rhamilto@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  58baf17e027bc1fd913cddd55c5eed4782400c60 (danw@redhat.com)
- UPSTREAM: revert: 902e416: <carry>: v1beta3 scc (dgoodwin@redhat.com)
- UPSTREAM: revert: 7d1b481: <carry>: scc (dgoodwin@redhat.com)
- Dashboard extended test should not be run (ccoleman@redhat.com)
- make all alias correctly (deads@redhat.com)
- Bug 1310616: Validate absolute dir in build secret for docker strategy in oc
  new-build (mfojtik@redhat.com)
- Remove unnecessary word from oc volume command (nakayamakenjiro@gmail.com)
- Revert "Updates to use the SCC allowEmptyDirVolumePlugin setting."
  (dgoodwin@redhat.com)
- UPSTREAM: <carry>: Increase test etcd request timeout to 30s
  (ccoleman@redhat.com)
- Fix services e2e tests for dev clusters (marun@redhat.com)
- tweak registry roles (deads@redhat.com)
- export OS_OUTPUT_GOPATH for target build (jawed.khelil@amadeus.com)
- Fix intra-pod kube e2e test (marun@redhat.com)
- WIP: Enable FSGroup in restricted and hostNS SCCs (pmorie@gmail.com)
- Change default ClusterNetworkCIDR and HostSubnetLength (danw@redhat.com)
- Remove downward api call for Jenkins kubernetes example (mfojtik@redhat.com)

* Fri Mar 04 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.910
- Resolving visual defect on Storage .page-header (rhamilto@redhat.com)
- Cleaning up random drop shadow and rounded corners on messenger messages
  (rhamilto@redhat.com)
- Correcting colspan value to resolve cosmetic bug with missing right border on
  <thead> (rhamilto@redhat.com)
- ignore unrelated build+pod events during tests (bparees@redhat.com)
- Adjust kube-topology so that it doesn't extend off of iOS viewport. Move
  bottom spacing from container-fluid to tab-content. (sgoodwin@redhat.com)
- Make mktmp call in common.sh compatible with OS X (cewong@redhat.com)
- Update external examples to include readiness/liveness probes
  (mfojtik@redhat.com)
- Fix build link in alert message (spadgett@redhat.com)
- Make "other routes" link go to browse routes page (spadgett@redhat.com)
- Skip hostPath test for upstream conformance test. (jay@apache.org)
- oc rsync: do not set owner when extracting with the tar strategy
  (cewong@redhat.com)
- removed SAR logfile from artifacts (skuznets@redhat.com)
- integration: Retry import from external registries when not reachable
  (miminar@redhat.com)
- Prevent log line number selection in Chrome (spadgett@redhat.com)
- Add create route button padding on mobile (jhadvig@redhat.com)
- test-cmd: ensure oc apply works with lists (mkargaki@redhat.com)
- UPSTREAM: 20948: Fix reference to versioned object in kubectl apply
  (mkargaki@redhat.com)
- Web console: fix problems with display route and route warnings
  (spadgett@redhat.com)
- Restrict events filter to certain fields in web console (spadgett@redhat.com)
- configchange: correlate triggers with the generated cause
  (mkargaki@redhat.com)
- Bug fix for negative reload intervals - bugz 1311459. (smitram@gmail.com)
- Fill image's metadata in the registry (miminar@redhat.com)
- Added additional quota check for layer upload in a registry
  (miminar@redhat.com)
- Resource quota for images and image streams (miminar@redhat.com)
- Add support for build config into oc set env (mfojtik@redhat.com)
- UPSTREAM: docker/distribution: 1474: Defined ErrAccessDenied error
  (miminar@redhat.com)
- UPSTREAM: docker/distribution: 1473: Commit uploaded blob with size
  (miminar@redhat.com)
- UPSTREAM: 20446: New features in ResourceQuota (miminar@redhat.com)

* Wed Mar 02 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.909
- Added more cmd tests for import-image to cover main branches
  (maszulik@redhat.com)
- Support API group and version in SAR/RAR (jliggitt@redhat.com)
- Pick correct strategy for binary builds (cewong@redhat.com)
- Metrics: show missing data as gaps in the chart (spadgett@redhat.com)
- Issue 7555 - fixed importimage which was picking wrong docker pull spec for
  images that failed previous import. (maszulik@redhat.com)
- Refactor import-image to Complete-Validate-Run scheme. Additionally split the
  code so it's testable + added tests. (maszulik@redhat.com)
- Adds completion for oc rsh command (akram@free.fr)
- Fix swagger description generation (jliggitt@redhat.com)
- Allow externalizing/encrypting config values (jliggitt@redhat.com)
- Add encrypt/decrypt helper commands (jliggitt@redhat.com)
- bump all template mem limits to 512 Mi (gmontero@redhat.com)
- Fixes as per @smarterclayton's review comments. (smitram@gmail.com)
- Use http[s] ports for environment values. Allows router ports to be overriden
  + multiple instances to run with host networking. (smitram@gmail.com)
- Switch from margin to padding and move it to the container-fluid div so gray
  bg extends length of page and maintains bottom spacing across pages Include
  fix to prevent filter appended button from wrapping in Safari
  (sgoodwin@redhat.com)
- check covers for role changes (deads@redhat.com)
- favicon.ico not copied during asset build (jforrest@redhat.com)
- Added liveness and readiness probes to database templates
  (mfojtik@redhat.com)
- Create policy for image registry users (agladkov@redhat.com)

* Mon Feb 29 2016 Scott Dodson <sdodson@redhat.com> 3.1.1.908
- enabled junitreport tool to stream output (skuznets@redhat.com)
- BZ_1312819: Can not add Environment Variables on buildconfig edit page
  (jhadvig@redhat.com)
- rewrite hack/test-go.sh (skuznets@redhat.com)
- UPSTREAM: <drop>: patch for 16146: Fix validate event for non-namespaced
  kinds (deads@redhat.com)
- added commands to manage serviceaccounts (skuznets@redhat.com)
- configchange: proceed with deployment with non-automatic ICTs
  (mkargaki@redhat.com)
- Allow use S2I builder with non-s2i build strategies (mfojtik@redhat.com)
- Verify the integration test build early (ccoleman@redhat.com)
- Support building from dirs symlinked from GOPATH (pmorie@gmail.com)
- remove openshift ex tokens (deads@redhat.com)
- Improve Dockerfile keyword highlighting in web console (spadgett@redhat.com)
- Don't shutdown etcd in integration tests (ccoleman@redhat.com)
- Fix OSE branding on web console (#1309205) (tdawson@redhat.com)
- hack/update-swagger-spec times out in integration (ccoleman@redhat.com)
- Dump debug info from etcd during integration tests (ccoleman@redhat.com)
- Upgrade dind image to fedora23 (marun@redhat.com)
- Add header back to logging in page (spadgett@redhat.com)
- UPSTREAM: 21265: added 'kubectl create sa' to create serviceaccounts
  (skuznets@redhat.com)
- Updated copy-kube-artifacts to current k8s (maszulik@redhat.com)

* Fri Feb 26 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.907
- Fix OSE branding on web console (#1309205) (tdawson@redhat.com)
- Add ability to push to image build script (tdawson@redhat.com)
- Change button label from "Cancel" to "Cancel Build" (spadgett@redhat.com)
- Filter and sort web console events table (spadgett@redhat.com)
- Fix timing problem enabling start build button (spadgett@redhat.com)
- Add Patternfly button styles to catalog browse button (spadgett@redhat.com)
- Bump Vagrant machine RAM requirement (dcbw@redhat.com)
- Remove json files added accidentally (rhcarvalho@gmail.com)
- Submit forms on enter (jhadvig@redhat.com)
- Set triggers via the CLI (ccoleman@redhat.com)
- UPSTREAM: <drop>: utility for the rolling updater (mkargaki@redhat.com)
- UPSTREAM: 21872: kubectl: preserve availability when maxUnavailability is not
  100%% (mkargaki@redhat.com)
- Including css declarations of flex specific prefixes for IE10 to position
  correctly (sgoodwin@redhat.com)
- Adjustments to the css controlling the filter widget so that it addresses
  some overlapping issues. Also, subtle changes to the project nav menu
  scrollbar so that it's more noticable. (sgoodwin@redhat.com)
- Only show builds bar chart when at least 4 builds (spadgett@redhat.com)
- Disable failing extended tests. (ccoleman@redhat.com)
- Remove v(4) logging of build admission startup (cewong@redhat.com)
- Include build hook in describer (rhcarvalho@gmail.com)
- Update hack/update-external-examples.sh (rhcarvalho@gmail.com)
- Update external examples (rhcarvalho@gmail.com)
- Add shasums to release build output (ccoleman@redhat.com)
- diagnostics: promote from openshift ex to oadm (lmeyer@redhat.com)
- Integrate etcd into the test cases themselves (ccoleman@redhat.com)
- Web console: improve repeated events message (spadgett@redhat.com)
- Improve web console metrics error message (spadgett@redhat.com)
- added generated swagger descriptions for v1 api (skuznets@redhat.com)
- Fix css compilation issues that affect IE, particularly flexbox
  (jforrest@redhat.com)
- pruned govet whitelist (skuznets@redhat.com)
- Use args flavor sample-app build hooks (rhcarvalho@gmail.com)
- added automatic swagger doc generator (skuznets@redhat.com)
- disabling start build/rebuild button when bc is deleted (jhadvig@redhat.com)
- UPSTREAM: <carry>: change BeforeEach to JustBeforeEach to ensure SA is
  granted to anyuid SCC (pweil@redhat.com)
- Fix reuse of release build on nfs mount (marun@redhat.com)
- Force dind deployment to build binaries by default (marun@redhat.com)
- Symlink repo mount to /origin for convenience (marun@redhat.com)
- Fix vagrant vm cluster and dind deployment (marun@redhat.com)
- Fix dind go build (marun@redhat.com)

* Wed Feb 24 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.906
- Fix cli download link (#1311396) (tdawson@redhat.com)
- Fix bindata diff (spadgett@redhat.com)
- Add `oc debug` to make it easy to launch a test pod (ccoleman@redhat.com)
- Hide hidden flags in help output (ccoleman@redhat.com)
- React to changes in upstream term (ccoleman@redhat.com)
- UPSTREAM: 21624: improve terminal reuse and attach (ccoleman@redhat.com)
- Add 'oc set probe' for setting readiness and liveness (ccoleman@redhat.com)
- Remove npm shrinkwrap by bumping html-min deps (jforrest@redhat.com)
- Fix router e2e validation for docker 1.9 (marun@redhat.com)
- Cache projects outside of projectHeader link fn (admin@benjaminapetersen.me)
- Use $scope.$emit to notify projectHeader when project settings change
  (admin@benjaminapetersen.me)
- fix builder version typo (bparees@redhat.com)
- Update swagger description (jliggitt@redhat.com)
- add hostnetwork scc (pweil@redhat.com)
- UPSTREAM: 21680: Restore service port validation compatibility with 1.0/1.1
  (jliggitt@redhat.com)
- Read extended user attributes from auth proxy (jliggitt@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  08a79d5adc8af21b14adcc0b9650df2d5fccf2f0 (danw@redhat.com)
- Display better route info in status (ccoleman@redhat.com)
- Ensure log run vars are set in GET and WATCH on build, pod & deployment
  (admin@benjaminapetersen.me)
- Contextualize errors from GetCGroupLimits (rhcarvalho@gmail.com)
- Fix extended tests and up default pod limit. (ccoleman@redhat.com)
- configchange: abort update once an image change is detected
  (mkargaki@redhat.com)
- UPSTREAM: 21671: kubectl: add container ports in pod description
  (mkargaki@redhat.com)
- Js error on overview for route warnings (jforrest@redhat.com)
- UPSTREAM: 21706: Ensure created service account tokens are available to the
  token controller (jliggitt@redhat.com)
- use imageid from trigger for imagesource inputs, instead of resolving them
  (bparees@redhat.com)
- increase binary build timeout to 5 minutes (bparees@redhat.com)
- Add status icon for ContainerCreating reason (spadgett@redhat.com)
- integration test for newapp dockerimagelookup (bparees@redhat.com)
- pod diagnostics: fix panic in bz 1302649, prettify (lmeyer@redhat.com)
- Fix log follow link on initial page load, add loading ellipsis while
  logViewer is pending (admin@benjaminapetersen.me)
- Don't show "Deployed" for plain RCs in web console (spadgett@redhat.com)
- Run post build hook with `/bin/sh -ic` (rhcarvalho@gmail.com)
- origin-pod rpm does not require the base rpm (sdodson@redhat.com)
- Mobile table headers missing on browse image page (jforrest@redhat.com)
- Suppress escape sequences at end of hack/test-assets.sh (ccoleman@redhat.com)
- Replace /bin/bash in oc rsh with /bin/sh (mfojtik@redhat.com)
- UPSTREAM: 19868: Fixed persistent volume claim controllers processing an old
  claim (jsafrane@redhat.com)
- Display additional tags after import (ccoleman@redhat.com)
- UPSTREAM: 21268: Delete provisioned volumes without claim.
  (jsafrane@redhat.com)
- UPSTREAM: 21273: kubectl: scale down based on ready during rolling updates
  (mkargaki@redhat.com)
- UPSTREAM: 20213: Fixed persistent volume claim controllers processing an old
  volume (jsafrane@redhat.com)

* Mon Feb 22 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.905
- Display better information when running 'oc' (ccoleman@redhat.com)
- Update completions to be cross platform (ccoleman@redhat.com)
- Add events tab to plain RCs in web console (spadgett@redhat.com)
- new-app broken when docker not installed (ccoleman@redhat.com)
- UPSTREAM: 21628: Reduce node controller debug logging (ccoleman@redhat.com)
- Drop 1.4 and add 1.6 to travis Go matrix (ccoleman@redhat.com)
- Add --since-time logs test (jliggitt@redhat.com)
- UPSTREAM: 21398: Fix sinceTime pod log options (jliggitt@redhat.com)
- Show project display name in breadcrumbs (spadgett@redhat.com)
- Hide copy to clipboard button on iOS (spadgett@redhat.com)
- Validate master/publicMaster args to create-master-certs
  (jliggitt@redhat.com)
- Update completions (ffranz@redhat.com)
- UPSTREAM: 21593: split adding global and external flags (ffranz@redhat.com)
- clean up wording of oc status build/deployment descriptions
  (bparees@redhat.com)
- iOS: prevent select from zooming page (spadgett@redhat.com)
- Restoring fix for route names overflowing .componet in Safari for iOS
  (rhamilto@redhat.com)
- On node startup, perform more checks of known requirements
  (ccoleman@redhat.com)
- Bug 1309195 - Return ErrNotV2Registry when falling back to http backend
  (maszulik@redhat.com)
- Set OS_OUTPUT_GOPATH=1 to build in a local GOPATH (ccoleman@redhat.com)
- Add back the filter bar to the bc and dc pages (jforrest@redhat.com)
- Router should tolerate not having permission to write status
  (ccoleman@redhat.com)
- Env vars with leading slashes cause major js errors in console create from
  image flow (jforrest@redhat.com)
- Refactor WaitForADeployment (rhcarvalho@gmail.com)
- bump(github.com/elazarl/goproxy): 07b16b6e30fcac0ad8c0435548e743bcf2ca7e92
  (ffranz@redhat.com)
- UPSTREAM: 21409: SPDY roundtripper support to proxy with Basic auth
  (ffranz@redhat.com)
- UPSTREAM: 21185: SPDY roundtripper must respect InsecureSkipVerify
  (ffranz@redhat.com)
- Use route ingress status in console (jforrest@redhat.com)
- correct cluster resource override tests (deads@redhat.com)
- UPSTREAM: 21341: Add a liveness and readiness describer to pods
  (mkargaki@redhat.com)
- oc: enhance deploymentconfig description (mkargaki@redhat.com)
- Fix commit checker to find commits with upstream changes
  (jliggitt@redhat.com)
- Verify extended tests build (jliggitt@redhat.com)
- Normalize usernames for AllowAllPasswordIdentityProvider
  (jliggitt@redhat.com)
- Add auth logging to login page, basic auth, and OAuth paths
  (jliggitt@redhat.com)
- Use "install" to install SDN script to make sure they get exec permission
  (danw@redhat.com)
- tar extract cannot hard link on vboxfs filesystem (horatiu@vlad.eu)
- Use pkg/util/homedir from upstream to detect home directory
  (ffranz@redhat.com)
- UPSTREAM: 17590: use correct home directory on Windows (ffranz@redhat.com)
- Web console: add "Completed" to status-icon directive (spadgett@redhat.com)
- Adjust empty state margin for pages with tabs (spadgett@redhat.com)
- Limit route.spec.to to kind/name (jliggitt@redhat.com)
- Add cross project promotion example (mfojtik@redhat.com)

* Fri Feb 19 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.904
- Updated URLs fo OpenShift Enterprise (tdawson@redhat.com)
- Fix extended build compile error (jliggitt@redhat.com)
- refactored test-end-to-end/core to use os::cmd functions
  (skuznets@redhat.com)
- update completions (pweil@redhat.com)
- UPSTREAM: <carry>: add scc describer (pweil@redhat.com)
- Allow subdomain flag to create router (ccoleman@redhat.com)
- add gutter class to annotations directive to provide margin
  (admin@benjaminapetersen.me)
- vendor quickstart templates into origin (bparees@redhat.com)
- Add attach storage and create route to the actions dropdown
  (spadgett@redhat.com)
- properly check for nil docker client value (bparees@redhat.com)
- Web console: show more detailed pod status (spadgett@redhat.com)
- always pull the previous image for s2i builds (bparees@redhat.com)
- Improve log error messages (admin@benjaminapetersen.me)
- add DB icons and also add annotations to the 'latest' imagestream tags
  (bparees@redhat.com)
- Add Jenkins with kubernetes plugin example (mfojtik@redhat.com)
- UPSTREAM: 21470: fix limitranger to handle latent caches without live lookups
  every time (deads@redhat.com)
- react to limitrange update (deads@redhat.com)
- Handle multiple imageChange triggers in BC edit page (jhadvig@redhat.com)
- Resolving a couple cosmetic issues with navbar at mobile resolutions
  (rhamilto@redhat.com)
- Refactoring .component to prevent weird wrapping issues (rhamilto@redhat.com)
- UPSTREAM: 21335: make kubectl logs work for replication controllers
  (deads@redhat.com)
- make sure that logs for rc work correctly (deads@redhat.com)
- Use in-cluster-config without setting POD_NAMESPACE (jliggitt@redhat.com)
- UPSTREAM: 21095: Provide current namespace to InClusterConfig
  (jliggitt@redhat.com)
- Get rid of the plugins/ dir (ccoleman@redhat.com)
- Route ordering is unstable, and writes must be ignored (ccoleman@redhat.com)
- Replace kebab with actions button on browse pages (spadgett@redhat.com)
- Fixes for unnecessary scrollbars in certain areas and situations
  (sgoodwin@redhat.com)
- Fix asset build so that it leaves the dev environment in place without having
  to re-launch grunt serve (jforrest@redhat.com)
- addition of memory limits with online beta in mind (gmontero@redhat.com)
- make hello-openshift print to stdout when serving a request
  (bparees@redhat.com)
- Run-once pod duration: remove flag from plugin config (cewong@redhat.com)
- Add deletecollection verb to admin/edit roles (jliggitt@redhat.com)
- UPSTREAM: 21005: Use a different verb for delete collection
  (jliggitt@redhat.com)
- Validate wildcard certs against non-wildcard namedCertificate names
  (jliggitt@redhat.com)
- Image building resets the global script time (ccoleman@redhat.com)
- remove volumes when removing containers (skuznets@redhat.com)
- UPSTREAM: 21089: Default lockfile to empty string while alpha
  (pweil@redhat.com)
- UPSTREAM: 21340: Tolerate individual NotFound errors in DeleteCollection
  (pweil@redhat.com)
- UPSTREAM: 21318: kubectl: use the factory properly for recording commands
  (pweil@redhat.com)
- refactor api interface to allow returning an error (pweil@redhat.com)
- fixing tests (pweil@redhat.com)
- proxy config refactor (pweil@redhat.com)
- boring refactors (pweil@redhat.com)
- UPSTREAM: <carry>: update generated client code for SCC (pweil@redhat.com)
- UPSTREAM: 21278: include discovery client in adaptor (pweil@redhat.com)
- bump(k8s.io/kubernetes): bc4550d9e93d04e391b9e33fc85a679a0ca879e9
  (pweil@redhat.com)
- UPSTREAM: openshift/openshift-sdn: <drop>: openshift-sdn refactoring
  (pweil@redhat.com)
- bump(github.com/stretchr/testify): e3a8ff8ce36581f87a15341206f205b1da467059
  (pweil@redhat.com)
- bump(github.com/onsi/ginkgo): 07d85e6b10c4289c7d612f9b13f45ba36f66d55b
  (pweil@redhat.com)
- bump(github.com/fsouza/go-dockerclient):
  0099401a7342ad77e71ca9f9a57c5e72fb80f6b2 (pweil@redhat.com)
- UPSTREAM: coreos/etcd: 4503: expose error details for normal stringify
  (deads@redhat.com)
- bump(github.com/coreos/etcd): bc9ddf260115d2680191c46977ae72b837785472
  (pweil@redhat.com)
- godeps: fix broken hash before restore (pweil@redhat.com)
- The Host value should be written to all rejected routes (ccoleman@redhat.com)
- fix jenkins testjob xml; fix jenkins ext test deployment error handling
  (gmontero@redhat.com)
- Web console: honor cluster-resource-override-enabled (spadgett@redhat.com)
- bump(github.com/openshift/source-to-image):
  41947800efb9fb7f5c3a13e977d26ac0815fa4fb (maszulik@redhat.com)
- UPSTREAM: 21266: only load kubeconfig files one time (deads@redhat.com)
- Fix admission attribute comparison (agladkov@redhat.com)
- Suppress conflict error printout (ccoleman@redhat.com)

* Wed Feb 17 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.903
- Change web console to display OpenShift Enterprise logo and urls
  (tdawson@redhat.com)
- remove testing symlink (deads@redhat.com)
- Add pathseg polyfill to fix c3 bar chart runtime error (spadgett@redhat.com)
- Size build chart correctly in Firefox (spadgett@redhat.com)
- dump build logs when build test fails (bparees@redhat.com)
- fix circular input/output detection (bparees@redhat.com)
- do not tag for pushing if there is no output target (bparees@redhat.com)
- admission: cluster req/limit override plugin (lmeyer@redhat.com)
- ignore events from previous builds (bparees@redhat.com)
- Changes and additions to enable text truncation of the project menu and
  username at primary media query breakpoints (sgoodwin@redhat.com)
- Addition of top-header variables for mobile and desktop to set height and
  control offset of fixed header height.         This will ensure the proper
  bottom offset so that the flex containers extend to the bottom correctly.
  Switch margin-bottom to padding-bottom so that background color is maintained
  (sgoodwin@redhat.com)
- Set a timeout on integration tests of 4m (ccoleman@redhat.com)
- added support for paged queries in ldap sync (skuznets@redhat.com)
- bump(gopkg.in/ldap.v2): 07a7330929b9ee80495c88a4439657d89c7dbd87
  (skuznets@redhat.com)
- Resource specific events on browse pages (jhadvig@redhat.com)
- added Godoc to api types where Godoc was missing (skuznets@redhat.com)
- updated commitchecker regex to work for ldap package (skuznets@redhat.com)
- Fix layout in osc-key-value directive (admin@benjaminapetersen.me)
- bump(github.com/openshift/openshift-sdn):
  5cf5cd2666604324c3bd42f5c12774cfaf1a3439 (danw@redhat.com)
- Add docker-registry image store on glusterfs volume example
  (jcope@redhat.com)
- Bump travis to go1.5.3 (jliggitt@redhat.com)
- Provide a way in console to access orphaned builds / deployments
  (jforrest@redhat.com)
- Revising sidebar to better align with PatternFly standard
  (rhamilto@redhat.com)
- refactor docker image searching (bparees@redhat.com)
- Remove EtcdClient from MasterConfig (agladkov@redhat.com)
- Move 'adm' function into 'oc' as 'oc adm' (ccoleman@redhat.com)
- Break compile time dependency on etcd for clients (ccoleman@redhat.com)

* Mon Feb 15 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.902
-  Backporting dist-git Dockerfile to Dockerfile.product (tdawson@redhat.com)
- added infra namespace to PV recycler (mturansk@redhat.com)
- Run user-provided command as part of build flow (rhcarvalho@gmail.com)
- Fix deployment log var (admin@benjaminapetersen.me)
- Add direnv .envrc to gitignore (pmorie@gmail.com)
- Use normal GOPATH for build (ccoleman@redhat.com)
- DeploymentConfig hooks did not have round trip defaulting
  (ccoleman@redhat.com)
- Fix pod warnings popup (spadgett@redhat.com)
- Move slow newapp tests to integration (ccoleman@redhat.com)
- filter events being tested (bparees@redhat.com)
- Improve performance of overview page with many deployments
  (spadgett@redhat.com)
- mark slow extended build/image tests (bparees@redhat.com)
- Tweak LDAP sync config error flags (jliggitt@redhat.com)
- Add extension points to the nav menus and add sample extensions for online
  (jforrest@redhat.com)
- UPSTREAM: coreos/etcd: 4503: expose error details for normal stringify
  (deads@redhat.com)
- suppress query scope issue on member extraction (skuznets@redhat.com)
- use correct fixture path (bparees@redhat.com)
- Add a TagImages hook type to lifecycle hooks (ccoleman@redhat.com)
- Support create on update of imagestreamtags (ccoleman@redhat.com)
- Many anyuid programs fail due to SETGID/SETUID caps (ccoleman@redhat.com)
- Exclude failing tests, add [Kubernetes] and [Origin] skip targets
  (ccoleman@redhat.com)
- Scheduler has an official default name (ccoleman@redhat.com)
- Disable extended networking testing of services (marun@redhat.com)
- Fix filename of network test entry point (marun@redhat.com)
- Make sure extended networking isolation test doesn't run for subnet plugin
  (dcbw@redhat.com)
- Ensure more code uses the default transport settings (ccoleman@redhat.com)
- read docker pull secret from correct path (bparees@redhat.com)
- Ignore .vscode (ccoleman@redhat.com)
- Have routers take ownership of routes (ccoleman@redhat.com)
- Fix web console dev env certificate problems for OS X Chrome
  (spadgett@redhat.com)
- Update build info in web console pod template (spadgett@redhat.com)
- Changing .ace_editor to .ace_editor-bordered so the border around .ace_editor
  is optional (rhamilto@redhat.com)
- Web console: Warn about problems with routes (spadgett@redhat.com)
- fix variable shadowing complained about by govet for 1.4 (bparees@redhat.com)
- added LDIF for suppression testing (skuznets@redhat.com)
- launch integration tests using only the API server when possible
  (deads@redhat.com)
- bump(k8s.io/kubernetes): f0cd09aabeeeab1780911c8023203993fd421946
  (pweil@redhat.com)
- Create oscUnique directive to provide unique-in-list validation on DOM nodes
  with ng-model attribute (admin@benjaminapetersen.me)
- Support modifiable pprof web port (nakayamakenjiro@gmail.com)
- Add additional docker volume (dmcphers@redhat.com)
- Web console: Use service port name for route targetPort (spadgett@redhat.com)
- Correcting reference to another step (rhamilto@redhat.com)
- fix jobs package import naming (bparees@redhat.com)
- fix ldap sync decode codec (deads@redhat.com)
- Fix attachScrollEvents on window.resize causing affixed follow links in
  logViewer to behave inconsistently (admin@benjaminapetersen.me)
- Use SA config when creating clients (ironcladlou@gmail.com)
- fix up client code to use the RESTMapper functions they mean
  (deads@redhat.com)
- fix ShortcutRESTMapper and prevent it from ever silently failing again
  (deads@redhat.com)
- UPSTREAM: 20968: make partial resource detection work for singular matches
  (deads@redhat.com)
- UPSTREAM: 20829: Union rest mapper (deads@redhat.com)
- validate default imagechange triggers (bparees@redhat.com)
- ignore .vscode settings (jliggitt@redhat.com)
- handle additional cgroup file locations (bparees@redhat.com)
- Fix web console type error when image has no env (spadgett@redhat.com)
- Fixing livereload so that it works with https (rhamilto@redhat.com)
- Display source downloading in build logs by default (mfojtik@redhat.com)
- Clean up test scripts (jliggitt@redhat.com)
- Forging consistency among empty tables at xs screen size #7163
  (rhamilto@redhat.com)
- UPSTREAM: 20814: type RESTMapper errors to better handle MultiRESTMapper
  errors (deads@redhat.com)
- Renamed extended tests files by removing directory name from certain files
  (maszulik@redhat.com)
- oc: enable autoscale for dcs (mkargaki@redhat.com)
- add fuzzer tests for config scheme (deads@redhat.com)
- Updates to use the SCC allowEmptyDirVolumePlugin setting.
  (dgoodwin@redhat.com)
- UPSTREAM: <carry>: scc (dgoodwin@redhat.com)
- UPSTREAM: <carry>: v1beta3 scc (dgoodwin@redhat.com)
- kebab case urls (and matching view templates), add legacy redirects
  (admin@benjaminapetersen.me)
- Add tests for explain (ccoleman@redhat.com)
- Bug fix where table border on right side of thead was disappearing at sm and
  md sizes (rhamilto@redhat.com)
- Cherrypick should force delete branch (ccoleman@redhat.com)
- Support block profile by pprof webserver (nakayamakenjiro@gmail.com)
- Allow recursive DNS to be enabled (ccoleman@redhat.com)
- Prevent dev cluster deploy from using stale config (marun@redhat.com)
- remove grep -P usage (skuznets@redhat.com)
- Support debugging networking tests with delve (marun@redhat.com)

* Tue Feb 09 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.901
- Add organization restriction to github IDP (jliggitt@redhat.com)
- UPSTREAM: 20827: Backwards compat for old Docker versions
  (ccoleman@redhat.com)
- Handle cert-writing error (jliggitt@redhat.com)
- image scripts updated to work with dependents. (tdawson@redhat.com)
- Add aos-3.2 (tdawson@redhat.com)
- Web console: only show "no services" message if overview empty
  (spadgett@redhat.com)
- Remove custom SIGQUIT handler (rhcarvalho@gmail.com)
- Use the vendored KUBE_REPO_ROOT (ccoleman@redhat.com)
- Include Kube examples, needed for extended tests (rhcarvalho@gmail.com)
- Update copy-kube-artifacts.sh (rhcarvalho@gmail.com)
- Return the Origin schema for explain (ccoleman@redhat.com)
- Regenerate conversions with stable order (ccoleman@redhat.com)
- UPSTREAM: 20847: Force a dependency order between extensions and api
  (ccoleman@redhat.com)
- UPSTREAM: 20858: Ensure public conversion name packages are imported
  (ccoleman@redhat.com)
- UPSTREAM: 20775: Set kube-proxy arg default values (jliggitt@redhat.com)
- Add kube-proxy config, match upstream proxy startup (jliggitt@redhat.com)
- allow either iptables-based or userspace-based proxy (danw@redhat.com)
- UPSTREAM: 20846: fix group mapping and encoding order: (deads@redhat.com)
- UPSTREAM: 20481: kubectl: a couple of edit fixes (deads@redhat.com)
- mark tests that access the host system as LocalNode (bparees@redhat.com)
- Build secrets isn't using fixture path (ccoleman@redhat.com)
- Test extended in parallel by default (ccoleman@redhat.com)
- UPSTREAM: 20796: SecurityContext tests wrong volume dir (ccoleman@redhat.com)
- UPSTREAM: 19947: Cluster DNS test is wrong (ccoleman@redhat.com)
- Replacing zeroclipboard with clipboard.js #5115 (rhamilto@redhat.com)
- import the AlwaysPull admission controller (pweil@redhat.com)
- GitLab IDP tweaks (jliggitt@redhat.com)
- BuildConfig editor fix (jhadvig@redhat.com)
- Tiny update in HACKING.md (nakayamakenjiro@gmail.com)
- Document requirements on kubernetes clone for cherry-picking.
  (jsafrane@redhat.com)
- Update recycler controller initialization. (jsafrane@redhat.com)
- Removing copyright leftovers (maszulik@redhat.com)
- UPSTREAM: 19365: Retry recycle or delete operation on failure
  (jsafrane@redhat.com)
- UPSTREAM: 19707: Fix race condition in cinder attach/detach
  (jsafrane@redhat.com)
- UPSTREAM: 19600: Fixed cleanup of persistent volumes. (jsafrane@redhat.com)
- example_test can fail due to validations (ccoleman@redhat.com)
- Add option and support for router id offset - this enables multiple
  ipfailover router installations to run within the same cluster. Rebased and
  changes as per @marun and @smarterclayton review comments.
  (smitram@gmail.com)
- Test extended did not compile (ccoleman@redhat.com)
- Unique host check should not delete when route is same (ccoleman@redhat.com)
- UPSTREAM: 20779: Take GVK in SwaggerSchema() (ccoleman@redhat.com)
- sanitize/consistentize how env variables are added to build pods
  (bparees@redhat.com)
- Update tag for Origin Kube (ccoleman@redhat.com)
- Fix typo (rhcarvalho@gmail.com)
- Add custom auth error template (jliggitt@redhat.com)
- fix multiple component error handling (bparees@redhat.com)
- Template test is not reentrant (ccoleman@redhat.com)
- Fix log viewer urls (jliggitt@redhat.com)
- make unit tests work (deads@redhat.com)
- make unit tests work (maszulik@redhat.com)
- eliminate v1beta3 round trip in the fuzzer.  We don't have to go out from
  there, only in (deads@redhat.com)
- move configapi back into its own scheme until we split the group
  (deads@redhat.com)
- refactor admission plugin types to avoid cycles and keep api types consistent
  (deads@redhat.com)
- update code generators (deads@redhat.com)
- make docker registry image auto-provisioning work with new status details
  (deads@redhat.com)
- add CLI helpers to convert lists before display since encoding no longer does
  it (deads@redhat.com)
- remove most of the latest package; it should go away completely
  (deads@redhat.com)
- template encoding/decoding no longer works like it used to (deads@redhat.com)
- add runtime.Object conversion method that works for now, but doesn't span
  groups or versions (deads@redhat.com)
- api type installation (deads@redhat.com)
- openshift launch sequence changed for rebase (deads@redhat.com)
- replacement etcd client (deads@redhat.com)
- oc behavior change by limiting generator scope (deads@redhat.com)
- runtime.EmbeddedObject removed (deads@redhat.com)
- scheme/codec changes (deads@redhat.com)
- API registration changes (deads@redhat.com)
- boring refactors for rebase (deads@redhat.com)
- UPSTREAM: 20736: clear env var check for unit test (deads@redhat.com)
- UPSTREAM: <drop>: make etcd error determination support old client until we
  drop it (deads@redhat.com)
- UPSTREAM: 20730: add restmapper String methods for debugging
  (deads@redhat.com)
- UPSTREAM: <drop>: disable kubelet image GC unit test (deads@redhat.com)
- UPSTREAM: 20648: fix validation error path for namespace (deads@redhat.com)
- UPSTREAM: <carry>: horrible hack for intstr types (deads@redhat.com)
- UPSTREAM: 20706: register internal types with scheme for reference unit test
  (deads@redhat.com)
- UPSTREAM: 20226:
  Godeps/_workspace/src/k8s.io/kubernetes/pkg/conversion/error.go
  (deads@redhat.com)
- UPSTREAM: 20511: let singularization handle non-conflicting ambiguity
  (deads@redhat.com)
- UPSTREAM: 20487: expose unstructured scheme as codec (deads@redhat.com)
- UPSTREAM: 20431: tighten api server installation for bad groups
  (deads@redhat.com)
- UPSTREAM: <drop>: patch for 16146: Fix validate event for non-namespaced
  kinds (deads@redhat.com)
- UPSTREAM: <drop>: merge multiple registrations for the same group
  (deads@redhat.com)
- UPSTREAM: emicklei/go-restful: <carry>: Add "Info" to go-restful ApiDecl
  (ccoleman@redhat.com)
- UPSTREAM: openshift/openshift-sdn: <drop>: minor updates for kube rebase
  (deads@redhat.com)
- UPSTREAM: docker/distribution: <carry>: remove parents on delete
  (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: export app.Namespace
  (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: 1050: Exported API functions needed for
  pruning (miminar@redhat.com)
- bump(k8s.io/kubernetes): 9da202e242d8ceedb549332fb31bf1a933a6c6b6
  (deads@redhat.com)
- bump(github.com/docker/docker): 0f5c9d301b9b1cca66b3ea0f9dec3b5317d3686d
  (deads@redhat.com)
- bump(github.com/coreos/go-systemd): b4a58d95188dd092ae20072bac14cece0e67c388
  (deads@redhat.com)
- bump(github.com/coreos/etcd): e0c7768f94cdc268b2fce31ada1dea823f11f505
  (deads@redhat.com)
- describe transitivity of bump commits (deads@redhat.com)
- clean godeps.json (deads@redhat.com)
- transitive bump checker (jliggitt@redhat.com)
- Clarify how to enable coverage report (rhcarvalho@gmail.com)
- Include offset of JSON syntax error (rhcarvalho@gmail.com)
- Move env and volume to a new 'oc set' subcommand (ccoleman@redhat.com)
- Make clean up before test runs more consistent (rhcarvalho@gmail.com)
- Prevent header and toolbar flicker for empty project (spadgett@redhat.com)
- Add GitLab OAuth identity provider (fabio@fh1.ch)
- release notes: incorrect field names will be rejected (pweil@redhat.com)
- Rename system:humans group to system:authenticated:oauth
  (jliggitt@redhat.com)
- Check index.docker.io/v1 when auth.docker.io/token has no auth
  (ccoleman@redhat.com)
- Update markup in chromeless templates to fix log scrolling issues
  (admin@benjaminapetersen.me)
- Adding missing bindata.go (rhamilto@redhat.com)
- Missing loading message on browse pages, only show tables on details tabs
  (jforrest@redhat.com)
- Set up API service and resourceGroupVersion helpers (jliggitt@redhat.com)
- Removing the transition on .sidebar-left for cleaner rendering on resize
  (rhamilto@redhat.com)
- Fix of project name alignment in IE, fixes bug 1304228 (sgoodwin@redhat.com)
- Test insecure TLS without CA for import (ccoleman@redhat.com)
- Admission control plugin to override run-once pod ActiveDeadlineSeconds
  (cewong@redhat.com)
- Fix problem with iOS zoom using the YAML editor (spadgett@redhat.com)
- Web console: editing compute resources limits (spadgett@redhat.com)
- Align edit build config styles with other edit pages (spadgett@redhat.com)
- Move build "rebuild" button to primary actions (spadgett@redhat.com)
- UPSTREAM: 16146: Fix validate event for non-namespaced kinds
  (deads@redhat.com)
- Bug 1304635: fix termination type for oc create route reencrypt
  (mkargaki@redhat.com)
- Bug 1304604: add missing route generator param for path (mkargaki@redhat.com)
- Support readiness checking on recreate strategy (ccoleman@redhat.com)
- Allow values as arguments in oc process (ffranz@redhat.com)
- Implement a mid hook for recreate deployments (ccoleman@redhat.com)
- Allow new-app to create test deployments (ccoleman@redhat.com)
- Force mount path to word break so it works at mobile (jforrest@redhat.com)
- Bug fix:  adding Go installation step, formatting fix (rhamilto@redhat.com)
- Modify buildConfig from web console (jhadvig@redhat.com)
- Bug fixes:  broken link, formatting fixes, addition of missing step
  (rhamilto@redhat.com)
- Preserve labels and annotations during reconcile scc (agladkov@redhat.com)
- Use tags consistently in top-level extended tests descriptions
  (mfojtik@redhat.com)
- Improve namer.GetName (rhcarvalho@gmail.com)
- Fix args check for role and scc modify (nakayamakenjiro@gmail.com)
- UPSTREAM: 19490: Don't print hairpin_mode error when not using Linux bridges
  (danw@gnome.org)
- added property deduping for junitreport (skuznets@redhat.com)
- Fixes #6797  - router flake due to some refactoring done. The probe needs an
  initial delay to allow the router to start up + harden the tests a lil' more
  by waiting for the router to come up and become available.
  (smitram@gmail.com)
- remove unused flag (pweil@redhat.com)
- reverted typo in extended cmd (skuznets@redhat.com)
- Fix kill_all_processes on OS X (cewong@redhat.com)
- declared variables better for RHEL (skuznets@redhat.com)

* Thu Feb 04 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.900
- made login fail with bad token (skuznets@redhat.com)
- Support test deployments (ccoleman@redhat.com)
- logs: return application logs for dcs (mkargaki@redhat.com)
- Constants in web console can be customized with JS extensions
  (ffranz@redhat.com)
- About page, configurable cli download links (ffranz@redhat.com)
- Preserve existing oauth client secrets on startup (jliggitt@redhat.com)
- Bug 1298750 - Force IE document mode to be edge (jforrest@redhat.com)
- Only pass --ginkgo.skip when focus is absent (ccoleman@redhat.com)
- Unify new-app Resolver and Searcher (ccoleman@redhat.com)
- UPSTREAM: 20053: Don't duplicate error prefix (ccoleman@redhat.com)
- new-app should chase a defaulted "latest" tag to the stable ref
  (ccoleman@redhat.com)
- Update web console tab style (spadgett@redhat.com)
- add list feature (tdawson@redhat.com)
- apply builder pod cgroup limits to launched containers (bparees@redhat.com)
- Remove transition, no longer needed, that causes Safari mobile menu flicker
  https://github.com/openshift/origin/issues/6958 Remove extra alert from
  builds page Make spacing consistent by moving <h1>, actions, and labels into
  middle-header Add missing btn default styling to copy-to-clipboard
  (sgoodwin@redhat.com)
- Document how to run test/cmd tests in development (rhcarvalho@gmail.com)
- Use an insecure TLS config for insecure: true during import
  (ccoleman@redhat.com)
- Only load secrets if import needs them (ccoleman@redhat.com)
- Fix both header dropdowns staying open (jforrest@redhat.com)
- script to check our images for security updates (tdawson@redhat.com)
- Add patching tests (jliggitt@redhat.com)
- Build defaults and build overrides admission plugins (cewong@redhat.com)
- Initial build http proxy admission control plugin (deads@redhat.com)
- Bug 1254431 - fix display of ICTs to handle the from subobject
  (jforrest@redhat.com)
- Bug 1275902 - fix help text for name field on create from image
  (jforrest@redhat.com)
- Fix display when build has not started, no startTimestamp exists
  (jforrest@redhat.com)
- Bug 1291535 - alignment of oc commands in next steps page is wrong
  (jforrest@redhat.com)
- UPSTREAM: ugorji/go: <carry>: Fix empty list/map decoding
  (jliggitt@redhat.com)
- Updates to console theme (sgoodwin@redhat.com)
- Bug 1293578 - The Router liveness/readiness probes should always use
  localhost (bleanhar@redhat.com)
- handle .dockercfg and .dockerconfigjson independently (bparees@redhat.com)
- ImageStreamImage returns incorrect image info (ccoleman@redhat.com)
- Replace NamedTagReference with TagReference (ccoleman@redhat.com)
- Don't fetch an is image if we are already in the process of fetching it
  (jforrest@redhat.com)
- Allow different version encoding for custom builds (nagy.martin@gmail.com)
- bump(github.com/openshift/source-to-image):
  f30208380974bdf302263c8a21b3e8a04f0bb909 (gmontero@redhat.com)
- Update java console to 1.0.42 (slewis@fusesource.com)
- UPSTREAM: 19366: Support rolling update to 0 desired replicas
  (dmace@redhat.com)
- Move graph helpers to deploy/api (ccoleman@redhat.com)
- oc: add `create route` subcommands (mkargaki@redhat.com)
- Allow images to be pulled through the Docker registry (ccoleman@redhat.com)
- Remove debug logging from project admission (ccoleman@redhat.com)
- Improve godoc and add validation tests (ccoleman@redhat.com)
- Review 1 - Added indenting and more log info (ccoleman@redhat.com)
- Watch "run" change as part of watchGroup in logViewer to ensure logs run when
  ready (admin@benjaminapetersen.me)
- bump(github.com/openshift/source-to-image):
  91769895109ea8f193f41bc0e2eb6ba83b30a894 (mfojtik@redhat.com)
- Add API Group to UI config (jliggitt@redhat.com)
- Add a customizable interstitial page to select login provider
  (jforrest@redhat.com)
- Update help for docker/config.json secrets (jliggitt@redhat.com)
- Bug 1303012 - Validate the build secret name in new-build
  (mfojtik@redhat.com)
- Add support for coalescing router reloads:   *  second implementation with a
  rate limiting function.   *  Fixes as per @eparis and @smarterclayton review
  comments.      Use duration instead of string/int values. This also updates
  the      usage to only allow values that time.ParseDuration accepts via
  either the infra router command line or via the RELOAD_INTERVAL
  environment variables. (smitram@gmail.com)
- bump(github.com/openshift/openshift-sdn):
  04aafc3712ec4d612f668113285370f58075e1e2 (maszulik@redhat.com)
- Update java console to 1.0.40 (slewis@fusesource.com)
- update kindToResource to match upstream (pweil@redhat.com)
- tweaks and explanation to use move-upstream.sh for rebase (deads@redhat.com)
- Delay fetching of logs on pod, build & deployment until logs are ready to run
  (admin@benjaminapetersen.me)
- bump(github.com/openshift/source-to-image):
  56dd02330716bd0ed94b87236a9989933b490237 (vsemushi@redhat.com)
- Fix js error in truncate directive (jforrest@redhat.com)
- make example imagesource path relative (bparees@redhat.com)
- Add a crashlooping error to oc status and a suggestion (ccoleman@redhat.com)
- Update the image import controller to schedule recurring import
  (ccoleman@redhat.com)
- oc: re-use edit from kubectl (mkargaki@redhat.com)
- oc: support `logs -p` for builds and deployments (mkargaki@redhat.com)
- api: enable serialization tests for Pod{Exec,Attach}Options
  (mkargaki@redhat.com)
- Project request quota admission control (cewong@redhat.com)
- Add environment to the relevant browse pages (jforrest@redhat.com)
- Make image stream import level driven (ccoleman@redhat.com)
- Add support to cli to set and display scheduled flag (ccoleman@redhat.com)
- Add a bucketed, rate-limited queue for periodic events (ccoleman@redhat.com)
- Allow admins to allow unlimited imported tags (ccoleman@redhat.com)
- Add server API config variables to control scheduling (ccoleman@redhat.com)
- API types for addition of scheduled to image streams (ccoleman@redhat.com)
- Update to fedora 23 (dmcphers@redhat.com)
- Edit project display name & description via settings page
  (admin@benjaminapetersen.me)
- UPSTREAM: <carry>: enable daemonsets by default (pweil@redhat.com)
- enable daemonset (pweil@redhat.com)
- oc: generate path-based routes in expose (mkargaki@redhat.com)
- Generated docs, swagger, completions, conversions (jliggitt@redhat.com)
- API group enablement for master/controllers (jliggitt@redhat.com)
- Explicit API version during login (jliggitt@redhat.com)
- API Group Version changes (maszulik@redhat.com)
- Make etcd registries consistent, updates for etcd test tooling changes
  (maszulik@redhat.com)
- API registration (maszulik@redhat.com)
- Change validation to use field.Path and field.ErrorList (maszulik@redhat.com)
- Determine PublicAddress automatically if masterIP is empty or loopback
  (jliggitt@redhat.com)
- Add origin client negotiation (jliggitt@redhat.com)
- Boring rebase changes (jliggitt@redhat.com)
- UPSTREAM: <drop>: Copy kube artifacts (jliggitt@redhat.com)
- UPSTREAM: 20157: Test specific generators for kubectl (jliggitt@redhat.com)
- UPSTREAM: <carry>: allow hostDNS to be included along with ClusterDNS setting
  (maszulik@redhat.com)
- UPSTREAM: 20093: Make annotate and label fall back to replace on patch
  compute failure (jliggitt@redhat.com)
- UPSTREAM: 19988: Fix kubectl annotate and label to use versioned objects when
  operating (maszulik@redhat.com)
- UPSTREAM: <carry>: Keep default generator for run 'run-pod/v1'
  (jliggitt@redhat.com)
- UPSTREAM: <drop>: stop registering versions in reverse order
  (jliggitt@redhat.com)
- UPSTREAM: 19892: Add WrappedRoundTripper methods to round trippers
  (jliggitt@redhat.com)
- UPSTREAM: 19887: Export transport constructors (jliggitt@redhat.com)
- UPSTREAM: 19866: Export PrintOptions struct (jliggitt@redhat.com)
- UPSTREAM: <carry>: remove types.generated.go (jliggitt@redhat.com)
- UPSTREAM: openshift/openshift-sdn: 253: update to latest client API
  (maszulik@redhat.com)
- UPSTREAM: emicklei/go-restful: <carry>: Add "Info" to go-restful ApiDecl
  (ccoleman@redhat.com)
- UPSTREAM: 17922: <partial>: Allow additional groupless versions
  (jliggitt@redhat.com)
- UPSTREAM: 20095: Restore LoadTLSFiles to client.Config (maszulik@redhat.com)
- UPSTREAM: 18653: Debugging round tripper should wrap CancelRequest
  (ccoleman@redhat.com)
- UPSTREAM: 18541: Allow node IP to be passed as optional config for kubelet
  (rpenta@redhat.com)
- UPSTREAM: <carry>: Tolerate node ExternalID changes with no cloud provider
  (sross@redhat.com)
- UPSTREAM: 19481: make patch call update admission chain after applying the
  patch (deads@redhat.com)
- UPSTREAM: revert: fa9f3ea88: coreos/etcd: <carry>: etcd is using different
  version of ugorji (jliggitt@redhat.com)
- UPSTREAM: 18083: Only attempt PV recycling/deleting once, else fail
  permanently (jliggitt@redhat.com)
- UPSTREAM: 19239: Added missing return statements (jliggitt@redhat.com)
- UPSTREAM: 18042: Add cast checks to controllers to prevent nil panics
  (jliggitt@redhat.com)
- UPSTREAM: 18165: fixes get --show-all (ffranz@redhat.com)
- UPSTREAM: 18621: Implement GCE PD dynamic provisioner. (jsafrane@redhat.com)
- UPSTREAM: 18607: Implement OpenStack Cinder dynamic provisioner.
  (jsafrane@redhat.com)
- UPSTREAM: 18601: Implement AWS EBS dynamic provisioner. (jsafrane@redhat.com)
- UPSTREAM: 18522: Close web socket watches correctly (jliggitt@redhat.com)
- UPSTREAM: 17590: correct homedir on windows (ffranz@redhat.com)
- UPSTREAM: 16964: Preserve int64 data when unmarshaling (jliggitt@redhat.com)
- UPSTREAM: <carry>: allow specific, skewed group/versions
  (jliggitt@redhat.com)
- UPSTREAM: 16667: Make HPA Controller use Namespacers (jliggitt@redhat.com)
- UPSTREAM: <carry>: OpenShift 3.0.2 nodes report v1.1.0-alpha
  (ccoleman@redhat.com)
- UPSTREAM: 16067: Provide a RetryOnConflict helper for client libraries
  (maszulik@redhat.com)
- UPSTREAM: 12221: Allow custom namespace creation in e2e framework
  (deads@redhat.com)
- UPSTREAM: 15451: <partial>: Add our types to kubectl get error
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: add kubelet timeouts (maszulik@redhat.com)
- UPSTREAM: 8890: Allowing ActiveDeadlineSeconds to be updated for a pod
  (maszulik@redhat.com)
- UPSTREAM: <carry>: tweak generator to handle conversions in other packages
  (deads@redhat.com)
- UPSTREAM: <carry>: Suppress aggressive output of warning
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: v1beta3 (deads@redhat.com)
- UPSTREAM: <carry>: support pointing oc exec to old openshift server
  (deads@redhat.com)
- UPSTREAM: <carry>: Back n forth downward/metadata conversions
  (deads@redhat.com)
- UPSTREAM: <carry>: update describer for dockercfg secrets (deads@redhat.com)
- UPSTREAM: <carry>: reallow the ability to post across namespaces in api
  (pweil@redhat.com)
- UPSTREAM: <carry>: Add deprecated fields to migrate 1.0.0 k8s v1 data
  (jliggitt@redhat.com)
- UPSTREAM: <carry>: SCC (deads@redhat.com)
- UPSTREAM: <carry>: Disable UIs for Kubernetes and etcd (deads@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  eda3808d8fe615229f168661fea021a074a34750 (jliggitt@redhat.com)
- bump(github.com/ugorji/go): f1f1a805ed361a0e078bb537e4ea78cd37dcf065
  (maszulik@redhat.com)
- bump(github.com/emicklei/go-restful):
  777bb3f19bcafe2575ffb2a3e46af92509ae9594 (maszulik@redhat.com)
- bump(github.com/coreos/go-systemd): 97e243d21a8e232e9d8af38ba2366dfcfceebeba
  (maszulik@redhat.com)
- bump(github.com/coreos/go-etcd): 003851be7bb0694fe3cc457a49529a19388ee7cf
  (maszulik@redhat.com)
- bump(k8s.io/kubernetes): 4a65fa1f35e98ae96785836d99bf4ec7712ab682
  (jliggitt@redhat.com)
- fix sample usage (bparees@redhat.com)
- Add needed features (tdawson@redhat.com)
- Disable replication testing for MySQL 5.5 (nagy.martin@gmail.com)
- Install iptables-services for dev & dind clusters (marun@redhat.com)
- Check for forbidden error on ImageStreamImport client (cewong@redhat.com)
- Sanitize S2IBuilder tests (rhcarvalho@gmail.com)
- Add container from annotations to options for build log to generate kibana
  url (admin@benjaminapetersen.me)
- status: Report path-based passthrough terminated routes (mkargaki@redhat.com)
- Add extended test for MongoDB. (vsemushi@redhat.com)
- Simplify Makefile for limited parallelization (ccoleman@redhat.com)
- If release binaries exist, extract them instead of building
  (ccoleman@redhat.com)
- Webhooks: use constant-time string secret comparison (elyscape@gmail.com)
- bump(github.com/openshift/source-to-image):
  78f4e4fe283bd9619804da9e929c61f655df6d06 (bparees@redhat.com)
- Send error output from verify-gofmt to stderr to allow piping to xargs to
  clean up (jliggitt@redhat.com)
- Implement submodule init/update in s2i (christian@paral.in)
- guard openshift resource dump (skuznets@redhat.com)
- Add endpoints to oc describe route (agladkov@redhat.com)
- Use docker registry contants (miminar@redhat.com)
- deployapi: remove obsolete templateRef reference (mkargaki@redhat.com)
- Do not remove image after Docker build (rhcarvalho@gmail.com)
- If Docker is installed, run the e2e/integ variant automatically
  (ccoleman@redhat.com)
- UPSTREAM: <drop>: (do not merge) Make upstream tests pass on Mac
  (ccoleman@redhat.com)
- test/cmd/basicresources.sh fails on macs (ccoleman@redhat.com)
- Remove Travis + assets workarounds (ccoleman@redhat.com)
- Implement authenticated image import (ccoleman@redhat.com)
- refactored test-cmd core to use os::cmd functions (skuznets@redhat.com)
- Move limit range help text outside of header on settings page
  (spadgett@redhat.com)
- hello-openshift example: make the ports configurable (v.behar@free.fr)
- new-build support for image source (cewong@redhat.com)
- bump(github.com/blang/semver):31b736133b98f26d5e078ec9eb591666edfd091f
  (ccoleman@redhat.com)
- Do not retry if the UID changes on import (ccoleman@redhat.com)
- Make import-image a bit more flexible (ccoleman@redhat.com)
- UPSTREAM: <drop>: Allow client transport wrappers to support CancelRequest
  (ccoleman@redhat.com)
-  bump(github.com/fsouza/go-dockerclient)
  25bc220b299845ae5489fd19bf89c5278864b050 (bparees@redhat.com)
- restrict project requests to human users: ones with oauth tokens
  (deads@redhat.com)
- Improve display of quota and limits in web console (spadgett@redhat.com)
- cmd tests: fix diagnostics tests (lmeyer@redhat.com)
- Allow --all-namespaces on status (ccoleman@redhat.com)
- don't fail for parsing error (pweil@redhat.com)
- accept diagnostics as args (deads@redhat.com)
- mark validation commands as deprecated (skuznets@redhat.com)
- fix template processing (deads@redhat.com)
- deployapi: make dc selector optional (mkargaki@redhat.com)
- Not every registry has library (miminar@redhat.com)
- Set REGISTRY_HTTP_SECRET (agladkov@redhat.com)
- homogenize tmpdirs (skuznets@redhat.com)
- Add support for --build-secret to oc new-build command (mfojtik@redhat.com)
- WIP: Add extended test for source build secrets (mfojtik@redhat.com)
- Allow to specify secrets used for the build in source (vsemushi@redhat.com)
- Fixed typo in junitreport properties (maszulik@redhat.com)
- Suppress tooltip when hovering over cut-line at top of pod donut
  (spadgett@redhat.com)
- Described oadm prune builds command. (vsemushi@redhat.com)
- Block build cloning when BC is paused (nagy.martin@gmail.com)
- cleanup of extended cleanup traps (deads@redhat.com)
- oadm: should have prune groups cmd (mkargaki@redhat.com)
- Improve message for deprecated build-logs option. (vsemushi@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  fccee2120e9e8662639df6b40f4e1adf07872105 (danw@redhat.com)
- Set REGISTRY_HTTP_ADDR to first specified port (agladkov@redhat.com)
- ignore ctags file (ian.miell@gmail.com)
- Initial addition of image promotion proposal (mfojtik@redhat.com)
- oadm: bump prune deployments description/examples (mkargaki@redhat.com)
- Add annotation information when describing new-app results
  (ccoleman@redhat.com)
- made system logger get real error code (skuznets@redhat.com)
- wire an etcd dump into an API server for debugging (deads@redhat.com)
- add extended all.sh to allow [extended:all] to run all buckets
  (deads@redhat.com)
- diagnostics: add diagnostics from pod perspective (lmeyer@redhat.com)
- Use interfaces to pass config data to admission plugins (cewong@redhat.com)
- Create registry dc with readiness probe (miminar@redhat.com)
- removed global project cache (skuznets@redhat.com)
- Improve data population (ccoleman@redhat.com)
- stop status from checking SA mountable secrets (deads@redhat.com)
- remove outdated validation (skuznets@redhat.com)
- return a error+recommendation to the user on multiple matches, prioritize
  imagestream matches over annotation matches (bparees@redhat.com)
- make test-integration use our etcd (deads@redhat.com)
- Updated troubleshooting guide with information about insecure-registry
  (maszulik@redhat.com)
- DuelingRepliationControllerWarning -> DuelingReplicationControllerWarning
  (ian.miell@gmail.com)
- Updated generated docs (miminar@redhat.com)
- Fix hack/test-go.sh testing packages recursively (jliggitt@redhat.com)
- Expose admission control plugins list and config in master configuration
  (cewong@redhat.com)
- put openshift-f5-router in origin.spec (tdawson@redhat.com)
- add etcdserver launch mechanism (deads@redhat.com)
- Update minimum Docker version (rhcarvalho@gmail.com)
- added system logging utility (skuznets@redhat.com)
- Described oadm prune images command (miminar@redhat.com)
- add caps defaulting (pweil@redhat.com)
- hack/move-upstream now supports extracting true commits (ccoleman@redhat.com)
- bump(github.com/openshift/source-to-image):
  8ec5b0f51f8baa30159c1d8cceb62126bba6f384 (mfojtik@redhat.com)
- bump(fsouza/go-dockerclient): 299d728486342c894e7fafd68e3a4b89623bef1d
  (mfojtik@redhat.com)
- Update lodash to v 3.10.1 (admin@benjaminapetersen.me)
- Implement BuildConfig reaper (nagy.martin@gmail.com)
- clarify build-started message based on buildconfig triggers
  (bparees@redhat.com)
- Lock bootstrap-hover-dropdown version to 2.1.3 (spadgett@redhat.com)
- Enable swift storage backend for the registry (miminar@redhat.com)
- bump(ncw/swift): c54732e87b0b283d1baf0a18db689d0aea460ba3
  (miminar@redhat.com)
- Enable cloudfront storage driver in dockerregistry (miminar@redhat.com)
- bump(AdRoll/goamz/cloudfront): aa6e716d710a0c7941cb2075cfbb9661f16d21f1
  (miminar@redhat.com)
- Use const values as string for defaultLDAP(S)Port (nakayamakenjiro@gmail.com)
- Replace --master option with --config (miminar@redhat.com)
- Enable Azure Blob Storage (spinolacastro@gmail.com)
- bump(Azure/azure-sdk-for-go/storage):
  97d9593768bbbbd316f9c055dfc5f780933cd7fc (spinolacastro@gmail.com)
- Simplify reading of random bytes (rhcarvalho@gmail.com)
- stop etcd from retrying failures (deads@redhat.com)
- created structure for whitelisting directories for govet shadow testing
  (skuznets@redhat.com)
- Adapt to etcd changes from v2.1.2 to v2.2.2 (ccoleman@redhat.com)
- UPSTREAM: coreos/etcd: <carry>: etcd is using different version of ugorji
  (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):b4bddf685b26b4aa70e939445044bdeac822d042
  (ccoleman@redhat.com)
- bump(AdRoll/goamz): aa6e716d710a0c7941cb2075cfbb9661f16d21f1
  (miminar@redhat.com)
- Fix typo as per review comments from @Miciah (smitram@gmail.com)
- Fix old PR #4282 code and tests to use new layout (Spec.*) and fix gofmt
  errors. (smitram@gmail.com)
- Prohibit passthrough route with path (miciah.masters@gmail.com)
- Don't say "will retry in 5s seconds" in push failure message
  (danw@redhat.com)
- fix contrib doc (pweil@redhat.com)

* Mon Jan 18 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.5
- More fixes and tweeks (tdawson@redhat.com)
- Lock ace-builds version to 1.2.2 (spadgett@redhat.com)
- Do not check builds/details in build by strategy admission control
  (cewong@redhat.com)

* Sat Jan 16 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.4
- Fix up net.bridge.bridge-nf-call-iptables after kubernetes breaks it
  (danw@redhat.com)
- admission tests and swagger (pweil@redhat.com)
- UPSTREAM: <carry>: capability defaulting (pweil@redhat.com)
- Fix logic to add Dockerfile to BuildConfig (rhcarvalho@gmail.com)
- Add TIMES=N to rerun integration tests for flakes (ccoleman@redhat.com)
- Fix route serialization flake (mkargaki@redhat.com)
- STI -> S2I (dmcphers@redhat.com)
- Enable PostgreSQL replication tests for RHEL images (nagy.martin@gmail.com)

* Thu Jan 14 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.3
- Bug 1298457 - The link to pv doc is wrong (bleanhar@redhat.com)
- Fix typo in build generator (mfojtik@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  eda3808d8fe615229f168661fea021a074a34750 (dcbw@redhat.com)
- Auto generated bash completions for node-ip kubelet config option
  (rpenta@redhat.com)
- Use KubeletServer.NodeIP instead of KubeletServer.HostnameOverride to set
  node IP (rpenta@redhat.com)
- UPSTREAM: <carry>: Tolerate node ExternalID changes with no cloud provider
  (sross@redhat.com)
- Update certs for router tests (jliggitt@redhat.com)
- Retry adding roles to service accounts in conflict cases
  (jliggitt@redhat.com)
- Include update operation in build admission controller (cewong@redhat.com)
- UPSTREAM: 18541: Allow node IP to be passed as optional config for kubelet
  (rpenta@redhat.com)
- Fix test fixture fields (mkargaki@redhat.com)
- Make `oc cancel-build` to be suggested for `oc stop-build`.
  (vsemushi@redhat.com)
- The default codec should be v1.Codec, not v1beta3 (ccoleman@redhat.com)

* Wed Jan 13 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.2
- Wait for access tokens to be available in clustered etcd
  (jliggitt@redhat.com)
- Add a new image to be used for testing (ccoleman@redhat.com)
- Bug 1263609 - fix oc rsh usage (ffranz@redhat.com)
- Fix HPA default policy (jliggitt@redhat.com)
- oc rsync: expose additional rsync flags (cewong@redhat.com)
- Bug 1248463 - fixes exec help (ffranz@redhat.com)
- Bug 1273708 - mark --all in oc export deprecated in help (ffranz@redhat.com)
- Fix tests for route validation changes (mkargaki@redhat.com)
- Make new-build output BC with multiple sources (rhcarvalho@gmail.com)
- Remove code duplication (rhcarvalho@gmail.com)
- Require tls termination in route tls configuration (mkargaki@redhat.com)
- Fix nw extended test support for skipping build (marun@redhat.com)
- Fix broken switch in usageWithUnits filter (spadgett@redhat.com)
- UPSTREAM: 19481: make patch call update admission chain after applying the
  patch (deads@redhat.com)
- diagnostics: logs and units for origin (lmeyer@redhat.com)
- Update java console to 1.0.39 (slewis@fusesource.com)
- extended tests for jenkins openshift V3 plugin (gmontero@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  da8ad5dc5c94012eb222221d909b2b6fa678500f (dcbw@redhat.com)
- Update for openshift-sdn script installation changes (danw@redhat.com)
- use direct mount for etcd data (deads@redhat.com)
- mark image input source as experimental (bparees@redhat.com)
- made large file behavior smarter (skuznets@redhat.com)
- Revert "Allow parallel image stream importing" (jordan@liggitt.net)
- version now updates FROM (tdawson@redhat.com)
- Fix deployment CLI ops link and make all doc links https
  (jforrest@redhat.com)
- Fix detection of Python projects (rhcarvalho@gmail.com)
- add probe for mongodb template (haowang@redhat.com)
- deal with RawPath field added to url.URL in go1.5 (gmontero@redhat.com)
- Include kube e2e service tests in networking suite (marun@redhat.com)
- Fix replication controller usage in kube e2e tests (marun@redhat.com)
- Enable openshift-sdn sdn node by default (marun@redhat.com)
- Fix dind compatibility with centos/rhel (marun@redhat.com)
- Fix dind compatibility with centos/rhel (marun@redhat.com)
- added junitreport tool (skuznets@redhat.com)
- diagnostics: list diagnostic names in long desc (lmeyer@redhat.com)
- Increase web console e2e login timeout (spadgett@redhat.com)
- Persistent volume claims on the web console (ffranz@redhat.com)
- update extended test to point to correct version tool (skuznets@redhat.com)
- add warning about root user in images (bparees@redhat.com)
- allow parallel image streams (deads@redhat.com)
- Suppress conflict error logging when adding SA role bindings
  (jliggitt@redhat.com)
- BuildConfig envVars in wrong structure in sti build template
  (jhadvig@redhat.com)
- Remove unnecessary type conversions (rhcarvalho@gmail.com)

* Thu Jan 07 2016 Troy Dawson <tdawson@redhat.com> 3.1.1.1
- add option --insecure for oc import-image
  (haoran@dhcp-129-204.nay.redhat.com)
- new-app: search local docker daemon if registry search fails
  (cewong@redhat.com)
- Fix breadcrumb on next steps page (spadgett@redhat.com)
- Enable DWARF debuginfo (tdawson@redhat.com)
- update scripts to respect TMPDIR (deads@redhat.com)
- handle missing dockerfile with docker strategy (bparees@redhat.com)
- Bump kubernetes-topology-graph to 0.0.21 (spadgett@redhat.com)
- Print more enlightening string if test fails (rhcarvalho@gmail.com)
- Add to project catalog legend and accessibility fixes (spadgett@redhat.com)
- tolerate spurious failure during test setup (deads@redhat.com)
- fixed readiness endpoint route listing (skuznets@redhat.com)
- moved tools from cmd/ to tools/ (skuznets@redhat.com)
- Fix scale up button tooltip (spadgett@redhat.com)
- fix deploy test to use actual master (deads@redhat.com)
- Use angular-bootstrap dropdown for user menu (spadgett@redhat.com)
- added bash autocompletion for ldap sync config (skuznets@redhat.com)
- Don't allow clock icon to wrap in image tag table (spadgett@redhat.com)
- diagnostics: improve wording of notes (lmeyer@redhat.com)
- diagnostics: improve master/node config warnings (lmeyer@redhat.com)
- Show scalable deployments on web console overview even if not latest
  (spadgett@redhat.com)
- Use angular-bootstrap uib-prefixed components (spadgett@redhat.com)
- write output to file in e2e core (skuznets@redhat.com)
- Make web console alerts dismissable (spadgett@redhat.com)
- Reenabled original registry's /healthz route (miminar@redhat.com)
- fixed TestEditor output (skuznets@redhat.com)
- added readiness check to LDAP server pod (skuznets@redhat.com)
- diagnostics: avoid some redundancy (lmeyer@redhat.com)
- update auth tests to use actual master (deads@redhat.com)
- fix non-compliant build integration tests (deads@redhat.com)
- Wait until animation finishes to call chart.flush() (spadgett@redhat.com)
- oc: Add more doc and examples in oc get (mkargaki@redhat.com)
- Make KUBE_TIMEOUT take a duration (rhcarvalho@gmail.com)
- examples: Update resource quota README (mkargaki@redhat.com)
- promoted group prune and sync from experimental (skuznets@redhat.com)
- Various accessibility fixes, bumps angular-bootstrap version
  (jforrest@redhat.com)
- Avoid scrollbar flicker on build trends tooltip (spadgett@redhat.com)
- Show empty RCs in some cases on overview when no service
  (spadgett@redhat.com)
- make os::cmd::try_until* output smarter (skuznets@redhat.com)
- Fix tito ldflag manipulation at tag time (sdodson@redhat.com)
- graphapi: Remove dead code and add godoc (mkargaki@redhat.com)
- describe DockerBuildStrategy.DockerfilePath (v.behar@free.fr)
- integration-tests: retry get image on not found error (miminar@redhat.com)
- Wait for user permissions in test-cmd.sh (jliggitt@redhat.com)
- Wait for bootstrap policy on startup (jliggitt@redhat.com)
- Shorten image importer dialTimeout to 5 seconds (jliggitt@redhat.com)
- Increase specificity of CSS .yaml-mode .ace-numeric style
  (spadgett@redhat.com)
- Fix typo in example `oc run` command. (dusty@dustymabe.com)
- deployapi: Necessary refactoring after updating the internal objects
  (mkargaki@redhat.com)
- deployapi: Update generated conversions and deep copies (mkargaki@redhat.com)
- deployapi: Update manual conversions (mkargaki@redhat.com)
- deployapi: Refactor internal objects to match versioned (mkargaki@redhat.com)
- Allow using an image as source for a build (cewong@redhat.com)
- Updating after real world tests (tdawson@redhat.com)
- added os::cmd readme (skuznets@redhat.com)
- fixed caching bug in ldap sync (skuznets@redhat.com)
- update deltafifo usage to match upstream changes (bparees@redhat.com)
- UPSTREAM: 14881: fix delta fifo & various fakes for go1.5.1
  (maszulik@redhat.com)

* Sat Dec 19 2015 Scott Dodson <sdodson@redhat.com> 3.1.1.0
- Fix tito ldflag manipulation at tag time (sdodson@redhat.com)
- Fix oc status unit test (mkargaki@redhat.com)
- Improve web console scaling (spadgett@redhat.com)
- UPSTREAM: <drop>: fixup for 14537 (mturansk@redhat.com)
- Add bash auto-completion for oc, oadm, and openshift to the environments
  created by Vagrant. (bbennett@redhat.com)
- Fix github link when contextDir is set but not git ref (jforrest@redhat.com)
- install which in the base image (bparees@redhat.com)
- UPSTREAM: 18165: fixes get --show-all (ffranz@redhat.com)
- fix extra indentation for bare builds (deads@redhat.com)
- Edit resource YAML in the web console (spadgett@redhat.com)
- Use DeepEqual instead of field by field comparison (rhcarvalho@gmail.com)
- Fix regression: new-app with custom Git ref (rhcarvalho@gmail.com)
- Increase debug logging for image import controller (jliggitt@redhat.com)
- Gather logs for test-cmd (jliggitt@redhat.com)
- UPSTREAM: 18621: Implement GCE PD dynamic provisioner. (jsafrane@redhat.com)
- UPSTREAM: 17747: Implement GCE PD disk creation. (jsafrane@redhat.com)
- UPSTREAM: 18607: Implement OpenStack Cinder dynamic provisioner.
  (jsafrane@redhat.com)
- UPSTREAM: 18601: Implement AWS EBS dynamic provisioner. (jsafrane@redhat.com)
- Fix test-cmd.sh on OSX (jliggitt@redhat.com)
- Web console: Fix edge case scaling to 0 replicas (spadgett@redhat.com)
- correctly determine build pushability for status (deads@redhat.com)
- move imagestream creation to more sensible spot (bparees@redhat.com)
- Avoid generating tokens with leading dashes (jliggitt@redhat.com)
- Show 'none' when there is no builder image. (rhcarvalho@gmail.com)
- status: Add more details for a broken dc trigger (mkargaki@redhat.com)
- diagnose timeouts correctly in error message (skuznets@redhat.com)
- Fix up ClusterNetwork validation. (danw@redhat.com)
- Show warning when --env is used in start-build with binary source
  (mfojtik@redhat.com)
- Fix deployements/stragies anchor link (christophe@augello.be)
- don't output empty strings in logs (bparees@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  e7f0d8be285f73c896ff19455bd03d5189cbe5e6 (dcbw@redhat.com)
- Fix race between Kubelet initialization and plugin creation (dcbw@redhat.com)
- Output more helpful cancel message (jliggitt@redhat.com)
- Use the iptables-based proxier instead of the userland one (danw@redhat.com)
- Delete tag if its history is empty (miminar@redhat.com)
- Remove redundant admin routes (miminar@redhat.com)
- Reenabled registry dc's liveness probe (miminar@redhat.com)
- Refactor of OSO's code (miminar@redhat.com)
- Refactor dockerregistry: prevent a warning during startup
  (miminar@redhat.com)
- Refactor dockerregistry: unify logging style with upstream's codebase
  (miminar@redhat.com)
- Refactor dockerregistry: use registry's own health check (miminar@redhat.com)
- Refactor dockerregistry: adapt to upstream changes (miminar@redhat.com)
- Registry refactor: handle REGISTRY_CONFIGURATION_PATH (miminar@redhat.com)
- unbump(code.google.com/p/go-uuid/uuid): which is obsoleted
  (miminar@redhat.com)
- Always use port 53 as the dns service port. (abutcher@redhat.com)
- Fix service link on route page (spadgett@redhat.com)
- Increase contrast of web console log text (spadgett@redhat.com)
- custom Dockerfile path for the docker build (v.behar@free.fr)
- assets: Fix up the topology view icon colors and lines (stefw@redhat.com)
- Remove static nodes, deprecate --nodes (jliggitt@redhat.com)
- Add PV Provisioner Controller (mturansk@redhat.com)
- Fix for bugz https://bugzilla.redhat.com/show_bug.cgi?id=1290643   o Make
  Forwarded header value rfc7239 compliant.   o Set X-Forwarded-Proto for http
  (if insecure edge terminated routes     are allowed). (smitram@gmail.com)
- UPSTREAM: 14537: Add PersistentVolumeProvisionerController
  (mturansk@redhat.com)
- Fix test failures for scoped and host-overriden routers. (smitram@gmail.com)
- Unit test for hostname override (ccoleman@redhat.com)
- Routers should be able to override the host value on Routes
  (ccoleman@redhat.com)
- added support for recursive testing using test-go (skuznets@redhat.com)
- fixed group detection bug for LDAP prune (skuznets@redhat.com)
- removed tryuntil from test-cmd (skuznets@redhat.com)
- fixed LDAP test file extensions (skuznets@redhat.com)
- Allow image importing to work with proxy (jkhelil@gmail.com)
- oc: Use object name instead of provided arg in cancel-build
  (mkargaki@redhat.com)
- add step for getting the jenkins service ip (bparees@redhat.com)
- fix forcepull setup build config (gmontero@redhat.com)
- Add user/group reapers (jliggitt@redhat.com)
- Make project template use default rolebinding name (jliggitt@redhat.com)
- update missing imagestream message to warning (bparees@redhat.com)
- Temporary fix for systemd upgrade path issues (sdodson@redhat.com)
- Add logging for etcd integration tests (jliggitt@redhat.com)
- Fix the mysql replica extended test (mfojtik@redhat.com)
- Use service port name as route targetPort in 'oc expose service'
  (jliggitt@redhat.com)
- Refactor SCM auth and use of env vars in builders (cewong@redhat.com)
- oc: Cosmetic fixes in oc status (mkargaki@redhat.com)
- allow for test specification from command-line for test-cmd
  (skuznets@redhat.com)
- Change uuid imports to github.com/pborman/uuid (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: remove parents on delete
  (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: export app.Namespace
  (miminar@redhat.com)
- UPSTREAM: docker/distribution: <carry>: custom routes/auth
  (agoldste@redhat.com)
- UPSTREAM: docker/distribution: 1050: Exported API functions needed for
  pruning (miminar@redhat.com)
- bump(github.com/stevvooe/resumable): 51ad44105773cafcbe91927f70ac68e1bf78f8b4
  (miminar@redhat.com)
- bump(github.com/docker/distribution):
  e6c60e79c570f97ef36f280fcebed497682a5f37 (miminar@redhat.com)
- Give user suggestion about new-app on new-project (ccoleman@redhat.com)
- Controllers should always go async on start (ccoleman@redhat.com)
- Clean up docker-in-docker image (marun@redhat.com)
- doc: create glusterfs service to persist endpoints (hchen@redhat.com)
- [RPMs] Add requires on git (sdodson@redhat.com)
- added better output to test-end-to-end/core (skuznets@redhat.com)
- refactored scripts to use hack/text (skuznets@redhat.com)

* Mon Dec 14 2015 Troy Dawson <tdawson@redhat.com> 3.1.0.902
- Include LICENSE in client zips (ccoleman@redhat.com)
- Minor commit validation fixes (ironcladlou@gmail.com)
- remove build-related type fields from internal api (bparees@redhat.com)
- Add godeps commit verification (ironcladlou@gmail.com)
- Retry build logs in start build when waiting for build (mfojtik@redhat.com)
- Create proper client packages for mac and windows (ccoleman@redhat.com)
- update jenkins tutorial for using plugin (gmontero@redhat.com)
- Allow oc new-build to accept zero arguments (ccoleman@redhat.com)
- Fix fallback scaling behavior (ironcladlou@gmail.com)
- Fix deployment e2e flake (mkargaki@redhat.com)
- UPSTREAM: 18522: Close web socket watches correctly (jliggitt@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  8a7e17c0c3eea529955229dfd7b4baefad56633b (rpenta@redhat.com)
- Start SDN controller after running kubelet (rpenta@redhat.com)
- [RPMs] Cleanup kubeplugin path from old sdn-ovs installs (sdodson@redhat.com)
- Fix flakiness in builds extended tests (cewong@redhat.com)
- Add suggestions in oc status (mkargaki@redhat.com)
- status: Report tls routes with unspecified termination type
  (mkargaki@redhat.com)
- Use the dockerclient ClientFromEnv setup (ccoleman@redhat.com)
- Don't show build trends chart scrollbars if not needed (spadgett@redhat.com)
- Prevent y-axis label overlap when filtering build trends chart
  (spadgett@redhat.com)
- Packaging specfile clean up (admiller@redhat.com)
- added prune-groups; refactored rfc2307 ldapinterface (skuznets@redhat.com)
- only expand resources when strictly needed (deads@redhat.com)
- added logging of output for failed builds (ipalade@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  0f33df18b9747ebfe2c337f2bf4443b520a8f2ab (rpenta@redhat.com)
- UPSTREAM: revert: 97bd6c: <carry>: Allow pod start to be delayed in Kubelet
  (rpenta@redhat.com)
- Don't use KubeletConfig 'StartUpdates' channel for blocking kubelet
  (rpenta@redhat.com)
- after live test (tdawson@redhat.com)
- Unload filtered groups from build config chart (spadgett@redhat.com)
- Fix login tests for OSE variants (sdodson@redhat.com)
- Optionally skip builds for dev cluster provision (marun@redhat.com)
- Fix for bugz 1283952 and add a test. (smitram@gmail.com)
- more code (tdawson@redhat.com)
- more code (tdawson@redhat.com)
- Disable delete button for RCs with status replicas (spadgett@redhat.com)
- Web console support for deleting individual builds and deployments
  (spadgett@redhat.com)
- add new options (tdawson@redhat.com)
- UPSTREAM: 18065: Fixed forbidden window enforcement in horizontal pod
  autoscaler (sross@redhat.com)
- more updating of script (tdawson@redhat.com)
- first rough draft (tdawson@redhat.com)
- HACKING.md: clarify meaning and provide proper usage of OUTPUT_COVERAGE
  variable. (vsemushi@redhat.com)

* Tue Dec 08 2015 Scott Dodson <sdodson@redhat.com> 3.1.0.901
- Show build trends chart on build config page (spadgett@redhat.com)
- fix ruby-22 scl enablement (bparees@redhat.com)
- dump build logs on failure (bparees@redhat.com)
- make sure imagestreams are imported before using them with new-app
  (bparees@redhat.com)
- increase build timeout to 60mins (bparees@redhat.com)
- Update config of dind cluster image registry (marun@redhat.com)
- Allow junit output filename to be overridden (marun@redhat.com)
- improved os::cmd handling of test names, timing (skuznets@redhat.com)
- Improve deployment scaling behavior (ironcladlou@gmail.com)
- refactored test/cmd/admin to use wrapper functions (skuznets@redhat.com)
- Create sourcable bash env as part of dind cluster deploy (marun@redhat.com)

* Fri Dec 04 2015 Scott Dodson <sdodson@redhat.com> 3.1.0.900
- Update liveness/readiness probe to always use the /healthz endpoint (on the
  stats port or on port 1936 if the stats are disabled). (smitram@gmail.com)
- Fix incorrect status icon on pods page (spadgett@redhat.com)
- Update rest of controllers to use new ProjectsService controller, remove ng-
  controller directives in templates, all controllers defined in routes, minor
  update to tests (admin@benjaminapetersen.me)
- Remove the tooltip from the delete button (spadgett@redhat.com)
- Add success line to dry run (rhcarvalho@gmail.com)
- Add integration test for router healthz endpoint as per @smarterclayton
  review comments. (smitram@gmail.com)
- UPSTREAM: drop: part of upstream kube PR #15843. (avagarwa@redhat.com)
- UPSTREAM: drop: Fix kube e2e tests in origin. This commit is part of upstream
  kube PR 16360. (avagarwa@redhat.com)
- Bug 1285626 - need to handle IS tag with a from of kind DockerImage
  (jforrest@redhat.com)
- Fix for Bug 1285647 and issue 6025  - Border width will be expanded when long
  value added in Environment Variables for project  - Route link on overview is
  truncated even at wide browser widths (sgoodwin@redhat.com)
- Patch AOS tuned-profiles manpage during build (sdodson@redhat.com)
- Do not print steps for alternative output formats (rhcarvalho@gmail.com)
- UPSTREAM: 17920: Fix frequent kubernetes endpoint updates during cluster
  start (abutcher@redhat.com)
- Add additional route settings to UI (spadgett@redhat.com)
- switch hello-world tests to expect ruby-22 (bparees@redhat.com)
- Upstream: 16728: lengthened pv controller sync period to 10m
  (mturansk@redhat.com)
- Support deleting routes in web console (spadgett@redhat.com)
- oc rsync: pass-thru global command line options (cewong@redhat.com)
- Sync Dockerfile.product from dist-git (sdodson@redhat.com)
- add namespace to field selectors (pweil@redhat.com)
- Use minutes instead of seconds where possible. (vsemushi@redhat.com)
- added volume length check for overriden recycler (mturansk@redhat.com)
- make adding infrastructure SAs easier (deads@redhat.com)
- fixed origin recycler volume config w/ upstream cli flags
  (mturansk@redhat.com)
- oc: Make env use PATCH instead of PUT (mkargaki@redhat.com)
- Prompt before scaling deployments to 0 in UI (spadgett@redhat.com)
- UPSTREAM: 18000: Fix test failure due to days-in-month check. Issue #17998.
  (mkargaki@redhat.com)
- Update install for osdn plugin reorg (danw@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  0d3440e224aeb26a056c0c4c91c30fdbb59588f9 (danw@redhat.com)
- Update various templates to use status-icon (admin@benjaminapetersen.me)
- UPSTREAM: 17973: Validate pod spec.nodeName (jliggitt@redhat.com)
- Rename project service to ProjectsService, update all pre-existing occurances
  (admin@benjaminapetersen.me)
- Hide old deployments in the topology view (spadgett@redhat.com)
- refactored constructors to allow for better code resue with prune-groups
  (skuznets@redhat.com)
- Prevent identical input/output IST in oc new-build (rhcarvalho@gmail.com)
- Add aria-describedby attributes to template parameter inputs
  (spadgett@redhat.com)
- remove url workaround (now at correct s2i level) (gmontero@redhat.com)
- New flag to oc new-build to produce no output (rhcarvalho@gmail.com)
- Fix sr-only text for pod status chart (spadgett@redhat.com)
- Add README.md to examples/db-templates (rhcarvalho@gmail.com)
- Fix test registry resource location (ffranz@redhat.com)
- UPSTREAM: 17886: pod log location must validate container if provided
  (ffranz@redhat.com)
- Background the node service so we handle SIGTERM (sdodson@redhat.com)
- hack/util.sh(delete_large_and_empty_logs): optimize find usage.
  (vsemushi@redhat.com)
- Bug 1281928 - fix image stream tagging for DockerImage type images.
  (maszulik@redhat.com)
- Remove type conversion (rhcarvalho@gmail.com)
- Bug1277420 - show friendly prompt when cancelling a completed build
  (jhadvig@redhat.com)
- Add new-build flag to set output image reference (rhcarvalho@gmail.com)
- Output uppercase, hex, 2-character padded serial.txt (jliggitt@redhat.com)
- Fix test/cmd/admin.shwq (jliggitt@redhat.com)
- Fix template processing multiple values (jliggitt@redhat.com)
- CLI usability - proposed aliases (ffranz@redhat.com)
- overhaul imagestream definitions and update latest (bparees@redhat.com)
- refactored test/cmd/images to use wrapper methods (skuznets@redhat.com)
- added unit testing to existing LDAP sync code (skuznets@redhat.com)
- fix macro order for bundled listing in tito custom builder
  (admiller@redhat.com)
- refactor fedora packaging additions with tito custom builder updates
  (admiller@redhat.com)
- Fedora packaging: (admiller@redhat.com)
- Typo fixes (jhadvig@redhat.com)
- Update templates to use navigateResourceURL filter where appropriate
  (admin@benjaminapetersen.me)
- Update completions (ffranz@redhat.com)
- bump(github.com/spf13/pflag): 08b1a584251b5b62f458943640fc8ebd4d50aaa5
  (ffranz@redhat.com)
- bump(github.com/spf13/cobra): 1c44ec8d3f1552cac48999f9306da23c4d8a288b
  (ffranz@redhat.com)
- Adds .docker/config.json secret example (ffranz@redhat.com)
- refactored test/cmd/basicresources to use wrapper functions
  (skuznets@redhat.com)
- added os::cmd::try_until* (skuznets@redhat.com)
- no redistributable for either fedora or epel (tdawson@redhat.com)
- Skip Daemonset and DaemonRestart as these are not enabled yet and keep
  failing. (avagarwa@redhat.com)
- Retry failed attempts to talk to remote registry (miminar@redhat.com)
- Split UI tests into e2e vs rest_api integration suites (jforrest@redhat.com)
- refactored test/cmd/templates to use wrapper methods (skuznets@redhat.com)
- refactored test/cmd/policy to use wrapper methods (skuznets@redhat.com)
- refactored test/cmd/builds to use wrapper methods (skuznets@redhat.com)
- refactored test/cmd/newapp to use wrapper functions (skuznets@redhat.com)
- refactored test/cmd/help to use wrapper functions (skuznets@redhat.com)
- avoid: no such file error in test-cmd (deads@redhat.com)
- added ldap test client and query tests (skuznets@redhat.com)
- Link in the alert for the newly triggered build (jhadvig@redhat.com)
- reorganized ldaputil error and query code (skuznets@redhat.com)
- refactored test/cmd/export to use wrapper functions (skuznets@redhat.com)
- refactored test/cmd/deployments to use wrapper methods (skuznets@redhat.com)
- refactored test/cmd/volumes to use helper methods (skuznets@redhat.com)
- Update build revision information when building (cewong@redhat.com)
- Enable /healthz irrespective of stats port being enabled/disabled. /healthz
  is available on the stats port or the default stats port 1936 (if stats are
  turned off via --stats-port=0). (smitram@gmail.com)
- refactored test/cmd/secrets to use helper methods (skuznets@redhat.com)
- added cmd util function test to CI (skuznets@redhat.com)
- bump(gopkg.in/asn1-ber.v1): 4e86f4367175e39f69d9358a5f17b4dda270378d
  (jliggitt@redhat.com)
- bump(gopkg.in/ldap.v2): e9a325d64989e2844be629682cb085d2c58eef8d
  (jliggitt@redhat.com)
- Rename github.com/go-ldap/ldap to gopkg.in/ldap.v2 (jliggitt@redhat.com)
- bump(gopkg.in/ldap.v2): b4c9518ccf0d85087c925e4a3c9d5802c9bc7025 (package
  rename) (jliggitt@redhat.com)
- add role reaper (deads@redhat.com)
- Exposes the --token flag in login command help (ffranz@redhat.com)
- allow unknown secret types (deads@redhat.com)
- allow startup of API server only for integration tests (deads@redhat.com)
- bump(github.com/openshift/source-to-image)
  7597eaa168a670767bf2b271035d29b92ab13b5c (cewong@redhat.com)
- refactored hack/test to not use aliases, detect tty (skuznets@redhat.com)
- Refactor pkg/generate/app (rhcarvalho@gmail.com)
- add image-pusher role (deads@redhat.com)
- refactored test-cmd/edit to use new helper methods (skuznets@redhat.com)
- remove db tag from jenkins template (bparees@redhat.com)
- Change default instance size (dmcphers@redhat.com)
- bump the wait for images timeout (bparees@redhat.com)
- dump container logs when s2i incremental tests fail (bparees@redhat.com)
- test non-db sample templates also (bparees@redhat.com)
- Use correct homedir on Windows (ffranz@redhat.com)
- UPSTREAM: 17590: correct homedir on windows (ffranz@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  919e0142fe594ab5115ecf7fa3f7ad4f5810f009 (dcbw@redhat.com)
- Update to new osdn plugin API (dcbw@redhat.com)
- Accept CamelCase versions of TLS config (ccoleman@redhat.com)
- fix git-ls to leverage GIT_SSH for non-git, secret/token access
  (gmontero@redhat.com)
- Bug 1277046 - fixed tagging ImageStreamImage from the same ImageStream to
  point to original pull spec instead of the internal registry.
  (maszulik@redhat.com)
- Get rid of util.StringList (ffranz@redhat.com)
- UPSTREAM: revert: 199adb7: <drop>: add back flag types to reduce noise during
  this rebase (ffranz@redhat.com)
- added start-build parameters to cli.md (ipalade@redhat.com)
- UPSTREAM: 17567: handle the HEAD verb correctly for authorization
  (deads@redhat.com)
- examples/sample-app/README.md: fix commands to simplify user experience.
  (vsemushi@redhat.com)
- Add PostgreSQL replication tests (nagy.martin@gmail.com)
- Allow to override environment and build log level in oc start-build
  (mfojtik@redhat.com)
- Remove hook directory by default in gitserver example (cewong@redhat.com)
- wait for imagestream import before running a build in tests
  (bparees@redhat.com)
- extended tests for docker and sti bc with no outputname defined
  (ipalade@redhat.com)
- The git:// protocol with proxy is not allowed (mfojtik@redhat.com)
- Fix build controller integration test flake (cewong@redhat.com)
- add requester username to project template (deads@redhat.com)
- Fix extended test for start-build (mfojtik@redhat.com)
- prevent go panic when output not specified for build config
  (gmontero@redhat.com)
- fix start-build --from-webhook (cewong@redhat.com)
- SCMAuth: use local proxy when password length exceeds 255 chars
  (cewong@redhat.com)
- Use constants for defaults instead of strings (rhcarvalho@gmail.com)
- Add extended test for proxy (mfojtik@redhat.com)
- Cleanup extended test output some more (nagy.martin@gmail.com)
- Use router's /healtz route for health checks (miminar@redhat.com)
- Unflake MySQL extended test (nagy.martin@gmail.com)
- Add git ls-remote to validate the remote GIT repository (mfojtik@redhat.com)
- Handle openshift.io/build-config.name label as well as buildconfig.
  (vsemushi@redhat.com)
- update the readme (haoran@dhcp-129-204.nay.redhat.com)
- Completions for persistent flags (ffranz@redhat.com)
- UPSTREAM: spf13/cobra 180: fixes persistent flags completions
  (ffranz@redhat.com)
- Generated docs must use the short command name (ffranz@redhat.com)
- UPSTREAM: 17033: Fix default value for StreamingConnectionIdleTimeout
  (avagarwa@redhat.com)
- Fixes bug 1275518 https://bugzilla.redhat.com/show_bug.cgi?id=1275518
  (avagarwa@redhat.com)
- tito builder/tagger cleanup: (admiller@redhat.com)
- stop oc export from exporting SA secrets that aren't round-trippable
  (deads@redhat.com)
- accept new dockercfg format (deads@redhat.com)
- Fix alignment of icon on settings page by overriding patternfly rule
  (sgoodwin@redhat.com)
- spec: Use relative symlinks in bin/ (walters@verbum.org)
- Added readiness probe for Router (miminar@redhat.com)
- Add openshift/origin-gitserver to push-release.sh (cewong@redhat.com)
- To replace Func postfixed identifier with Fn postfixed in e2e extended
  (salvatore-dario.minonne@amadeus.com)
- to add a job test to extended test (salvatore-dario.minonne@amadeus.com)
- Allow kubelet to be configured for dind compat (marun@redhat.com)
- Force network tests to wait until cluster is ready (marun@redhat.com)
- Update dind docs to configure more vagrant memory (marun@redhat.com)
- Run networking sanity checks separately. (marun@redhat.com)
- Add 'redeploy' command to dind cluster script (marun@redhat.com)
- Fix networking extended test suite declaration (marun@redhat.com)
- Skip internet check in network extended test suite (marun@redhat.com)
- Skip sdn node during dev cluster deploy (marun@redhat.com)
- Fix handling of network plugin arg in deployment (marun@redhat.com)
- Ensure deltarpm is used for devcluster deployment. (marun@redhat.com)
- Rename OPENSHIFT_SDN env var (marun@redhat.com)
- Refactor extended networking test script (marun@redhat.com)
- Increase verbosity of networking test setup (marun@redhat.com)
- Deploy ssh by default on dind cluster nodes (marun@redhat.com)
- Fix numeric comparison bug in cluster provisioning (marun@redhat.com)
- Make provisioning output less noisy (marun@redhat.com)
- Retain systemd logs from extended networking tests (marun@redhat.com)
- Allow networking tests to target existing cluster (marun@redhat.com)
- Optionally skip builds during cluster provision (marun@redhat.com)
- Enable parallel dev cluster deployment (marun@redhat.com)
- Disable scheduling for sdn node when provisioning (marun@redhat.com)
- Rename bash functions used for provisioning (marun@redhat.com)
- Doc and env var cleanup for dind refactor (marun@redhat.com)
- Switch docker-in-docker to use systemd (marun@redhat.com)
- fix merge conflicts in filters/resources.js, update bindata
  (gabriel_ruiz@symantec.com)
- add tests for variable expansion (bparees@redhat.com)
- Use assets/config.local.js if present for development config
  (spadgett@redhat.com)
- Fix typos (dmcphers@redhat.com)
- Allow tag already exist when pushing a release (ccoleman@redhat.com)
- Fixes attach example (ffranz@redhat.com)
- UPSTREAM: 17239: debug filepath in config loader (ffranz@redhat.com)
- Allow setting build config environment variables, show env vars on build
  config page (jforrest@redhat.com)
- UPSTREAM: 17236: fixes attach example (ffranz@redhat.com)
- Remove failing Docker Registry client test (ccoleman@redhat.com)
- leverage new source-to-image API around git clone spec validation/correction
  (gmontero@redhat.com)
- Reload proxy rules on firewalld restart, etc (danw@redhat.com)
- Fix serviceaccount in gitserver example yaml (cewong@redhat.com)
- gitserver: return appropriate error when auth fails (cewong@redhat.com)
- provide validation for build source type (bparees@redhat.com)
- Show less output in test-cmd.sh (ccoleman@redhat.com)
- Change the image workdir to be /var/lib/origin (ccoleman@redhat.com)
- bump(github.com/openshift/source-to-image):
  c9985b5443c4a0a0ffb38b3478031dcc2dc8638d (gmontero@redhat.com)
- Add deployment logs to UI (spadgett@redhat.com)
- Push recycler image (jliggitt@redhat.com)
- Prevent sending username containing colon via basic auth
  (jliggitt@redhat.com)
- added test-cmd test wrapper functions and tests (skuznets@redhat.com)
- Dont loop over all the builds / deployments for the config when we get an
  update for one (jforrest@redhat.com)
- Avoid mobile Safari zoom on input focus (spadgett@redhat.com)
- add build pod name annotation to builds (bparees@redhat.com)
- Prevent autocorrect and autocapitilization for some inputs
  (spadgett@redhat.com)
- bump rails test retry timeout (bparees@redhat.com)
- Adding the recycle tool to the specfile (bleanhar@redhat.com)
- Prevent route from overflowing box in mobile Safari (spadgett@redhat.com)
- Update recycler image to use binary (jliggitt@redhat.com)
- bump(go-ldap/ldap): b4c9518ccf0d85087c925e4a3c9d5802c9bc7025
  (skuznets@redhat.com)
- status: Warn about transient deployment trigger errors (mkargaki@redhat.com)
- Updated pv recycler to work with uid:gid (mturansk@redhat.com)
- Set registry service's session affinity (miminar@redhat.com)
- Several auto-completion fixes (ffranz@redhat.com)
- Fixes auto-completion for build config names (ffranz@redhat.com)
- show warning when pod's containers are restarting (deads@redhat.com)
- Change how we store association between builds and ICTs on DCs for the
  console overview (jforrest@redhat.com)
- Point docs at docs.openshift.com/enterprise/latest (sdodson@redhat.com)
- eliminate double bc reporting in status (deads@redhat.com)
- Use a different donut color for pods not ready (spadgett@redhat.com)
- disable --validate flag by default (deads@redhat.com)
- Update CONTRIBUTING.adoc (bpeterse@redhat.com)
- Show deployment status on overview when failed or cancelled
  (spadgett@redhat.com)
- UPSTREAM: revert: 0048df4: <carry>: Disable --validate by default
  (deads@redhat.com)
- add reconcile-cluster-role arg for specifying specific roles
  (deads@redhat.com)
- Reduce number of tick labels in metrics sparkline (spadgett@redhat.com)
- fix openshift client cache for different versions (deads@redhat.com)
- UPSTREAM: 17058: fix client cache for different versions (deads@redhat.com)
- make export-all work in failures (deads@redhat.com)
- Fix build-waiting logic to use polling instead of watcher
  (nagy.martin@gmail.com)
- add APIGroup to role describer (deads@redhat.com)
- UPSTREAM: 17017: stop jsonpath panicing on bad array length
  (deads@redhat.com)
- don't update the build phase once it reaches a terminal state
  (bparees@redhat.com)
- Run registry as non-root user (ironcladlou@gmail.com)
- warn on missing log and metric URLs for console (deads@redhat.com)
- don't show context nicknames that users don't recognize (deads@redhat.com)
- Allow non-alphabetic characters in expression generator (mfojtik@redhat.com)
- WIP - try out upstream e2e (ccoleman@redhat.com)
- add a wordpress template (bparees@redhat.com)
- UPSTREAM: 16945: kubelet: Fallback to api server for pod status
  (mkargaki@redhat.com)
- Bug 1278007 - Use commit instead of HEAD when streaming in start-build
  (mfojtik@redhat.com)
- Move os::util:install-sdn to contrib/node/install-sdn.sh (sdodson@redhat.com)
- Drop selinux relabeling for volumes, add images to push-release
  (sdodson@redhat.com)
- Build controller - set build status only if pod creation succeeds
  (cewong@redhat.com)
- Let users to select log content without line numbers (spadgett@redhat.com)
- pre-set the SA namespaces (deads@redhat.com)
- Add labels and annotations to DeploymentStrategy. (roque@juniper.net)
- containerized-installs -> containerized (sdodson@redhat.com)
- Add example systemd units for running as container (sdodson@redhat.com)
- Add sdn ovs enabled node image (sdodson@redhat.com)
- fix case for events (deads@redhat.com)
- prune: Remove deployer pods when pruning failed deployments
  (mkargaki@redhat.com)
- reenable static building of hello pod (bparees@redhat.com)
- do not make redistributable in fedora (tdawson@redhat.com)
- Make building clients for other architectures optional (tdawson@redhat.com)

* Tue Nov 10 2015 Scott Dodson <sdodson@redhat.com> 3.1.0.4
- change OS bootstrap SCCs to use RunAsAny for fsgroup and sup groups
  (pweil@redhat.com)
- UPSTREAM:<carry>:v1beta3 default fsgroup/supgroup strategies to RunAsAny
  (pweil@redhat.com)
- UPSTREAM:<carry>:default fsgroup/supgroup strategies to RunAsAny
  (pweil@redhat.com)
- UPSTREAM: 17061: Unnecessary updates to ResourceQuota when doing UPDATE
  (decarr@redhat.com)
- Fix typo (dmcphers@redhat.com)
- Update HPA bootstrap policy (sross@redhat.com)
- Run deployer as non-root user (ironcladlou@gmail.com)
- UPSTREAM: 15537: openstack: cache InstanceID and use it for volume
  management. (jsafrane@redhat.com)

* Mon Nov 09 2015 Troy Dawson <tdawson@redhat.com> 3.1.0.3
- Add jenkins status to readme (dmcphers@redhat.com)
- cAdvisor needs access to dmsetup for devicemapper info (ccoleman@redhat.com)
- Make auth-in-container tests cause test failure again (ccoleman@redhat.com)
- UPSTREAM: 16969: nsenter file writer mangles newlines (ccoleman@redhat.com)
- Doc fixes (mkargaki@redhat.com)
- Doc fixes (dmcphers@redhat.com)
- Specify scheme/port for metrics client (jliggitt@redhat.com)
- UPSTREAM: 16926: Enable specifying scheme/port for metrics client
  (jliggitt@redhat.com)
- Test template preservation of integers (jliggitt@redhat.com)
- UPSTREAM: 16964: Preserve int64 data when unmarshaling (jliggitt@redhat.com)
- Given we don't restrict on Travis success.  Make what it does report be 100%%
  reliable and fast.  So when it does fail we know something is truly wrong.
  (dmcphers@redhat.com)
- back project cache with local authorizer (deads@redhat.com)

* Sat Nov 07 2015 Brenton Leanhardt <bleanhar@redhat.com> 3.1.0.2
- bump(github.com/openshift/openshift-sdn)
  d5965ee039bb85c5ec9ef7f455a8c03ac0ff0214 (dcbw@redhat.com)
- Identify the upstream Kube tag more clearly (ccoleman@redhat.com)
- Move etcd.log out of etcd dir (jliggitt@redhat.com)
- Conditionally run extensions controllers (jliggitt@redhat.com)
- Make namespace delete trigger exp resource delete (sross@redhat.com)

* Fri Nov 06 2015 Troy Dawson <tdawson@redhat.com> 3.1.0.1
- Allow in-cluster config for oc (ccoleman@redhat.com)
- deprecation for buildconfig label (bparees@redhat.com)
- Unable to submit subject rolebindings to a v1 server (ccoleman@redhat.com)
- Allow a service account installation (ccoleman@redhat.com)
- UPSTREAM: 16818: Namespace controller should always get latest state prior to
  deletion (decarr@redhat.com)
- UPSTREAM: 16859: Return a typed error for no-config (ccoleman@redhat.com)
- New-app: Set context directory and strategy on source repos specified with
  tilde(~) (cewong@redhat.com)
- update UPGRADE.md (pweil@redhat.com)

* Wed Nov 04 2015 Scott Dodson <sdodson@redhat.com> 3.1.0.0
- add reconcile-sccs command (pweil@redhat.com)
- Guard against servers that return non-json for the /v2/ check
  (ccoleman@redhat.com)
- Added PVController service account (mturansk@redhat.com)
- Disable deployment config detail message updates (ironcladlou@gmail.com)
- updates via github discussions (admin@benjaminapetersen.me)
- UPSTREAM: 16432: fixed pv binder race condition (mturansk@redhat.com)
- Fixes completions (ffranz@redhat.com)
- UPSTREAM: spf13/cobra: fixes filename completion (ffranz@redhat.com)
- Revert 7fc8ab5b2696b533e6ac5bea003e5a0622bdbf58 (jordan@liggitt.net)
- Automatic commit of package [atomic-openshift] release [3.0.2.906].
  (tdawson@redhat.com)
- UPSTREAM: 16384: Large memory allocation with key prefix generation
  (ccoleman@redhat.com)
- Update bash completions (decarr@redhat.com)
- UPSTREAM: 16749: Kubelet serialize image pulls had incorrect default
  (decarr@redhat.com)
- UPSTREAM: 15914: make kubelet images pulls serialized by default
  (decarr@redhat.com)
- New kibanna archive log link on log tab for build & pod
  (admin@benjaminapetersen.me)
- Transfer ImagePullSecrets to deployment hook pods (ironcladlou@gmail.com)
- Copy volume mounts to hook pods (ironcladlou@gmail.com)
- UPSTREAM: 16717: Ensure HPA has valid resource/name/subresource, validate
  path segments (jliggitt@redhat.com)
- Switch back to subnet (dmcphers@redhat.com)
- Inline deployer hook logs (ironcladlou@gmail.com)
- Remove default subnet (dmcphers@redhat.com)
- Disable quay.io test (ccoleman@redhat.com)
- UPSTREAM: 16032: revert origin 03e50db: check if /sbin/mount.nfs is present
  (mturansk@redhat.com)
- UPSTREAM: 16277: Fixed resetting last scale time in HPA status
  (sross@redhat.com)
- Change subnet default (dmcphers@redhat.com)
- Add default subnet back (dmcphers@redhat.com)
- Remove default subnet (dmcphers@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  cb0e352cd7591ace30d592d4f82685d2bcd38a04 (rpenta@redhat.com)
- Disable new-app Git tests in Vagrant (ccoleman@redhat.com)
- oc logs long description is wrong (ffranz@redhat.com)
- scc sort by priority (pweil@redhat.com)
- UPSTREAM:<carry>:v1beta3 scc priority field (pweil@redhat.com)
- UPSTREAM:<carry>:scc priority field (pweil@redhat.com)
- Bug and issue fixes (3) (sgoodwin@redhat.com)
- Fix deploy test conflict flake (ironcladlou@gmail.com)
- Prevent early exit on install-assets failure (jliggitt@redhat.com)
- UPSTREAM: 15997: Prevent NPE in resource printer on HPA (ccoleman@redhat.com)
- UPSTREAM: 16478: Daemon controller shouldn't place pods on not ready nodes
  (ccoleman@redhat.com)
- UPSTREAM: 16340: Kubelet pod status update is not correctly occuring
  (ccoleman@redhat.com)
- UPSTREAM: 16191: Mirror pods don't show logs (ccoleman@redhat.com)
- UPSTREAM: 14182: Distinguish image registry unavailable and pull failure
  (decarr@redhat.com)
- UPSTREAM: 16174: NPE when checking for mounting /etc/hosts
  (ccoleman@redhat.com)
-  Bug 1275537 - Fixed the way image import controller informs about errors
  from imports. (maszulik@redhat.com)
- UPSTREAM: 16052: Control /etc/hosts in the kubelet (ccoleman@redhat.com)
- UPSTREAM: 16044: Don't shadow error in cache.Store (ccoleman@redhat.com)
- UPSTREAM: 16025: Fix NPE in describe of HPA (ccoleman@redhat.com)
- UPSTREAM: 16668: Fix hpa escalation (deads@redhat.com)
- UPSTREAM: 15944: DaemonSet controller modifies the wrong fields
  (ccoleman@redhat.com)
- UPSTREAM: 15414: Annotations for kube-proxy move to beta
  (ccoleman@redhat.com)
- UPSTREAM: 15745: Endpoint timeouts in the proxy are bad (ccoleman@redhat.com)
- UPSTREAM: 15646: DaemonSet validation (ccoleman@redhat.com)
- UPSTREAM: 15574: Validation on resource quota (ccoleman@redhat.com)
- Calculate correct bottom scroll position (spadgett@redhat.com)
- oc tag should retry on conflict errors (ccoleman@redhat.com)
- Remove log arg for travis (jliggitt@redhat.com)
- hack/install-assets: fix nonstandard bash (lmeyer@redhat.com)
- extend role covers with groups (deads@redhat.com)
- Bug 1277021 - Fixed import-image help information. (maszulik@redhat.com)
- Deprecate build-logs in favor of logs (mkargaki@redhat.com)
- Build and deployment logs should check kubelet response (ccoleman@redhat.com)
- DNS services are not resolving properly (ccoleman@redhat.com)
- hack/test-end-to-end.sh won't start on IPv6 system (ccoleman@redhat.com)
- Wait longer for etcd startup in integration tests (ccoleman@redhat.com)
- Restore detailed checking for forbidden exec (jliggitt@redhat.com)
- UPSTREAM: 16711: Read error from failed upgrade attempts
  (jliggitt@redhat.com)
- Only show scroll links when log is offscreen (spadgett@redhat.com)
- Temporarily accept 'Forbidden' and 'forbidden' responses
  (jliggitt@redhat.com)
- Add HPA support for DeploymentConfig (sross@redhat.com)
- UPSTREAM: 16570: Fix GetRequestInfo subresource parsing for proxy/redirect
  verbs (sross@redhat.com)
- UPSTREAM: 16671: Customize HPA Heapster service namespace/name
  (sross@redhat.com)
- Add Scale Subresource to DeploymentConfigs (sross@redhat.com)
- bz 1276319 - Fix oc rsync deletion with tar strategy (cewong@redhat.com)
- UPSTREAM: <carry>: s/imagestraams/imagestreams/ in `oc get`
  (eparis@redhat.com)
- UPSTREAM(go-dockerclient): 408: fix stdin-only attach (agoldste@redhat.com)
- Add error clause to service/project.js (admin@benjaminapetersen.me)
- Switch to checking for CrashLoopBackOff to show the container looping message
  (jforrest@redhat.com)
- Fix namespace initialization (jliggitt@redhat.com)
- UPSTREAM: 16590: Create all streams before copying in exec/attach
  (agoldste@redhat.com)
- UPSTREAM: 16677: Add Validator for Scale Objects (sross@redhat.com)
- UPSTREAM: 16537: attach must only allow a tty when container supports it
  (ffranz@redhat.com)
- Bug 1276602 - fixes error when scaling dc with --timeout (ffranz@redhat.com)
- test/cmd/admin.sh isn't reentrant (ccoleman@redhat.com)
- add group/version serialization to master (deads@redhat.com)
- UPSTREAM: <drop>: allow specific, skewed group/versions (deads@redhat.com)
- UPSTREAM: 16667: Make Kubernetes HPA Controller use Namespacers
  (sross@redhat.com)
- rsync: output warnings to stdout instead of using glog (cewong@redhat.com)
- allow cluster-admin and cluster-reader to use different groups
  (deads@redhat.com)
- UPSTREAM: 16127: Bump cAdvisor (jimmidyson@gmail.com)
- UPSTREAM: 15612: Bump cadvisor (jimmidyson@gmail.com)
- rsync: output warnings to stdout instead of using glog (cewong@redhat.com)
- UPSTREAM: 16223: Concurrency fixes in kubelet status manager
  (ccoleman@redhat.com)
- UPSTREAM: 15275: Kubelet reacts much faster to unhealthy containers
  (ccoleman@redhat.com)
- UPSTREAM: 15706: HorizontalPodAutoscaler and Scale subresource APIs graduated
  to beta (sross@redhat.com)
- Fixed how tags are being printed when describing ImageStream. Previously the
  image.Spec.Tags was ignored, which resulted in not showing the tags for which
  there were errors during imports. (maszulik@redhat.com)

* Wed Nov 04 2015 Troy Dawson <tdawson@redhat.com> 3.0.2.906
- Copy volume mounts to hook pods (ironcladlou@gmail.com)
- UPSTREAM: 16717: Ensure HPA has valid resource/name/subresource, validate
  path segments (jliggitt@redhat.com)
- Switch back to subnet (dmcphers@redhat.com)
- Inline deployer hook logs (ironcladlou@gmail.com)
- Remove default subnet (dmcphers@redhat.com)
- Disable quay.io test (ccoleman@redhat.com)
- UPSTREAM: 16032: revert origin 03e50db: check if /sbin/mount.nfs is present
  (mturansk@redhat.com)
- UPSTREAM: 16277: Fixed resetting last scale time in HPA status
  (sross@redhat.com)
- Change subnet default (dmcphers@redhat.com)
- Add default subnet back (dmcphers@redhat.com)
- Remove default subnet (dmcphers@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  cb0e352cd7591ace30d592d4f82685d2bcd38a04 (rpenta@redhat.com)
- Disable new-app Git tests in Vagrant (ccoleman@redhat.com)
- oc logs long description is wrong (ffranz@redhat.com)
- scc sort by priority (pweil@redhat.com)
- UPSTREAM:<carry>:v1beta3 scc priority field (pweil@redhat.com)
- UPSTREAM:<carry>:scc priority field (pweil@redhat.com)
- Bug and issue fixes (3) (sgoodwin@redhat.com)
- Fix deploy test conflict flake (ironcladlou@gmail.com)
- Prevent early exit on install-assets failure (jliggitt@redhat.com)
- UPSTREAM: 15997: Prevent NPE in resource printer on HPA (ccoleman@redhat.com)
- UPSTREAM: 16478: Daemon controller shouldn't place pods on not ready nodes
  (ccoleman@redhat.com)
- UPSTREAM: 16340: Kubelet pod status update is not correctly occuring
  (ccoleman@redhat.com)
- UPSTREAM: 16191: Mirror pods don't show logs (ccoleman@redhat.com)
- UPSTREAM: 14182: Distinguish image registry unavailable and pull failure
  (decarr@redhat.com)
- UPSTREAM: 16174: NPE when checking for mounting /etc/hosts
  (ccoleman@redhat.com)
-  Bug 1275537 - Fixed the way image import controller informs about errors
  from imports. (maszulik@redhat.com)
- UPSTREAM: 16052: Control /etc/hosts in the kubelet (ccoleman@redhat.com)
- UPSTREAM: 16044: Don't shadow error in cache.Store (ccoleman@redhat.com)
- UPSTREAM: 16025: Fix NPE in describe of HPA (ccoleman@redhat.com)
- UPSTREAM: 16668: Fix hpa escalation (deads@redhat.com)
- UPSTREAM: 15944: DaemonSet controller modifies the wrong fields
  (ccoleman@redhat.com)
- UPSTREAM: 15414: Annotations for kube-proxy move to beta
  (ccoleman@redhat.com)
- UPSTREAM: 15745: Endpoint timeouts in the proxy are bad (ccoleman@redhat.com)
- UPSTREAM: 15646: DaemonSet validation (ccoleman@redhat.com)
- UPSTREAM: 15574: Validation on resource quota (ccoleman@redhat.com)
- Calculate correct bottom scroll position (spadgett@redhat.com)
- oc tag should retry on conflict errors (ccoleman@redhat.com)
- Remove log arg for travis (jliggitt@redhat.com)
- hack/install-assets: fix nonstandard bash (lmeyer@redhat.com)
- extend role covers with groups (deads@redhat.com)
- Bug 1277021 - Fixed import-image help information. (maszulik@redhat.com)
- Deprecate build-logs in favor of logs (mkargaki@redhat.com)
- Build and deployment logs should check kubelet response (ccoleman@redhat.com)
- DNS services are not resolving properly (ccoleman@redhat.com)
- hack/test-end-to-end.sh won't start on IPv6 system (ccoleman@redhat.com)
- Wait longer for etcd startup in integration tests (ccoleman@redhat.com)
- Restore detailed checking for forbidden exec (jliggitt@redhat.com)
- UPSTREAM: 16711: Read error from failed upgrade attempts
  (jliggitt@redhat.com)
- Only show scroll links when log is offscreen (spadgett@redhat.com)
- Temporarily accept 'Forbidden' and 'forbidden' responses
  (jliggitt@redhat.com)
- Add HPA support for DeploymentConfig (sross@redhat.com)
- UPSTREAM: 16570: Fix GetRequestInfo subresource parsing for proxy/redirect
  verbs (sross@redhat.com)
- UPSTREAM: 16671: Customize HPA Heapster service namespace/name
  (sross@redhat.com)
- Add Scale Subresource to DeploymentConfigs (sross@redhat.com)
- bz 1276319 - Fix oc rsync deletion with tar strategy (cewong@redhat.com)
- UPSTREAM: <carry>: s/imagestraams/imagestreams/ in `oc get`
  (eparis@redhat.com)
- UPSTREAM(go-dockerclient): 408: fix stdin-only attach (agoldste@redhat.com)
- Add error clause to service/project.js (admin@benjaminapetersen.me)
- Switch to checking for CrashLoopBackOff to show the container looping message
  (jforrest@redhat.com)
- Fix namespace initialization (jliggitt@redhat.com)
- UPSTREAM: 16590: Create all streams before copying in exec/attach
  (agoldste@redhat.com)
- UPSTREAM: 16677: Add Validator for Scale Objects (sross@redhat.com)
- UPSTREAM: 16537: attach must only allow a tty when container supports it
  (ffranz@redhat.com)
- Bug 1276602 - fixes error when scaling dc with --timeout (ffranz@redhat.com)
- test/cmd/admin.sh isn't reentrant (ccoleman@redhat.com)
- add group/version serialization to master (deads@redhat.com)
- UPSTREAM: <drop>: allow specific, skewed group/versions (deads@redhat.com)
- UPSTREAM: 16667: Make Kubernetes HPA Controller use Namespacers
  (sross@redhat.com)
- rsync: output warnings to stdout instead of using glog (cewong@redhat.com)
- Throttle log updates to avoid UI flicker (spadgett@redhat.com)
- allow cluster-admin and cluster-reader to use different groups
  (deads@redhat.com)
- UPSTREAM: 16127: Bump cAdvisor (jimmidyson@gmail.com)
- UPSTREAM: 15612: Bump cadvisor (jimmidyson@gmail.com)
- Bug 1277017 - Added checking if spec.DockerImageRepository and spec.Tags are
  not empty. (maszulik@redhat.com)
- rsync: output warnings to stdout instead of using glog (cewong@redhat.com)
- Bug 1276657 - Reuse --insecure-registry flag value when creating ImageStream
  from passed docker image. (maszulik@redhat.com)
- Use pficon-info on pod terminal tab (spadgett@redhat.com)
- Bug 1268000 - replace Image.DockerImageReference with value from status.
  (maszulik@redhat.com)
- UPSTREAM: 16223: Concurrency fixes in kubelet status manager
  (ccoleman@redhat.com)
- UPSTREAM: 15275: Kubelet reacts much faster to unhealthy containers
  (ccoleman@redhat.com)
- UPSTREAM: 16033: Mount returns verbose error (ccoleman@redhat.com)
- UPSTREAM: 16032: check if /sbin/mount.nfs is present (ccoleman@redhat.com)
- UPSTREAM: 15555: Use default port 3260 for iSCSI (ccoleman@redhat.com)
- UPSTREAM: 15562: iSCSI use global path to mount (ccoleman@redhat.com)
- UPSTREAM: 15236: Better error output from gluster (ccoleman@redhat.com)
- UPSTREAM: 15706: HorizontalPodAutoscaler and Scale subresource APIs graduated
  to beta (sross@redhat.com)
- Fixed how tags are being printed when describing ImageStream. Previously the
  image.Spec.Tags was ignored, which resulted in not showing the tags for which
  there were errors during imports. (maszulik@redhat.com)
- UPSTREAM: 15961: Add streaming subprotocol negotation (agoldste@redhat.com)
- Systemd throws an error on Restart=Always (sdodson@redhat.com)

* Sun Nov 01 2015 Scott Dodson <sdodson@redhat.com> 3.0.2.905
- Add and test field label conversions (jliggitt@redhat.com)
- logs: View logs from older deployments/builds with --version
  (mkargaki@redhat.com)
- UPSTREAM: 15733: Disable keepalive on liveness probes (ccoleman@redhat.com)
- UPSTREAM: 15845: Add service locator in service rest storage
  (ccoleman@redhat.com)
- install-assets: retry bower update (lmeyer@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  1f449c7f0d3cd41314a895ef119f9d25a15b54de (rpenta@redhat.com)
- Improve UI performance when displaying large logs (spadgett@redhat.com)
- fixes as per @smarterclayton review comments. (smitram@gmail.com)
- new SCCs (pweil@redhat.com)
- UPSTREAM: 16068: Increase annotation size significantly (ccoleman@redhat.com)
- UPSTREAM(go-dockerclient): 408: fix stdin-only attach (agoldste@redhat.com)
- stop creating roles with resourcegroups (deads@redhat.com)
- Fixes as per @smarterclayton and @liggit review comments. (smitram@gmail.com)
- Various style/positioning fixes (sgoodwin@redhat.com)
- Add missing kube resources to bootstrap policy (jliggitt@redhat.com)
- UPSTREAM: <carry>: OpenShift 3.0.2 nodes report v1.1.0-alpha
  (ccoleman@redhat.com)
- UPSTREAM: 16137: Release node port correctly (ccoleman@redhat.com)
- Run serialization tests for upstream types (mkargaki@redhat.com)
- UPSTREAM: <carry>: Update v1beta3 (mkargaki@redhat.com)
- UPSTREAM: 15930: Deletion of pods managed by old kubelets
  (ccoleman@redhat.com)
- UPSTREAM: 15900: Delete succeeded and failed pods immediately
  (ccoleman@redhat.com)
- add istag list, update (deads@redhat.com)
- diagnostics: default server conf paths changed (lmeyer@redhat.com)
- diagnostics: systemd unit name changes (lmeyer@redhat.com)
- Handle passwords with colon in basic auth (pep@redhat.com)
- Bug 1275564 - Removed the requirement for spec.dockerImageRepository from
  import-image command. (maszulik@redhat.com)
- bump(github.com/openshift/source-to-image)
  65d46436ab599633b76e570311a05f46a818389b (mfojtik@redhat.com)
- Add openshift.io/build-config.name label to builds. (vsemushi@redhat.com)
- oc: Use default resources where it makes sense (mkargaki@redhat.com)
- logs: Support all flags for builds and deployments (mkargaki@redhat.com)
- UPSTREAM: <carry>: Update v1beta3 PodLogOptions (mkargaki@redhat.com)
- UPSTREAM: 16494: Remove dead pods upon stopping a job (maszulik@redhat.com)
- Add local IP addresses to node certificate (jliggitt@redhat.com)
- Rest validation of binary builds is more aggressive (ccoleman@redhat.com)
-   o Add support to expose/redirect/disable insecure schemes (http) for
  edge secured routes.   o Add changes to template, haproxy and f5 router
  implementations.   o Add generated* files. (smitram@gmail.com)
- removed unneeded squash and chown from nfs doc (mturansk@redhat.com)
- Only run pod nodeenv admission on create (agoldste@redhat.com)
- fix up latest tags and add new scl image versions (bparees@redhat.com)
- UPSTREAM: 16532: Allow log tail and log follow to be specified together
  (ccoleman@redhat.com)
- Need to be doing a bower update instead of install so dependencies will
  update without conflict (jforrest@redhat.com)
- Update swagger spec (pmorie@gmail.com)
- UPSTREAM: 15799: Fix PodPhase issue caused by backoff (mkargaki@redhat.com)
- watchObject in console triggers callbacks for events of items of the same
  kind (jforrest@redhat.com)
- status: Report routes that have no route port specified (mkargaki@redhat.com)
- Add special casing for v1beta3 DeploymentConfig in serialization_test
  (pmorie@gmail.com)
- UPSTREAM: <carry>: respect fuzzing defaults for v1beta3 SecurityContext
  (pmorie@gmail.com)
- OS integration for PSC (pweil@redhat.com)
- UPSTREAM: <carry>: v1beta3 scc integration for PSC (pweil@redhat.com)
- UPSTREAM: <carry>: scc integration for PSC (pweil@redhat.com)
- UPSTREAM: <carry>: Workaround for cadvisor/libcontainer config schema
  mismatch (pmorie@gmail.com)
- UPSTREAM: 15323: Support volume relabling for pods which specify an SELinux
  label (pmorie@gmail.com)
- Update completions (jimmidyson@gmail.com)
- Test scm password auth (jliggitt@redhat.com)
- Add prometheus exporter to haproxy router (jimmidyson@gmail.com)
- UPSTREAM: 16332: Remove invalid blank line when printing jobs
  (maszulik@redhat.com)
- UPSTREAM: 16234: Fix jobs unittest flakes (maszulik@redhat.com)
- UPSTREAM: 16196: Fix e2e test flakes (maszulik@redhat.com)
- UPSTREAM: 15791: Update master service ports and type via controller.
  (abutcher@redhat.com)
- Add dns ports to the master service (abutcher@redhat.com)
- UPSTREAM: 15352: FSGroup implementation (pmorie@gmail.com)
- UPSTREAM: 14705: Inline some SecurityContext fields into PodSecurityContext
  (pmorie@gmail.com)
- UPSTREAM: 14991: Add Support for supplemental groups (pmorie@gmail.com)
- Allow processing template from different namespace (mfojtik@redhat.com)
- changed build for sti and docker to use OutputDockerImageReference, fixed
  tests (ipalade@redhat.com)
- Bug 1270728 - username in the secret don't override the username in the
  source URL (jhadvig@redhat.com)
- UPSTREAM: 15520: Move job to generalized label selector (maszulik@redhat.com)
- Update openshift-object-describer to 1.1.1 (jforrest@redhat.com)

* Thu Oct 29 2015 Scott Dodson <sdodson@redhat.com> 3.0.2.904
- Based on origin v1.0.7
- Provide informational output in new-app and new-build (ccoleman@redhat.com)
- Revert 'Retry ISM save upon conflicting IS error' commit upon retry mechanism
  introduced in ISM create method (maszulik@redhat.com)
- Bug 1275003: expose: Set route port based on service target port
  (mkargaki@redhat.com)
- Improve generation of name based on a git repo url to make it valid.
  (vsemushi@redhat.com)
- UPSTREAM: 16080: Convert from old mirror pods (1.0 to 1.1)
  (ccoleman@redhat.com)
- UPSTREAM: 15983: Store mirror pod hash in annotation (ccoleman@redhat.com)
- Slightly better output order in the CLI for login (ccoleman@redhat.com)
- Attempt to find merged parent (ccoleman@redhat.com)
- Fix extended tests for --from-* binary (ccoleman@redhat.com)
- Completions (ffranz@redhat.com)
- UPSTREAM: 16482: stdin is not a file extension for bash completions
  (ffranz@redhat.com)
- Bump angular-pattern to version 2.3.4 (spadgett@redhat.com)
- Use cmdutil.PrintSuccess to print objects (ffranz@redhat.com)
- Improve the error messages when something isn't found (ccoleman@redhat.com)
- UPSTREAM: 16445: Capitalize and expand UsageError message
  (ccoleman@redhat.com)
- Updates to address several issues (sgoodwin@redhat.com)
- Refactor to use ExponentialBackoff (ironcladlou@gmail.com)
- bump(github.com/openshift/openshift-sdn)
  c08ebda0774795eec624b5ce9063662b19959cf3 (rpenta@redhat.com)
- Auto-generated docs/bash-completions for 'oadm pod-network make-projects-
  global' (rpenta@redhat.com)
- Show empty deployments on overview if latest (spadgett@redhat.com)
- Support retrying mapping creation on conflict (ironcladlou@gmail.com)
- Support installation via containers in new-app (ccoleman@redhat.com)
- Disable create form inputs and submit button while the API request is
  happening (jforrest@redhat.com)
- fix project request with quota (deads@redhat.com)
- UPSTREAM: 16441: Pass runtime.Object to Helper.Create/Replace
  (deads@redhat.com)
- Slim down the extended tests (ccoleman@redhat.com)
- upper case first letter of status message (bparees@redhat.com)
- Rsync fixes (cewong@redhat.com)
- better error message for missing dockerfile (bparees@redhat.com)
- add verbose logging to new-app flake and remove extraneous tryuntil logging
  (bparees@redhat.com)
- Support --binary on new-build (ccoleman@redhat.com)
- Fix text-overflow issue on Safari (sgoodwin@redhat.com)
- bump(github.com/openshift/source-to-image)
  9728b53c11218598acb2cc1b9c8cc762c36f44bc (cewong@redhat.com)
- Include name of port in pod template, switch to port/protocol format
  (jforrest@redhat.com)
- Various logs fixes: (admin@benjaminapetersen.me)
- add --context-dir flag to new-build (bparees@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  22b9a4176435ac4453c30c53799338979ef79050 (rpenta@redhat.com)
- Adding border above builds without a service so that they look more
  connected. (sgoodwin@redhat.com)
- Updates to the log-view so that it's more inline with pod terminal. And
  switch to more subtle ellipsis loader. (sgoodwin@redhat.com)
- Expose all container ports creating from source in the UI
  (spadgett@redhat.com)
- Fail if timeout is reached (ccoleman@redhat.com)
- UPSTREAM: 15975: Validate names in BeforeCreate (jliggitt@redhat.com)
- Bug 1275234 - fixes --resource-version error in scale (ffranz@redhat.com)
- skip project validation when creating a new-project (deads@redhat.com)
- Show route target port in the routes table if its set (jforrest@redhat.com)
- Allow users to click monopod donut charts on overview (spadgett@redhat.com)
- Add verbose output to test execution (marun@redhat.com)
- Show warning popup when builds have a status message (jforrest@redhat.com)
- UPSTREAM: 16241: Deflake wsstream stream_test.go (ccoleman@redhat.com)
- Read kubernetes remote from git repository. (maszulik@redhat.com)
- Fix dind vagrant provisioning and test execution (marun@redhat.com)
- Bug 1261548 - oc run --attach support for DeploymentConfig
  (ffranz@redhat.com)
- Use server time for end time in metrics requests (spadgett@redhat.com)
- Fix typo (jliggitt@redhat.com)
- Update angular-patternfly to version 2.3.3 (spadgett@redhat.com)
- UPSTREAM: 16109: expose attachable pod discovery in factory
  (ffranz@redhat.com)
- libvirt use nfs for dev cluster synced folder (marun@redhat.com)
- report a warning and do not continue on a partial match (bparees@redhat.com)
- Move the cgroup regex to package level. (roque@juniper.net)
- UPSTREAM: 16286: Avoid CPU hotloop on client-closed websocket
  (jliggitt@redhat.com)
- Increase vagrant memory default by 512mb (marun@redhat.com)
- Source dind script from the docker repo (marun@redhat.com)
- dind: Fix disabling of sdn node scheduling (marun@redhat.com)
- Switch dind image to use fedora 21 (marun@redhat.com)
- Only run provision-full.sh for single-vm clusters (marun@redhat.com)
- Fix extended network test invocation (marun@redhat.com)
- Invoke docker with sudo when archiving test logs (marun@redhat.com)
- Enhance dind vagrant deployment (marun@redhat.com)
- Add dind detail to the contribution doc (marun@redhat.com)
- Enhance dind-cluster.sh documentation (marun@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  1e4edc9abb6bb8ac7e5cd946ddec4c10cc714d67 (danw@redhat.com)
- Add rsync daemon copy strategy for windows support (cewong@redhat.com)
- UPSTREAM: 11694: http proxy support for exec/pf (agoldste@redhat.com)
- hack/cherry-pick.sh: fix typo (agoldste@redhat.com)
- hack/cherry-pick.sh: support binary files in diffs (agoldste@redhat.com)
- Added job policy (maszulik@redhat.com)
- platformmanagement_public_514 - Allow importing tags from ImageStreams
  pointing to external registries. (maszulik@redhat.com)
- Minor fix for nfs readme (liangxia@users.noreply.github.com)
- delete: Remove both image stream spec and status tags (mkargaki@redhat.com)
- move newapp commands that need docker into extended tests
  (bparees@redhat.com)
- Only watch pod statuses for overview donut chart (spadgett@redhat.com)
- Vagrantfile should allow multiple sync folders (ccoleman@redhat.com)
- Fix context dir for STI (ccoleman@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- Deflake test/cmd/newapp.sh (ccoleman@redhat.com)
- Review comments (ccoleman@redhat.com)
- Docs (ccoleman@redhat.com)
- Completions (ccoleman@redhat.com)
- Generated conversions (ccoleman@redhat.com)
- Docker and STI builder images support binary extraction (ccoleman@redhat.com)
- Enable binary build endpoint and CLI via start-build (ccoleman@redhat.com)
- UPSTREAM: 15053<carry>: Conversions for v1beta3 (ccoleman@redhat.com)
- UPSTREAM: 15053: Support stdinOnce and fix attach (ccoleman@redhat.com)
- Disable all e2e portforward tests (ccoleman@redhat.com)
- Fixes infinite loop on login and forces auth when password were provided
  (ffranz@redhat.com)
- increase timeout for helper pods (bparees@redhat.com)
- bump(github.com/openshift/source-to-image)
  84e4633329181926ec8d746e189769522b1ff6a7 (roque@juniper.net)
- When running as a pod, pass the network context to source-to-image.
  (roque@juniper.net)
- allow dockersearcher to be setup after config is parsed (bparees@redhat.com)
- UPSTREAM: <carry>: Back n forth downward/metadata conversions
  (maszulik@redhat.com)
- Ensure no overlap between SDN cluster network and service/portal network
  (rpenta@redhat.com)
- [RPMS] expand obsoletes to include OSE versions (sdodson@redhat.com)
- [RPMS] bump docker requirement to 1.8.2 (sdodson@redhat.com)
- UPSTREAM: 15194: Avoid spurious "Hairpin setup failed..." errors
  (danw@gnome.org)
- Change to healthz as per @liggit comments. (smitram@gmail.com)
- Add a monitoring uri to the stats port. This allows us to not affect hosted
  backends but rather use the listener on the stats port to service health
  check requests. Of course the side effect here is that if you turn off stats,
  then the monitoring uri will not be available. (smitram@gmail.com)

* Fri Oct 23 2015 Scott Dodson <sdodson@redhat.com> 3.0.2.903
- Revert "bump(github.com/openshift/openshift-sdn):
  699716b85d1ac5b2f3e48969bbdbbb2a1266e9d0" (sdodson@redhat.com)
- More cleanup of the individual pages, use the extra space better
  (jforrest@redhat.com)
- don't attempt to call create on files that don't exist (bparees@redhat.com)
- Use remove() rather than html('') to empty SVG element (spadgett@redhat.com)
- enable storage interface functions (deads@redhat.com)
- Retry ISM save upon conflicting IS error (maszulik@redhat.com)
- assets: Update topology-graph widget (stefw@redhat.com)
- logs: Re-use from upstream (mkargaki@redhat.com)
- assets: Use $evalAsync when updating topology widget (stefw@redhat.com)
- Handle data gaps when calculating CPU usage in UI (spadgett@redhat.com)
- add SA and role for job and hpa controllers (deads@redhat.com)
- UPSTREAM: 16067: Provide a RetryOnConflict helper for client libraries
  (maszulik@redhat.com)
- UPSTREAM: 10707: logs: Use resource builder (mkargaki@redhat.com)
- Update pod status chart styles (spadgett@redhat.com)
- rework how allow-missing-images resolves (bparees@redhat.com)
- bump(github.com/docker/spdystream): 43bffc4 (agoldste@redhat.com)
- Remove unused godep (agoldste@redhat.com)
- Increase height of metrics utilization sparkline (spadgett@redhat.com)
- Disable a known broken test (ccoleman@redhat.com)
- UID repair should not fail when out of range allocation (ccoleman@redhat.com)
- Add v1beta3 removal notes to UPGRADE.md (ironcladlou@gmail.com)
- Add iptables to origin image (sdodson@redhat.com)
- policy changes for extensions (deads@redhat.com)
- enable extensions (deads@redhat.com)
- UPSTREAM: 7f6f85bd7b47db239868bcd868ae3472373a4f05: fixes attach broken
  during the refactor (ffranz@redhat.com)
- UPSTREAM: 16042: fix missing error handling (deads@redhat.com)
- UPSTREAM: 16084: Use NewFramework in all tests (ccoleman@redhat.com)
- e2e: Verify service account token and log cli errors (ccoleman@redhat.com)
- add commands for managing SCCs (deads@redhat.com)
- Allow patching events (jliggitt@redhat.com)
- Fix problems with overview donut chart on IE (spadgett@redhat.com)
- Bug 1274200: Prompt by default when deleting a non-existent tag
  (mkargaki@redhat.com)
- correct htpasswd error handling: (deads@redhat.com)
- Set build.status.reason on error (rhcarvalho@gmail.com)
- Fixes several issues with tabbed output (ffranz@redhat.com)
- Referenced tags should get updated on a direct tag (ccoleman@redhat.com)
- oc tag should take an explicit reference to ImageStreamImage
  (ccoleman@redhat.com)
- Add back a fix for the libvirt dev_cluster case (danw@redhat.com)
- Completions (ccoleman@redhat.com)
- Fixes 'oc edit' file blocking on windows (ffranz@redhat.com)
- On Windows, use CRLF line endings in oc edit (ccoleman@redhat.com)
- Add iptables requirement to openshift package (sdodson@redhat.com)
- Remove extra copy of main.css from bindata (jforrest@redhat.com)
- add grace period for evacuate (pweil@redhat.com)
- Remove "Pod" prefix from header on browse pod page (spadgett@redhat.com)
- Only try to update build if status message changed (rhcarvalho@gmail.com)
- Show status details for a DC in both the deployments table and DC pages
  (jforrest@redhat.com)
- Bug 1273787 - start deployment btn doesnt get enabled after dep finishes
  (jforrest@redhat.com)
- UPSTREAM: Proxy: do not send X-Forwarded-Host or X-Forwarded-Proto with an
  empty value (cewong@redhat.com)
- handle new non-resource urls (deads@redhat.com)
- UPSTREAM: 15958: add nonResourceURL detection (deads@redhat.com)
- expose: Set route port (mkargaki@redhat.com)
- UPSTREAM: 15461: expose: Enable exposing multiport objects
  (mkargaki@redhat.com)

* Wed Oct 21 2015 Scott Dodson <sdodson@redhat.com> 3.0.2.902
- Revert "Support deleting image stream status tags" (ccoleman@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  699716b85d1ac5b2f3e48969bbdbbb2a1266e9d0 (danw@redhat.com)
- Adjust line-height of route link to avoid clipping (spadgett@redhat.com)
- Support deleting image stream status tags (kargakis@tuta.io)
- tag: Support deleting a tag with -d (mkargaki@redhat.com)
- Bump K8s label selector to next version (sgoodwin@redhat.com)
- Add a container terminal to the pod details page (stefw@redhat.com)
- Remove old labels rendering from individual pages since its duplicate info
  (jforrest@redhat.com)
- Remove duplicate bootstrap.js from dependencies (spadgett@redhat.com)
- updated LDIF and tests (skuznets@redhat.com)
- Bug 1273350 - make click open the secondary nav instead of navigating
  (jforrest@redhat.com)
- Update label key/value pairs truncate at <769. Fixes
  https://github.com/openshift/origin/issues/5181 (sgoodwin@redhat.com)
- remove the deprecated build label on pods (bparees@redhat.com)
- UPSTREAM: 15621: Correctly handle empty source (ccoleman@redhat.com)
- UPSTREAM: 15953: Return unmodified error from negotiate (ccoleman@redhat.com)
- Fix exposing deployment configs (mkargaki@redhat.com)
- Remove code duplication (rhcarvalho@gmail.com)
- Fix patternfly CSS ordering (spadgett@redhat.com)
- Initial impl of viewing logs in web console (admin@benjaminapetersen.me)
- This commit implements birthcry for openshift proxy. This also addresses
  rhbz: https://bugzilla.redhat.com/show_bug.cgi?id=1270474
  (avagarwa@redhat.com)
- Bug 1271989 - error when navigating to resources in diff projects
  (jforrest@redhat.com)
- fix --config flag (deads@redhat.com)
- client side changes for deployment logs (mkargaki@redhat.com)
- server side changes for deployment logs (mkargaki@redhat.com)
- api changes for deployment logs (mkargaki@redhat.com)
- Disable new upstream e2e tests (ccoleman@redhat.com)
- Remove authentication from import (ccoleman@redhat.com)
- Web console scaling (spadgett@redhat.com)
- Bug 1268891 - pods not always grouped when service selector should cover
  template of a dc/deployment (jforrest@redhat.com)
- ImageStream status.dockerImageRepository should always be local
  (ccoleman@redhat.com)
- remove kubectl apply from oc (deads@redhat.com)
- UPSTREAM: <drop>: disable kubectl apply until there's an impl
  (deads@redhat.com)
- Add ng-cloak to navbar to reduce flicker on load (spadgett@redhat.com)
- Disable v1beta3 in REST API (ironcladlou@gmail.com)
- help unit tests compile (maszulik@redhat.com)
- refactors (deads@redhat.com)
- UPSTREAM: openshift-sdn(TODO): update for iptables.New call
  (deads@redhat.com)
- UPSTREAM: openshift-sdn(TODO): handle boring upstream refactors
  (deads@redhat.com)
- UPSTREAM: 12221: Allow custom namespace creation in e2e framework
  (deads@redhat.com)
- UPSTREAM: 15807: Platform-specific setRLimit implementations
  (jliggitt@redhat.com)
- UPSTREAM: TODO: expose ResyncPeriod function (deads@redhat.com)
- UPSTREAM: 15451 <partial>: Add our types to kubectl get error
  (ccoleman@redhat.com)
- UPSTREAM: 14496: deep-copies: Structs cannot be nil (mkargaki@redhat.com)
- UPSTREAM: 11827: allow permissive SA secret ref limitting (deads@redhat.com)
- UPSTREAM: 12498: Re-add timeouts for kubelet which is not in the upstream PR.
  (deads@redhat.com)
- UPSTREAM: 15232: refactor logs to be composeable (deads@redhat.com)
- UPSTREAM: 8890: Allowing ActiveDeadlineSeconds to be updated for a pod
  (deads@redhat.com)
- UPSTREAM: <drop>: tweak generator to handle conversions in other packages
  (deads@redhat.com)
- UPSTREAM: <drop>: make test pass with old codec (deads@redhat.com)
- UPSTREAM: <drop>: add back flag types to reduce noise during this rebase
  (deads@redhat.com)
- UPSTREAM: <none>: Hack date-time format on *util.Time (ccoleman@redhat.com)
- UPSTREAM: <none>: Suppress aggressive output of warning (ccoleman@redhat.com)
- UPSTREAM: <carry>: v1beta3 (deads@redhat.com)
- UPSTREAM: <carry>: support pointing oc exec to old openshift server
  (deads@redhat.com)
- UPSTREAM: <carry>: Back n forth downward/metadata conversions
  (deads@redhat.com)
- UPSTREAM: <carry>: Disable --validate by default (mkargaki@redhat.com)
- UPSTREAM: <carry>: update describer for dockercfg secrets (deads@redhat.com)
- UPSTREAM: <carry>: reallow the ability to post across namespaces in api
  (pweil@redhat.com)
- UPSTREAM: <carry>: helper methods paralleling old latest fields
  (deads@redhat.com)
- UPSTREAM: <carry>: Add deprecated fields to migrate 1.0.0 k8s v1 data
  (jliggitt@redhat.com)
- UPSTREAM: <carry>: SCC (deads@redhat.com)
- UPSTREAM: <carry>: Allow pod start to be delayed in Kubelet
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: Disable UIs for Kubernetes and etcd (deads@redhat.com)
- bump(k8s.io/kubernetes): 4c8e6f47ec23f390978e651232b375f5f9cde3c7
  (deads@redhat.com)
- bump(github.com/coreos/go-etcd): de3514f25635bbfb024fdaf2a8d5f67378492675
  (deads@redhat.com)
- bump(github.com/ghodss/yaml): 73d445a93680fa1a78ae23a5839bad48f32ba1ee
  (deads@redhat.com)
- bump(github.com/fsouza/go-dockerclient):
  1399676f53e6ccf46e0bf00751b21bed329bc60e (deads@redhat.com)
- bump(github.com/prometheus/client_golang):
  3b78d7a77f51ccbc364d4bc170920153022cfd08 (deads@redhat.com)
- Change api version in example apps (jhadvig@redhat.com)
- Bug 1270185 - service link on route details page missing project name
  (jforrest@redhat.com)
- make cherry-pick.sh easier to work with (deads@redhat.com)
- Minor deployment describer formatting fix (ironcladlou@gmail.com)
- Fix deployment config minor ui changes. (sgoodwin@redhat.com)
- Several fixes to the pods page (jforrest@redhat.com)
- test/cmd/export.sh shouldn't dump everything to STDOUT (ccoleman@redhat.com)
- Use the privileged SCC for all kube e2e tests (ccoleman@redhat.com)
- Preserve case of subresources when normalizing URLs (jliggitt@redhat.com)
- Output less info on hack/test-cmd.sh failures (ccoleman@redhat.com)
- Disable potentially insecure TLS cipher suites by default
  (ccoleman@redhat.com)
- Raw sed should not be used in hack/* scripts for Macs (ccoleman@redhat.com)
- Show container metrics in UI (spadgett@redhat.com)
- Fix provisions to be overrides (dmcphers@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  62ec906f6563828364474ef117371ea2ad804dc8 (danw@redhat.com)
- UPSTREAM: Fix non-multitenant pod routing (sdodson@redhat.com)
- Make image trigger test more reliable (ironcladlou@gmail.com)
- [RPMS] atomic-openshift services use openshift bin (sdodson@redhat.com)
- [RPMS] fix rpm build related to sdn restructure (sdodson@redhat.com)
- Fix fuzzing versions in serialization tests (pmorie@gmail.com)
- Show labels on all individual pages. Add label filtering to build config and
  deployment config pages. (jforrest@redhat.com)
- Provide initialized cloud provider in Kubelet config. (jsafrane@redhat.com)
- Fixes to the warning returned by the dc controller (mkargaki@redhat.com)
- Fix govet error (jliggitt@redhat.com)
- adding keystone IdP (sseago@redhat.com)
- Bug 1259260 - when searching docker registry, should not exit in case of no
  matches (ffranz@redhat.com)
- Fix asset config warning (jliggitt@redhat.com)
- Configurable identity mapper strategies (jliggitt@redhat.com)
- Bump proxy resync from 30s to 10m (agoldste@redhat.com)
- Convert secondary nav to a hover menu (sgoodwin@redhat.com)
- remove useless ginkgo test for LDAP (skuznets@redhat.com)
- Sample app readme update (jhadvig@redhat.com)
- Fix s2i build with environment file extended test (mfojtik@redhat.com)
- Return error instead of generating arbitrary names (rhcarvalho@gmail.com)
- Replace local constant with constant from Kube (rhcarvalho@gmail.com)
- Godoc formatting (rhcarvalho@gmail.com)
- Print etcd version when calling openshift version (mfojtik@redhat.com)
- Use cmdutil.PrintSuccess() to display bulk output (ccoleman@redhat.com)
- Add SNI support (jliggitt@redhat.com)
- make ldap sync job accept group/foo whitelists (deads@redhat.com)
- move bash_completion.d/oc to clients package (tdawson@redhat.com)
- Add a cluster diagnostic to check if master is also running as a node.
  (dgoodwin@redhat.com)
- Update the hacking guide (mkargaki@redhat.com)
- add examples for rpm-based installs (jeder@redhat.com)
- Add fibre channel guide (hchen@redhat.com)
- Add Cinder Persistent Volume guide (jsafrane@redhat.com)
- Move NFS documentation to a subchapter. (jsafrane@redhat.com)
- Add environment values to oc new-app help for mysql
  (nakayamakenjiro@gmail.com)
- Update oadm router and registry help message (nakayamakenjiro@gmail.com)
- Enable cpu cfs quota by default (decarr@redhat.com)
- Gluster Docs (screeley@redhat.com)

* Wed Oct 14 2015 Scott Dodson <sdodson@redhat.com> 3.0.2.901
- Build transport for etcd directly (jliggitt@redhat.com)
- Remove volume dir chcon from e2e-docker (agoldste@redhat.com)
- Always try to chcon the volume dir (agoldste@redhat.com)
- Filter service endpoints when using multitenant plugin (danw@redhat.com)
- Update for osdn plugin argument changes (danw@redhat.com)
- Updated generated docs for openshift-sdn changes (danw@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  5a41fee40db41b65578c07eff9fef35d183dce1c (danw@redhat.com)
- Generalize move-upstream.sh (ccoleman@redhat.com)
- Add links to things from the overview and pod template (jforrest@redhat.com)
- deconflict swagger ports (deads@redhat.com)
- Scan for selinux write error in registry diagnostic. (dgoodwin@redhat.com)
- Report transient deployment trigger errors via API field
  (mkargaki@redhat.com)
- Add build timeout, by setting ActiveDeadlineSeconds on a build pod
  (maszulik@redhat.com)
- Update completions (ffranz@redhat.com)
- Append missing flags to cobra flags (jchaloup@redhat.com)
- added an LDAP host label (skuznets@redhat.com)
- add openshift group mapping for ldap sync (deads@redhat.com)
- Convert tables to 2 column layout at mobile res. And fix incorrect url to js
  files. (sgoodwin@redhat.com)
- add ldapsync blacklisting (deads@redhat.com)
- Move to openshift-jvm 1.0.29 (slewis@fusesource.com)
- bump(github.com/openshift/openshift-sdn)
  12f0efeb113058e04e9d333b92bbdddcfc34a9b4 (rpenta@redhat.com)
- Auto generated bash completion and examples doc for oadm pod-network
  (rpenta@redhat.com)
- Remove duplicated helper (rhcarvalho@gmail.com)
- union group name mapper (deads@redhat.com)
- UPSTREAM: 14871: Additional service ports config for master service.
  (abutcher@redhat.com)
- Update completions for kubernetes-service-node-port (abutcher@redhat.com)
- UPSTREAM: 13978 <drop>: NodePort option: Allowing for apiservers behind load-
  balanced endpoint. (abutcher@redhat.com)
- Update image stream page to use a table for the tags (jforrest@redhat.com)
- tighten ldap sync query types (deads@redhat.com)
- refactor building the syncer (deads@redhat.com)
- enhanced active directory ldap sync (deads@redhat.com)
- Use only official Dockerfile parser (rhcarvalho@gmail.com)
- Support oadm pod-network cmd (rpenta@redhat.com)
- added extended tests for LDAP sync (skuznets@redhat.com)
- update ldif to work (deads@redhat.com)
- e2e util has unused import 'regexp' (ccoleman@redhat.com)
- add master API proxy client cert (jliggitt@redhat.com)
- Change CA lifetime defaults (jliggitt@redhat.com)
- UPSTREAM: 15224: Refactor SSH tunneling, fix proxy transport TLS/Dial
  extraction (jliggitt@redhat.com)
- UPSTREAM: 15224: Allow specifying scheme when proxying (jliggitt@redhat.com)
- UPSTREAM: 14889: Honor InsecureSkipVerify flag (jliggitt@redhat.com)
- UPSTREAM: 14967: Add util to set transport defaults (jliggitt@redhat.com)
- Fix for issue where pod template is clipped at mobile res.      - fix
  https://github.com/openshift/origin/issues/4489   - switch to pf-image icon
  - correct icon alignment in pod template        - align label and meta data
  on overview (sgoodwin@redhat.com)
- Fix issue where long text strings extend beyond pod template container at
  mobile res.  - remove flex and min/max width that are no longer needed
  (sgoodwin@redhat.com)
- OS support for host pid and ipc (pweil@redhat.com)
- UPSTREAM:<carry>:hostPid/hostIPC scc support (pweil@redhat.com)
- UPSTREAM:14279:IPC followup (pweil@redhat.com)
- UPSTREAM:<carry>:v1beta3 hostIPC (pweil@redhat.com)
- UPSTREAM:12470:Support containers with host ipc in a pod (pweil@redhat.com)
- UPSTREAM:<carry>:v1beta3 hostPID (pweil@redhat.com)
- UPSTREAM:13447:Allow sharing the host PID namespace (pweil@redhat.com)
- Specify scheme in the jolokia URL (slewis@fusesource.com)
- Fix tag and package name in new-build example (rhcarvalho@gmail.com)
- UPSTREAM: <drop>: disable oidc tests (jliggitt@redhat.com)
- Verify `oc get` returns OpenShift resources (ccoleman@redhat.com)
- UPSTREAM: 15451 <partial>: Add our types to kubectl get error
  (ccoleman@redhat.com)
- Add loading message to all individual pages (jforrest@redhat.com)
- Remove build history and deployment history from main tables
  (jforrest@redhat.com)
- status: Fix incorrect missing registry warning (mkargaki@redhat.com)
- Add configuration options for logging and metrics endpoints
  (spadgett@redhat.com)
- Fix masterCA conversion (jliggitt@redhat.com)
- Fix oc logs (jliggitt@redhat.com)
- Revert "Unique output image stream names in new-app" (ccoleman@redhat.com)
- Bug 1263562 - users without projects should get default ctx when logging in
  (ffranz@redhat.com)
- added LDAP entries for other schemas (skuznets@redhat.com)
- remove cruft from options (deads@redhat.com)
- provide feedback while the ldap group sync job is running (deads@redhat.com)
- Allow POST access to node stats for cluster-reader and system:node-reader
  (jliggitt@redhat.com)
- Update role bindings in compatibility test (jliggitt@redhat.com)
- Add cluster role bindings diagnostic (jliggitt@redhat.com)
- Fix template minification to keep line breaks, remove html files from bindata
  (jforrest@redhat.com)
- Post 4902 fixes (maszulik@redhat.com)
- Add oc rsync command (cewong@redhat.com)
- bump(github.com/openshift/source-to-image)
  1fd4429c584d688d83c1247c03fa2eeb0b083ccb (cewong@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- allocate supplemental groups to namespace (pweil@redhat.com)
- Remove forgotten code no longer used (nagy.martin@gmail.com)
- assets: Fix null dereference in updateTopology() (stefw@redhat.com)
- make oc logs support builds and buildconfigs (deads@redhat.com)
- Wait until the slave pod is gone (nagy.martin@gmail.com)
- UPSTREAM: 14616: Controller framework test flake fix (mfojtik@redhat.com)
- Disable verbose extended run (mfojtik@redhat.com)
- Unique output image stream names in new-app (rhcarvalho@gmail.com)
- Fix start build extended test (mfojtik@redhat.com)
- Make cleanup less noisy (mfojtik@redhat.com)
- Replace default reporter in Ginkgo with SimpleReporter (mfojtik@redhat.com)
- Atomic feature flags followup (miminar@redhat.com)
- examples: Move hello-openshift example to API v1 (stefw@redhat.com)
- Fix Vagrant provisioning after move to contrib/vagrant (dcbw@redhat.com)
- Always bring up openshift's desired network configuration on Vagrant
  provision (dcbw@redhat.com)
- Add annotations to the individual pages (jforrest@redhat.com)
- OS swagger and descriptions (pweil@redhat.com)
- UPSTREAM:<carry>:introduce scc types for fsgroup and supplemental groups
  (pweil@redhat.com)
- ldap sync active directory (deads@redhat.com)
- Change connection-based kubelet auth to application-level authn/authz
  interfaces (jliggitt@redhat.com)
- UPSTREAM: 15232`: refactor logs to be composeable (deads@redhat.com)
- bump(github.com/hashicorp/golang-lru):
  7f9ef20a0256f494e24126014135cf893ab71e9e (jliggitt@redhat.com)
- UPSTREAM: 14700: Add authentication/authorization interfaces to kubelet,
  always include /metrics with /stats (jliggitt@redhat.com)
- UPSTREAM: 14134: sets.String#Intersection (jliggitt@redhat.com)
- UPSTREAM: 15101: Add bearer token support for kubelet client config
  (jliggitt@redhat.com)
- UPSTREAM: 14710: Add verb to authorizer attributes (jliggitt@redhat.com)
- UPSTREAM: 13885: Cherry pick base64 and websocket patches
  (ccoleman@redhat.com)
- Delete --all option for oc export in cli doc (nakayamakenjiro@gmail.com)
- Fix cadvisor in integration test (jliggitt@redhat.com)
- Disable --allow-missing-image test (cewong@redhat.com)
- Make kube-proxy iptables sync period configurable (mkargaki@redhat.com)
- Update to latest version of PatternFly 2.2.0 and Bootstrap  3.3.5
  (sgoodwin@redhat.com)
- PHP hot deploy extended test (jhadvig@redhat.com)
- Ruby hot deploy extended test (jhadvig@redhat.com)
- change show-all default to true (deads@redhat.com)
- Update post-creation messages for builds in CLI (rhcarvalho@gmail.com)
- [Bug 4959] sample-app/cleanup.sh: fix usage of not-installed killall command.
  (vsemushi@redhat.com)
- fix bad oadm line (max.andersen@gmail.com)
- Refactored Openshift Origin builder, decoupled from S2I builder, added mocks
  and testing (kirill.frolov@servian.com)
- Use correct master url for internal token request, set master CA correctly
  (jliggitt@redhat.com)
- fix non-default all-in-one ports for testing (deads@redhat.com)
- Add extended test for git authentication (cewong@redhat.com)
- reconcile-cluster-role-bindings command (jliggitt@redhat.com)
- Change validation timing on create from image page (spadgett@redhat.com)
- Issue 2378 - Show TLS information for routes, create routes and
  routes/routename pages (jforrest@redhat.com)
- Lowercase resource names (rhcarvalho@gmail.com)
- create local images as docker refs (bparees@redhat.com)
- Preserve deployment status sequence (mkargaki@redhat.com)
- Add tests for S2I Perl and Python images with Hot Deploy
  (nagy.martin@gmail.com)
- Test asset config (jliggitt@redhat.com)
- UPSTREAM: 14967: Add util to set transport defaults (jliggitt@redhat.com)
- UPSTREAM: 14246: Fix race in lifecycle admission test (mfojtik@redhat.com)
- Fix send to closed channel in waitForBuild (mfojtik@redhat.com)
- UPSTREAM: 13885: Update error message in wsstream for go 1.5
  (mfojtik@redhat.com)
- Fix go vet (jliggitt@redhat.com)
- Change to use a 503 error page to fully address #4215 This allows custom
  error pages to be layered on in custom haproxy images. (smitram@gmail.com)
- Reduce number of test cases and add cleanup - travis seems to be hitting
  memory errors with starting multiple haproxy routers. (smitram@gmail.com)
- Fixes as per @Miciah review comments. (smitram@gmail.com)
- Update generated completions. (smitram@gmail.com)
- Add/update f5 tests for partition path. (smitram@gmail.com)
- Add partition path support to the f5 router - this will also allows us to
  support sharded routers with f5 using different f5 partitions.
  (smitram@gmail.com)
- Fixes as per @smarterclayton & @pweil- review comments and add generated docs
  and bash completions. (smitram@gmail.com)
- Turn on haproxy statistics by default since its now on a protected page.
  (smitram@gmail.com)
- Bind to router stats options (fixes issue #4884) and add help text
  clarifications. (smitram@gmail.com)
- Add tabs to pod details page (spadgett@redhat.com)
- Bug 1268484 - Use build.metadata.uid to track dismissed builds in UI
  (spadgett@redhat.com)
- cleanup ldap sync validation (deads@redhat.com)
- update auth test to share long running setup (deads@redhat.com)
- make ldap sync-group work (deads@redhat.com)
- fix ldap sync types to be more understandable (deads@redhat.com)
- remove unused mapper (pweil@redhat.com)
- change from kind to resource (pweil@redhat.com)
- fix decoding to handle yaml (deads@redhat.com)
- UPSTREAM: go-ldap: add String for debugging (deads@redhat.com)
- UPSTREAM: 14451: Fix a race in pod backoff. (mfojtik@redhat.com)
- Fix unbound variable in hack/cherry-pick.sh (mfojtik@redhat.com)
- Cleanup godocs of build types (rhcarvalho@gmail.com)
- Update travis to go 1.5.1 (mfojtik@redhat.com)
- Enable Go 1.5 (ccoleman@redhat.com)
- UPSTREAM: Fix typo in e2e pods test (nagy.martin@gmail.com)
- rename allow-missing to allow-missing-images (bparees@redhat.com)
- prune images: Conform to the hacking guide (mkargaki@redhat.com)
- Drop imageStream.spec.dockerImageRepository tags/IDs during conversion
  (ccoleman@redhat.com)
- Add validation to prevent IS.spec.dockerImageRepository from having tags
  (ccoleman@redhat.com)
- Add a helper to move things upstream (ccoleman@redhat.com)
- Cherry-pick helper (ccoleman@redhat.com)
- Clean up root folder (ccoleman@redhat.com)
- UPSTREAM: 13885: Support websockets on exec and pod logs
  (ccoleman@redhat.com)
- add test for patching anonymous fields in structs (deads@redhat.com)
- UPSTREAM: 14985: fix patch for anonymous struct fields (deads@redhat.com)
- fix exec admission controller flake (deads@redhat.com)
- updated template use (skuznets@redhat.com)
- fixed go vet invocation and errors (skuznets@redhat.com)
- Add ethtool to base/Dockerfile.rhel7 too (sdodson@redhat.com)
- [RPMS] atomic-openshift services use openshift bin (sdodson@redhat.com)
- [RPMS] fix rpm build related to sdn restructure (sdodson@redhat.com)
- UPSTREAM: 14831: allow yaml as argument to patch (deads@redhat.com)
- Move host etc master/node directories to official locations.
  (dgoodwin@redhat.com)
- Wait for service account to be accessible (mfojtik@redhat.com)
- Wait for builder account (mfojtik@redhat.com)
- Pull RHEL7 images from internal CI registry (mfojtik@redhat.com)
- Initial addition of S2I SCL enablement extended tests (mfojtik@redhat.com)
- tolerate missing docker images (bparees@redhat.com)
- bump(github.com/spf13/cobra): d732ab3a34e6e9e6b5bdac80707c2b6bad852936
  (ffranz@redhat.com)
- Rename various openshift directories to origin. (dgoodwin@redhat.com)
- allow SAR requests in lifecycle admission (pweil@redhat.com)
- Issue 4001 - add requests and limits to resource limits on settings page
  (jforrest@redhat.com)
- prevent force pull set up from running in other focuses; add some debug clues
  in hack/util.sh; comments from Cesar, Ben; create new builder images so we
  run concurrent;  MIchal's comments; move to just one builder
  (gmontero@redhat.com)
- fix govet example error (skuznets@redhat.com)
- Add Restart=always to master service (sdodson@redhat.com)
- Issue 4867 - route links should open in a new window (jforrest@redhat.com)
- Fix output of git basic credentials in builder (cewong@redhat.com)
- Set build status message in case of error (rhcarvalho@gmail.com)
- Fix the reference of openshift command in Makefile (akira@tagoh.org)
- Issue 4632 - remove 'Project' from the project overview header
  (jforrest@redhat.com)
- Issue 4860 - missing no deployments msg when only have RCs
  (jforrest@redhat.com)
- Support deployment hook volume inheritance (ironcladlou@gmail.com)
- fix RAR test flake (deads@redhat.com)
- UPSTREAM: 14688: Deflake max in flight (deads@redhat.com)
- Fix vagrant provisioning (danw@redhat.com)
- setup makefile to be parallelizeable (deads@redhat.com)
- api group support for authorizer (deads@redhat.com)
- Apply OOMScoreAdjust and Restart policy to openshift node (decarr@redhat.com)
- Fix nit (dmcphers@redhat.com)
- Add ethtool to our deps (mkargaki@redhat.com)
- Issue 4855 - warning flickers about build config and deployment config not
  existing (jforrest@redhat.com)
- Add MySQL extended replication tests (nagy.martin@gmail.com)
- extended: Disable Daemon tests (mkargaki@redhat.com)
- Adds explicit suggestions for some cli commands (ffranz@redhat.com)
- bump(github.com/spf13/pflag): b084184666e02084b8ccb9b704bf0d79c466eb1d
  (ffranz@redhat.com)
- bump(github.com/cpf13/cobra): 046a67325286b5e4d7c95b1d501ea1cd5ba43600
  (ffranz@redhat.com)
- Don't show errors in name field until blurred (spadgett@redhat.com)
- Allow whitespace-only values in UI for required parameters
  (spadgett@redhat.com)
- make RAR allow evaluation errors (deads@redhat.com)
- Watch routes on individual service page (spadgett@redhat.com)
- bump(github.com/vjeantet/ldapserver) 19fbc46ed12348d5122812c8303fb82e49b6c25d
  (mkargaki@redhat.com)
- Bug 1266859: UPSTREAM: <drop>: expose: Truncate service names
  (mkargaki@redhat.com)
- Update docs regarding the rebase (mkargaki@redhat.com)
- add timing statements (deads@redhat.com)
- Routes should be able to specify which port they desire (ccoleman@redhat.com)
- Add dbus for the OVS setup required docker restart. (dgoodwin@redhat.com)
- Return to OSBS preferred labels. (dgoodwin@redhat.com)
- Source info should only be loaded when Git is used for builds
  (ccoleman@redhat.com)
- Rename ose3 node run script to match aos. (dgoodwin@redhat.com)
- Bump aos-master and aos-node to 3.0.2.100. (dgoodwin@redhat.com)
- Rename ose3 script to matching aos, standardize labels. (dgoodwin@redhat.com)
- Merge in latest work from oseonatomic repo. (dgoodwin@redhat.com)
- Expose version as prometheus metric (jimmidyson@gmail.com)
- Cleanup unused code and add proper labels. (dgoodwin@redhat.com)
- Update for AOS 3.1 version and a more consistent bz component name.
  (dgoodwin@redhat.com)
- Add missing Name label. (dgoodwin@redhat.com)
- Cleanup aos-master image for OSBS build. (dgoodwin@redhat.com)
- aos-node container building locally. (dgoodwin@redhat.com)
- Use Dockerfile.product for ose-master to future proof a bit.
  (dgoodwin@redhat.com)
- Move to Dockerfile.product for future upstream compatability.
  (dgoodwin@redhat.com)
- Initial draft of aos-master container. (dgoodwin@redhat.com)

* Tue Sep 29 2015 Scott Dodson <sdodson@redhat.com> 3.0.2.900
- OSE Docs URLs (sdodson@redhat.com)
- Do not treat directories named Dockerfile as file (rhcarvalho@gmail.com)
- Disable the pods per node test - it requires the kubelet stats
  (ccoleman@redhat.com)
- Set Build.Status.Pushspec to resolved pushspec (rhcarvalho@gmail.com)
- Bug: 1266442 1266447 (jhadvig@redhat.com)
- new-app: Better output in case of invalid Dockerfile (mkargaki@redhat.com)
- Compatibility test for Volume Source (mkargaki@redhat.com)
- Update UPGRADE.md about Metadata/Downward (mkargaki@redhat.com)
- get local IP like the server would and add retries to build test watch
  (pweil@redhat.com)
- Interesting refactoring (mkargaki@redhat.com)
- Fix verify-open-ports.sh to not fail on success (mkargaki@redhat.com)
- Boring refactoring; code generations (mkargaki@redhat.com)
- UPSTREAM: openshift-sdn: 167: plugins: Update Kube client imports
  (mkargaki@redhat.com)
- UPSTREAM: <carry>: Move to test pkg to avoid linking test flags in binaries
  (pweil@redhat.com)
- UPSTREAM: <carry>: Add etcd prefix (mkargaki@redhat.com)
- UPSTREAM: 14664: fix testclient prepend (deads@redhat.com)
- UPSTREAM: 14502: don't fatal on missing sorting flag (deads@redhat.com)
- UPSTREAM: <drop>: hack experimental versions and client creation
  (deads@redhat.com)
- UPSTREAM: 14496: deep-copies: Structs cannot be nil (mkargaki@redhat.com)
- UPSTREAM: <carry>: Back n forth downward/metadata conversions
  (mkargaki@redhat.com)
- UPSTREAM: 13728: Allow to replace os.Exit() with panic when CLI command fatal
  (mfojtik@redhat.com)
- UPSTREAM: 14291: add patch verb to APIRequestInfo (deads@redhat.com)
- UPSTREAM: 13910: Fix resourcVersion = 0 in cacher (mkargaki@redhat.com)
- UPSTREAM: 13864: Fix kubelet logs --follow bug (mkargaki@redhat.com)
- UPSTREAM: 14063: enable system CAs (mkargaki@redhat.com)
- UPSTREAM: 13756: expose: Avoid selector resolution if a selector is not
  needed (mkargaki@redhat.com)
- UPSTREAM: 13746: Fix field=metadata.name (ccoleman@redhat.com)
- UPSTREAM: 9870: Allow Volume Plugins to be configurable (deads@redhat.com)
- UPSTREAM: 11827: allow permissive SA secret ref limitting (deads@redhat.com)
- UPSTREAM: 12221: Allow custom namespace creation in e2e framework
  (mfojtik@redhat.com)
- UPSTREAM: 12498: Re-add timeouts for kubelet which is not in the upstream PR.
  (deads@redhat.com)
- UPSTREAM: 9009: Retry service account update when adding token reference
  (deads@redhat.com)
- UPSTREAM: 9844: EmptyDir volume SELinux support (deads@redhat.com)
- UPSTREAM: 7893: scc allocation interface methods (deads@redhat.com)
- UPSTREAM: 7893: scc (pweil@redhat.com)
- UPSTREAM: 8890: Allowing ActiveDeadlineSeconds to be updated for a pod
  (deads@redhat.com)
- UPSTREAM: <drop>: add back flag types to reduce noise during this rebase
  (deads@redhat.com)
- UPSTREAM: <none>: Hack date-time format on *util.Time (ccoleman@redhat.com)
- UPSTREAM: <none>: Suppress aggressive output of warning (ccoleman@redhat.com)
- UPSTREAM: <carry>: Disable --validate by default (mkargaki@redhat.com)
- UPSTREAM: <carry>: update describer for dockercfg secrets (deads@redhat.com)
- UPSTREAM: <carry>: reallow the ability to post across namespaces in api
  (pweil@redhat.com)
- UPSTREAM: <carry>: support pointing oc exec to old openshift server
  (deads@redhat.com)
- UPSTREAM: <carry>: Add deprecated fields to migrate 1.0.0 k8s v1 data
  (jliggitt@redhat.com)
- UPSTREAM: <carry>: Allow pod start to be delayed in Kubelet
  (ccoleman@redhat.com)
- UPSTREAM: <carry>: Disable UIs for Kubernetes and etcd (deads@redhat.com)
- UPSTREAM: <carry>: v1beta3 (deads@redhat.com)
- bump(github.com/emicklei/go-restful) 1f9a0ee00ff93717a275e15b30cf7df356255877
  (mkargaki@redhat.com)
- bump(k8s.io/kubernetes) 86b4e777e1947c1bc00e422306a3ca74cbd54dbe
  (mkargaki@redhat.com)
- Update java console (slewis@fusesource.com)
- fix QueryForEntries API (deads@redhat.com)
- added sync-groups command basics (skuznets@redhat.com)
- add ldap groups sync (skuznets@redhat.com)
- Remove templates.js from bindata and fix HTML minification
  (spadgett@redhat.com)
- fedora 21 Vagrant provisioning fixes (dcbw@redhat.com)
- Issue 4795 - include ref and contextdir in github links (jforrest@redhat.com)
- new-app: Actually use the Docker parser (mkargaki@redhat.com)
- Update old references to _output/local/go/bin (rhcarvalho@gmail.com)
- improved robustness of recycler script (mturansk@redhat.com)
- Do not export test utility (rhcarvalho@gmail.com)
- Change build-go to generate binaries to _output/local/bin/${platform}
  (ccoleman@redhat.com)
- Make deployment trigger logging quieter (ironcladlou@gmail.com)
- bump(github.com/openshift/openshift-sdn)
  669deb4de23ab7f79341a132786b198c7f272082 (rpenta@redhat.com)
- Fix openshift-sdn imports in origin (rpenta@redhat.com)
- Move plugins/osdn to Godeps/workspace/src/github.com/openshift/openshift-
  sdn/plugins/osdn (rpenta@redhat.com)
- Move sdn ovssubnet to pkg/ovssubnet dir (rpenta@redhat.com)
- Reorganize web console create flow (spadgett@redhat.com)
- Fix handle on watchObject. Set deletionTimestamp instead of deleted.
  (jforrest@redhat.com)
- Use direct CLI invocations rather than embedding CLI (ccoleman@redhat.com)
- Next steps page (after creating stuff in console) (ffranz@redhat.com)
- Disable parallelism for now (ccoleman@redhat.com)
- Do not require the master components from test/util (ccoleman@redhat.com)
- [userinterface_public_538] Create individual pages for all resources With
  some changes from @sg00dwin and @spadgett (jforrest@redhat.com)
- Fix some of the govet issues (mfojtik@redhat.com)
- annotate builds on clone (bparees@redhat.com)
- Make network fixup during provision conditional (marun@redhat.com)
- Update roadmap url (dmcphers@redhat.com)
- better error message for immutable edits to builds (bparees@redhat.com)
- Default NetworkConfig.ServiceNetworkCIDR to
  KubernetesMasterConfig.ServicesSubnet (jliggitt@redhat.com)
- create SCCExecRestriction admission plugin (deads@redhat.com)
- added oadm commands to validate node and master config (skuznets@redhat.com)
- Add an e2e test for Dockerfile and review comments (ccoleman@redhat.com)
- Add extended tests for start-build (mfojtik@redhat.com)
- allow local access reviews while namespace is terminating (deads@redhat.com)
- build oc, move to clients, move alt clients to redistributable
  (tdawson@redhat.com)
- Add --kubeconfig support for compat with kubectl (ccoleman@redhat.com)
- assets: Filter topology correctly and refactor relations (stefw@redhat.com)
- allow self SARs using old policy: (deads@redhat.com)
- Capture panic in extended CLI and return them as Go errors
  (mfojtik@redhat.com)
- UPSTREAM: 13728: Allow to replace os.Exit() with panic when CLI command fatal
  (mfojtik@redhat.com)
- Take stdin on OpenShift CLI (ccoleman@redhat.com)
- add extended tests for example repos (bparees@redhat.com)
- added syntax highlighting to readme (skuznets@redhat.com)
- Retry finalizer on conflict error (decarr@redhat.com)
- Build from a Dockerfile directly (ccoleman@redhat.com)
- fix backwards poll args (bparees@redhat.com)
- Add missing rolling hook conversions (ironcladlou@gmail.com)
- Fix potential race conditions during SDN setup (rpenta@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  0f9e6558e8dceb8c8317e3587d9c9c94ae07ecb8 (rpenta@redhat.com)
- Add missing rolling hook conversions (ironcladlou@gmail.com)
- Make "default" an admin namespace in multitenant (danw@redhat.com)
- add patch to default roles (deads@redhat.com)
- Exclude a few more tests (ccoleman@redhat.com)
- Refactor setting and resetting HTTP proxies (rhcarvalho@gmail.com)
- UPSTREAM: 14291: add patch verb to APIRequestInfo (deads@redhat.com)
- Add --portal-net back to all-in-one args (danw@redhat.com)
- Bump kubernetes-ui-label-selector to v0.0.10 - fixes js error
  (jforrest@redhat.com)
- Fix broken link in readme (mtayer@redhat.com)
- add SA role bindings to auto-provisioned namespaces (deads@redhat.com)
- diagnostics: fail gracefully on broken kubeconfig (lmeyer@redhat.com)
- Improve systemd detection for diagnostics. (dgoodwin@redhat.com)
- Refactor pkg/build/builder (rhcarvalho@gmail.com)
- Set kubeconfig in extended tests (ccoleman@redhat.com)
- Extended failure (ccoleman@redhat.com)
- Only run k8s upstream tests that are passing (ccoleman@redhat.com)
- Add example env vars (rhcarvalho@gmail.com)
- Pass env vars defined in Docker build strategy (rhcarvalho@gmail.com)
- [RPMs] Ease the upgrade to v1.0.6 (sdodson@redhat.com)
- BZ1221441 - new filter that shows unique project name (rafabene@gmail.com)
- Push F5 image (ccoleman@redhat.com)
- Making the regex in ./test/cmd/admin.sh a little more flexible for downstream
  (bleanhar@redhat.com)
- Making the regex in ./test/cmd/admin.sh a little more flexible for downstream
  (bleanhar@redhat.com)
- Add generated-by annotation to CLI new-app and web console
  (mfojtik@redhat.com)
- disable go vet in make check-test (skuznets@redhat.com)
- more comments (pweil@redhat.com)
- add cluster roles to diagnostics (deads@redhat.com)
- UPSTREAM: 14063: enable system CAs (deads@redhat.com)
- Linux 386 cross compile (ccoleman@redhat.com)
- Add X-Forwarded-* headers and the new Forwarded header for rfc7239 so that
  the backend has info about the proxied request (and requestor).
  (smitram@gmail.com)
- switch to https for sample repo url (bparees@redhat.com)
- Add SA secret checking to SA readiness test in integration tests
  (cewong@redhat.com)
- Adding source secret (jhadvig@redhat.com)
- disable go vet in make check-test (skuznets@redhat.com)
- Update bash-completion (nakayamakenjiro@gmail.com)
- Allow metadata on images to be edited after creation (ccoleman@redhat.com)
- Enable linux-386 (ccoleman@redhat.com)
- .commit is a file, not a directory (ccoleman@redhat.com)
- Retry deployment resource updates (ironcladlou@gmail.com)
- Improve latest deployment output (ironcladlou@gmail.com)
- Make release extraction a separate step for builds (ccoleman@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  0f9e6558e8dceb8c8317e3587d9c9c94ae07ecb8 (rpenta@redhat.com)
- Fix potential race conditions during SDN setup (rpenta@redhat.com)
- Fix casing of output (ironcladlou@gmail.com)
- Adjust help templates to latest version of Cobra (ffranz@redhat.com)
- Update generated completions (ffranz@redhat.com)
- bump(github.com/spf13/cobra): 6d7031177028ad8c5b4b428ac9a2288fbc1c0649
  (ffranz@redhat.com)
- bump(github.com/spf13/pflag): 8e7dc108ab3a1ab6ce6d922bbaff5657b88e8e49
  (ffranz@redhat.com)
- Update to version 0.0.9 for kubernetes-label-selector. Fixes issue
  https://github.com/openshift/origin/issues/3180 (sgoodwin@redhat.com)
- UPSTREAM: 211: Allow listen only ipv4 (ccoleman@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- bump(github.com/skynetservices/skydns):bb2ebadc9746f23e4a296e3cbdb8c01e956bae
  e1 (jimmidyson@gmail.com)
- Fixes #4494: Don't register skydns metrics on nodes (jimmidyson@gmail.com)
- Move positional parameters before package lists (nagy.martin@gmail.com)
- Simplify the readme to point to docs (ccoleman@redhat.com)
- Normalize extended tests into test/extended/*.sh (ccoleman@redhat.com)
- Improve deploy --cancel output (ironcladlou@gmail.com)
- Bump min Docker version in docs (agoldste@redhat.com)
- Normalize extended tests into test/extended/*.sh (ccoleman@redhat.com)
- Return empty Config field in FromName to avoid nil pointer error
  (nakayamakenjiro@gmail.com)
- docs: Fixed broken links in openshift_model.md. (stevem@gnulinux.net)
- Show corrent error message if passed json template is invalid
  (prukner.jan@seznam.cz)
- Clean up test directories (ccoleman@redhat.com)
- hack/build-images.sh fails on vboxfs due to hardlink (ccoleman@redhat.com)
- Fail with stack trace in test bash (ccoleman@redhat.com)
- Make oc rsh behave more like ssh (ccoleman@redhat.com)
- better error messages for parameter errors (bparees@redhat.com)
- Simplify the release output, create a zip (ccoleman@redhat.com)
- Cleaning useless colon (remy.binsztock@tech-angels.com)
- app: Implement initial topology-graph based view (stefw@redhat.com)
- app: Normalize kind property on retrieved items (stefw@redhat.com)
- app: Toggle overview modes between tiles and topology (stefw@redhat.com)
- Test for exposing external services (mkargaki@redhat.com)
- UPSTREAM: 13756: expose: Avoid selector resolution if a selector is not
  needed (mkargaki@redhat.com)
- Add helper script to run kube e2e tests (marun@redhat.com)
- Stop adding user to 'docker' group (marun@redhat.com)

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
