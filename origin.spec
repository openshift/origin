#debuginfo not supported with Go
%global debug_package %{nil}
# modifying the Go binaries breaks the DWARF debugging
%global __os_install_post %{_rpmconfigdir}/brp-compress

%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin

# docker_version is the version of docker requires by packages
%global docker_version 1.13
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.6.1
# this is the version we obsolete up to. The packaging changed for Origin
# 1.0.6 and OSE 3.1 such that 'openshift' package names were no longer used.
%global package_refactor_version 3.0.2.900
%global golang_version 1.9.1
# %commit and %os_git_vars are intended to be set by tito custom builders provided
# in the .tito/lib directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit 86b5e46426ba828f49195af21c56f7c6674b48f7
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# os_git_vars needed to run hack scripts during rpm builds
%{!?os_git_vars:
%global os_git_vars OS_GIT_VERSION='' OS_GIT_COMMIT='' OS_GIT_MAJOR='' OS_GIT_MINOR='' OS_GIT_TREE_STATE=''
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

%{!?version: %global version 0.0.1}
%{!?release: %global release 1}

Name:           %{package_name}
Version:        %{version}
Release:        %{release}%{package_dist}
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
Obsoletes:      openshift < %{package_refactor_version}

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

%package hypershift
Summary:        %{product_name} server commands

%description hypershift
%{summary}

%package hyperkube
Summary:        %{product_name} Kubernetes server commands

%description hyperkube
%{summary}

%package master
Summary:        %{product_name} Master
Requires:       %{name} = %{version}-%{release}
Obsoletes:      openshift-master < %{package_refactor_version}

%description master
%{summary}

%package tests
Summary: %{product_name} Test Suite

%description tests
%{summary}

%package node
Summary:        %{product_name} Node
Requires:       %{name} = %{version}-%{release}
Requires:       %{name}-hyperkube = %{version}-%{release}
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
Obsoletes:      openshift-node < %{package_refactor_version}
Obsoletes:      tuned-profiles-%{name}-node
Provides:       tuned-profiles-%{name}-node

%description node
%{summary}

%package clients
Summary:        %{product_name} Client binaries for Linux
Obsoletes:      openshift-clients < %{package_refactor_version}
Requires:       bash-completion

%description clients
%{summary}

%if 0%{?make_redistributable}
%package clients-redistributable
Summary:        %{product_name} Client binaries for Linux, Mac OSX, and Windows
Obsoletes:      openshift-clients-redistributable < %{package_refactor_version}
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
Obsoletes:        openshift-sdn-ovs < %{package_refactor_version}

%description sdn-ovs
%{summary}

%package template-service-broker
Summary: Template Service Broker
%description template-service-broker
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
%endif

# Generate man pages
%{os_git_vars} hack/generate-docs.sh
%endif

%install

PLATFORM="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
install -d %{buildroot}%{_bindir}

# Install linux components
for bin in oc oadm openshift hypershift hyperkube template-service-broker openshift-node-config
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
    openshift-router
do
    ln -s openshift %{buildroot}%{_bindir}/$cmd
done

ln -s oc %{buildroot}%{_bindir}/kubectl

install -d -m 0755 %{buildroot}%{_sysconfdir}/origin/{master,node}

# different service for origin vs aos
# install -m 0644 contrib/systemd/%{name}-node.service %{buildroot}%{_unitdir}/%{name}-node.service
# same sysconfig files for origin vs aos
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

%files
%doc README.md
%license LICENSE
%{_bindir}/openshift
%{_bindir}/openshift-deploy
%{_bindir}/openshift-f5-router
%{_bindir}/openshift-recycle
%{_bindir}/openshift-router
%{_bindir}/openshift-docker-build
%{_bindir}/openshift-sti-build
%{_bindir}/openshift-git-clone
%{_bindir}/openshift-extract-image-content
%{_bindir}/openshift-manage-dockerfile
%{_sharedstatedir}/origin
%{_sysconfdir}/bash_completion.d/openshift
%defattr(-,root,root,0700)
%dir %config(noreplace) %{_sysconfdir}/origin
%ghost %dir %config(noreplace) %{_sysconfdir}/origin
%ghost %config(noreplace) %{_sysconfdir}/origin/.config_managed
%{_mandir}/man1/openshift*

%files tests
%{_libexecdir}/%{name}
%{_libexecdir}/%{name}/extended.test

%files hypershift
%{_bindir}/hypershift
%defattr(-,root,root,0700)

%files hyperkube
%{_bindir}/hyperkube
%defattr(-,root,root,0700)

%files master
%defattr(-,root,root,0700)
%config(noreplace) %{_sysconfdir}/origin/master

%files node
%{_bindir}/openshift-node-config
%{_sysconfdir}/systemd/system.conf.d/origin-accounting.conf
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}-node
%defattr(-,root,root,0700)
%config(noreplace) %{_sysconfdir}/origin/node

%preun node
%systemd_preun %{name}-node.service

%postun node
%systemd_postun

%files sdn-ovs
%dir %{_sysconfdir}/cni/net.d
%dir /opt/cni/bin
/opt/cni/bin/*

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

%changelog
