#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%global kube_plugin_path /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet
%global sdn_import_path github.com/openshift/openshift-sdn

# docker_version is the version of docker requires by packages
%global docker_verison 1.6.2
# tuned_version is the version of tuned requires by packages
%global tuned_version  2.3
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.3.1
# %commit and %ldflags are intended to be set by tito custom builders provided
# in the rel-eng directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit 86b5e46426ba828f49195af21c56f7c6674b48f7
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# OpenShift specific ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 0 -X github.com/openshift/origin/pkg/version.minorFromGit 0+ -X github.com/openshift/origin/pkg/version.versionFromGit v0.0.1 -X github.com/openshift/origin/pkg/version.commitFromGit 86b5e46 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit 6241a21 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion v0.11.0-330-g6241a21
}

Name:           openshift
# Version is not kept up to date and is intended to be set by tito custom
# builders provided in the rel-eng directory of this project
Version:        0.0.1
Release:        0%{?dist}
Summary:        Open Source Platform as a Service by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz

BuildRequires:  systemd
BuildRequires:  golang >= 1.4


%description
%{summary}

%package master
Summary:        OpenShift Master
Requires:       %{name} = %{version}-%{release}
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description master
%{summary}

%package node
Summary:        OpenShift Node
Requires:       %{name} = %{version}-%{release}
Requires:       docker-io >= %{docker_version}
Requires:       tuned-profiles-%{name}-node
Requires:       util-linux
Requires:       socat
Requires:       nfs-utils
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description node
%{summary}

%package -n tuned-profiles-%{name}-node
Summary:        Tuned profiles for OpenShift Node hosts
Requires:       tuned >= %{tuned_version}
Requires:       %{name} = %{version}-%{release}

%description -n tuned-profiles-%{name}-node
%{summary}

%package clients
Summary:      Openshift Client binaries for Linux, Mac OSX, and Windows
BuildRequires: golang-pkg-darwin-amd64
BuildRequires: golang-pkg-windows-386

%description clients
%{summary}

%package dockerregistry
Summary:        Docker Registry v2 for OpenShift
Requires:       %{name} = %{version}-%{release}

%description dockerregistry
%{summary}

%package pod
Summary:        OpenShift Pod
Requires:       %{name} = %{version}-%{release}

%description pod
%{summary}

%package sdn-ovs
Summary:          OpenShift SDN Plugin for Open vSwitch
Requires:         openvswitch >= %{openvswitch_version}
Requires:         %{name}-node = %{version}-%{release}
Requires:         bridge-utils
Requires:         ethtool

%description sdn-ovs
%{summary}


# ====
%package -n atomic-enterprise
Summary:         Open Source Platform as a Service by Red Hat


%description -n atomic-enterprise
%{summary}


%package -n atomic-enterprise-master
Summary:        Atomic Enterprise Master
Requires:       atomic-enterprise = %{version}-%{release}
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description -n atomic-enterprise-master
%{summary}

%package -n atomic-enterprise-node
Summary:        Origin Node
Requires:       atomic-enterprise = %{version}-%{release}
Requires:       docker-io >= %{docker_version}
Requires:       tuned-profiles-atomic-enterprise-node
Requires:       util-linux
Requires:       socat
Requires:       nfs-utils
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description -n atomic-enterprise-node
%{summary}

%package -n tuned-profiles-atomic-enterprise-node
Summary:        Tuned profiles for Origin Node hosts
Requires:       tuned >= %{tuned_version}
Requires:       atomic-enterprise = %{version}-%{release}

%description -n tuned-profiles-atomic-enterprise-node
%{summary}

%package -n atomic-enterprise-clients
Summary:      Origin Client binaries for Linux, Mac OSX, and Windows
BuildRequires: golang-pkg-windows-386

%description -n atomic-enterprise-clients
%{summary}

%package -n atomic-enterprise-dockerregistry
Summary:        Docker Registry v2 for Origin
Requires:       atomic-enterprise = %{version}-%{release}

%description -n atomic-enterprise-dockerregistry
%{summary}

%package -n atomic-enterprise-pod
Summary:        Origin Pod
Requires:       atomic-enterprise = %{version}-%{release}

%description -n atomic-enterprise-pod
%{summary}

%package -n atomic-enterprise-sdn-ovs
Summary:          Origin SDN Plugin for Open vSwitch
Requires:         openvswitch >= %{openvswitch_version}
Requires:         atomic-enterprise-node = %{version}-%{release}
Requires:         bridge-utils
Requires:         ethtool

%description -n atomic-enterprise-sdn-ovs
%{summary}
# ========

%prep
%setup -q

%build

# Don't judge me for this ... it's so bad.
mkdir _build

# Horrid hack because golang loves to just bundle everything
pushd _build
    mkdir -p src/github.com/openshift
    ln -s $(dirs +1 -l) src/%{import_path}
popd


# Gaming the GOPATH to include the third party bundled libs at build
# time. This is bad and I feel bad.
mkdir _thirdpartyhacks
pushd _thirdpartyhacks
    ln -s \
        $(dirs +1 -l)/Godeps/_workspace/src/ \
            src
popd
export GOPATH=$(pwd)/_build:$(pwd)/_thirdpartyhacks:%{buildroot}%{gopath}:%{gopath}
# Build all linux components we care about
for cmd in openshift dockerregistry
do
        go install -ldflags "%{ldflags}" %{import_path}/cmd/${cmd}
done

# Build only 'openshift' for other platforms
GOOS=windows GOARCH=386 go install -ldflags "%{ldflags}" %{import_path}/cmd/openshift
GOOS=darwin GOARCH=amd64 go install -ldflags "%{ldflags}" %{import_path}/cmd/openshift

#Build our pod
pushd images/pod/
    go build -ldflags "%{ldflags}" pod.go
popd

%install

install -d %{buildroot}%{_bindir}

# Install linux components
for bin in openshift dockerregistry
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 _build/bin/${bin} %{buildroot}%{_bindir}/${bin}
done

# openshift == atomic-enterprise
install -p -m 755 _build/bin/openshift _build/bin/atomic-enterprise
install -p -m 755 _build/bin/atomic-enterprise %{buildroot}%{_bindir}/atomic-enterprise

# Install 'openshift' as client executable for windows and mac
for pkgname in openshift atomic-enterprise
do
  install -d %{buildroot}%{_datadir}/${pkgname}/{linux,macosx,windows}
  install -p -m 755 _build/bin/openshift %{buildroot}%{_datadir}/${pkgname}/linux/oc
  install -p -m 755 _build/bin/darwin_amd64/openshift %{buildroot}%{_datadir}/${pkgname}/macosx/oc
  install -p -m 755 _build/bin/windows_386/openshift.exe %{buildroot}%{_datadir}/${pkgname}/windows/oc.exe
done

#Install openshift pod
install -p -m 755 images/pod/pod %{buildroot}%{_bindir}/

install -d -m 0755 %{buildroot}%{_unitdir}

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig

for cmd in oc oadm; do
    ln -s %{_bindir}/%{name} %{buildroot}%{_bindir}/$cmd
done
ln -s %{_bindir}/%{name} %{buildroot}%{_bindir}/kubectl

install -d -m 0755 %{buildroot}%{_sysconfdir}/origin/{master,node,allinone}
# Atomic-Enterprise has an allinone directory for all-in-one use.
install -d -m 0755 %{buildroot}%{_sysconfdir}/origin/allinone/{master,node}

for pkgname in openshift atomic-enterprise
do
  install -m 0644 rel-eng/${pkgname}-master.service %{buildroot}%{_unitdir}/${pkgname}-master.service
  install -m 0644 rel-eng/${pkgname}-node.service %{buildroot}%{_unitdir}/${pkgname}-node.service

  install -m 0644 rel-eng/${pkgname}-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/${pkgname}-master
  install -m 0644 rel-eng/${pkgname}-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/${pkgname}-node
  install -d -m 0755 %{buildroot}%{_prefix}/lib/tuned/${pkgname}-node-{guest,host}
  install -m 0644 tuned/%{name}-node-guest/tuned.conf %{buildroot}%{_prefix}/lib/tuned/${pkgname}-node-guest/tuned.conf
  install -m 0644 tuned/%{name}-node-host/tuned.conf %{buildroot}%{_prefix}/lib/tuned/${pkgname}-node-host/tuned.conf
  install -d -m 0755 %{buildroot}%{_mandir}/man7
  install -m 0644 tuned/man/tuned-profiles-%{name}-node.7 %{buildroot}%{_mandir}/man7/tuned-profiles-${pkgname}-node.7

done

# Atomic-Enterprise has an additional unit and config
install -m 0644 rel-eng/atomic-enterprise-allinone.service %{buildroot}%{_unitdir}/atomic-enterprise-allinone.service
install -m 0644 rel-eng/atomic-enterprise-allinone.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/atomic-enterprise-allinone

mkdir -p %{buildroot}%{_sharedstatedir}/%{name}
mkdir -p %{buildroot}%{_sharedstatedir}/origin


# Install sdn scripts
install -d -m 0755 %{buildroot}%{kube_plugin_path}
install -d -m 0755 %{buildroot}%{_unitdir}/docker.service.d
install -p -m 0644 rel-eng/docker-sdn-ovs.conf %{buildroot}%{_unitdir}/docker.service.d/
for pkgname in openshift atomic-enterprise
do

  pushd _thirdpartyhacks/src/%{sdn_import_path}/ovssubnet/controller/kube/bin
     install -p -m 755 %{name}-ovs-subnet %{buildroot}%{kube_plugin_path}/${pkgname}-ovs-subnet
     install -p -m 755 %{name}-sdn-kube-subnet-setup.sh %{buildroot}%{_bindir}/${pkgname}-sdn-kube-subnet-setup.sh
  popd
  pushd _thirdpartyhacks/src/%{sdn_import_path}/ovssubnet/controller/multitenant/bin
     install -p -m 755 %{name}-ovs-multitenant %{buildroot}%{_bindir}/${pkgname}-ovs-multitenant
     install -p -m 755 %{name}-sdn-multitenant-setup.sh %{buildroot}%{_bindir}/${pkgname}-sdn-multitenant-setup.sh
  popd
  install -d -m 0755 %{buildroot}%{_unitdir}/${pkgname}-node.service.d
  install -p -m 0644 rel-eng/%{name}-sdn-ovs.conf %{buildroot}%{_unitdir}/${pkgname}-node.service.d/${pkgname}-sdn-ovs.conf
done

# Install bash completions
install -d -m 755 %{buildroot}%{_sysconfdir}/bash_completion.d/
install -p -m 644 rel-eng/completions/bash/* %{buildroot}%{_sysconfdir}/bash_completion.d/
# Generate atomic-enterprise bash completions
%{__sed} -e "s|openshift|atomic-enterprise|g" rel-eng/completions/bash/openshift > %{buildroot}%{_sysconfdir}/bash_completion.d/atomic-enterprise

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/oc
%{_bindir}/oadm
%{_bindir}/kubectl
%{_sharedstatedir}/%{name}
%{_sysconfdir}/bash_completion.d/*
%dir %config(noreplace) %{_sysconfdir}/origin

%pre
# If /etc/openshift exists symlink it to /etc/origin
if [ -d "%{_sysconfdir}/openshift" ]; then
  ln -s %{_sysconfdir}/openshift %{_sysconfdir}/origin
fi

%files master
%defattr(-,root,root,-)
%{_unitdir}/%{name}-master.service
%ghost %config(noreplace) %{_sysconfdir}/sysconfig/%{name}-master
%ghost %config(noreplace) %{_sysconfdir}/origin/master
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

%post master
%systemd_post %{basename:openshift-master.service}

%preun master
%systemd_preun %{basename:openshift-master.service}

%postun master
%systemd_postun

%files node
%defattr(-,root,root,-)
%{_unitdir}/%{name}-node.service
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}-node
%config(noreplace) %{_sysconfdir}/origin/node

%post node
%systemd_post %{basename:openshift-node.service}

%preun node
%systemd_preun %{basename:openshift-node.service}

%postun node
%systemd_postun

%files sdn-ovs
%defattr(-,root,root,-)
%{_bindir}/%{name}-sdn-kube-subnet-setup.sh
%{kube_plugin_path}/%{name}-ovs-subnet
%{_unitdir}/%{name}-node.service.d/%{name}-sdn-ovs.conf
%{_unitdir}/docker.service.d/docker-sdn-ovs.conf

%files -n tuned-profiles-openshift-node
%defattr(-,root,root,-)
%{_prefix}/lib/tuned/openshift-node-host
%{_prefix}/lib/tuned/openshift-node-guest
%{_mandir}/man7/tuned-profiles-openshift-node.7*

%post -n tuned-profiles-openshift-node
recommended=`/usr/sbin/tuned-adm recommend`
if [[ "${recommended}" =~ guest ]] ; then
  /usr/sbin/tuned-adm profile openshift-node-guest > /dev/null 2>&1
else
  /usr/sbin/tuned-adm profile openshift-node-host > /dev/null 2>&1
fi

%preun -n tuned-profiles-openshift-node
# reset the tuned profile to the recommended profile
# $1 = 0 when we're being removed > 0 during upgrades
if [ "$1" = 0 ]; then
  recommended=`/usr/sbin/tuned-adm recommend`
  /usr/sbin/tuned-adm profile $recommended > /dev/null 2>&1
fi

%files clients
%{_datadir}/%{name}/linux/oc
%{_datadir}/%{name}/macosx/oc
%{_datadir}/%{name}/windows/oc.exe

%files dockerregistry
%defattr(-,root,root,-)
%{_bindir}/dockerregistry

%files pod
%defattr(-,root,root,-)
%{_bindir}/pod

# ===
%files -n atomic-enterprise
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/atomic-enterprise
%{_bindir}/oc
%{_bindir}/oadm
%{_bindir}/kubectl
%{_sharedstatedir}/origin
%{_sysconfdir}/bash_completion.d/*
%dir %config(noreplace) %{_sysconfdir}/origin

%pre -n atomic-enterprise
# If /etc/openshift exists symlink it to /etc/origin
if [ -d "%{_sysconfdir}/openshift" ]; then
  ln -s %{_sysconfdir}/openshift %{_sysconfdir}/origin
fi

%files -n atomic-enterprise-master
%defattr(-,root,root,-)
%{_unitdir}/atomic-enterprise-master.service
%{_unitdir}/atomic-enterprise-allinone.service
%config(noreplace) %{_sysconfdir}/sysconfig/atomic-enterprise-master
%config(noreplace) %{_sysconfdir}/sysconfig/atomic-enterprise-allinone
%dir %config(noreplace) %{_sysconfdir}/origin/master
%dir %config(noreplace) %{_sysconfdir}/origin/allinone/
%dir %config(noreplace) %{_sysconfdir}/origin/allinone/master
%dir %config(noreplace) %{_sysconfdir}/origin/allinone/node
%ghost %config(noreplace) %{_sysconfdir}/sysconfig/%{name}-master
%ghost %config(noreplace) %{_sysconfdir}/origin/master
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
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/admin.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/admin.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/admin.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/ca.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/ca.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/ca.serial.txt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/etcd.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/etcd.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master-config.yaml
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master.etcd-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master.etcd-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master.kubelet-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master.kubelet-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/master.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-master.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-master.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-master.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-registry.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-registry.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-registry.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-router.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-router.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/openshift-router.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/policy.json
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/serviceaccounts.private.key
%ghost %config(noreplace) %{_sysconfdir}/origin/allinone/master/serviceaccounts.public.key

%post -n atomic-enterprise-master
%systemd_post %{basename:atomic-enterprise-master.service}
# Create all-in-one master configs
%{_bindir}/atomic-enterprise start master --write-config=%{_sysconfdir}/origin/allinone/master
# Create all-in-one node configs
%{_bindir}/oadm create-node-config --node-dir=%{_sysconfdir}/origin/allinone/node/ --node=localhost --hostnames=localhost,127.0.0.1 --node-client-certificate-authority=%{_sysconfdir}/origin/allinone/master/ca.crt --signer-cert=%{_sysconfdir}/origin/allinone/master/ca.crt --signer-key=%{_sysconfdir}/origin/allinone/master/ca.key --signer-serial=%{_sysconfdir}/origin/allinone/master/ca.serial.txt --certificate-authority=%{_sysconfdir}/origin/allinone/master/ca.crt

%preun -n atomic-enterprise-master
%systemd_preun %{basename:atomic-enterprise-master.service}
%systemd_preun %{basename:atomic-enterprise-allinone.service}

%postun -n atomic-enterprise-master
%systemd_postun

%files -n atomic-enterprise-node
%defattr(-,root,root,-)
%{_unitdir}/atomic-enterprise-node.service
%config(noreplace) %{_sysconfdir}/sysconfig/atomic-enterprise-node
%config(noreplace) %{_sysconfdir}/origin/node

%post -n atomic-enterprise-node
%systemd_post %{basename:atomic-enterprise-node.service}

%preun -n atomic-enterprise-node
%systemd_preun %{basename:atomic-enterprise-node.service}

%postun -n atomic-enterprise-node
%systemd_postun


%files -n atomic-enterprise-sdn-ovs
%defattr(-,root,root,-)
%{_bindir}/atomic-enterprise-sdn-kube-subnet-setup.sh
%{kube_plugin_path}/atomic-enterprise-ovs-subnet
%{_unitdir}/atomic-enterprise-node.service.d/atomic-enterprise-sdn-ovs.conf
%{_unitdir}/docker.service.d/docker-sdn-ovs.conf

%files -n tuned-profiles-atomic-enterprise-node
%defattr(-,root,root,-)
%{_prefix}/lib/tuned/atomic-enterprise-node-host
%{_prefix}/lib/tuned/atomic-enterprise-node-guest
%{_mandir}/man7/tuned-profiles-atomic-enterprise-node.7*

%post -n tuned-profiles-atomic-enterprise-node
recommended=`/usr/sbin/tuned-adm recommend`
if [[ "${recommended}" =~ guest ]] ; then
  /usr/sbin/tuned-adm profile atomic-enterprise-node-guest > /dev/null 2>&1
else
  /usr/sbin/tuned-adm profile atomic-enterprise-node-host > /dev/null 2>&1
fi

%preun -n tuned-profiles-atomic-enterprise-node
# reset the tuned profile to the recommended profile
# $1 = 0 when we're being removed > 0 during upgrades
if [ "$1" = 0 ]; then
  recommended=`/usr/sbin/tuned-adm recommend`
  /usr/sbin/tuned-adm profile $recommended > /dev/null 2>&1
fi

%files -n atomic-enterprise-clients
%{_datadir}/atomic-enterprise/linux/oc
%{_datadir}/atomic-enterprise/macosx/oc
%{_datadir}/atomic-enterprise/windows/oc.exe

%files -n atomic-enterprise-dockerregistry
%defattr(-,root,root,-)
%{_bindir}/dockerregistry

%files -n atomic-enterprise-pod
%defattr(-,root,root,-)
%{_bindir}/pod

# ===

%changelog
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

