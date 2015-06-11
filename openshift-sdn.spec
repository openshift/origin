#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global osdn_gopath _output/local/go
%global kube_plugin_path /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet
%global gopkg_base  github.com/openshift
%global import_path %{gopkg_base}/openshift-sdn
%global commit      2d06ba8340dc3e6543a762294f97935220f52cc0

Name:           openshift-sdn
Version:        0.4
Release:        1%{?dist}
Summary:        SDN solutions for OpenShift
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz

BuildRequires:  systemd
BuildRequires:  golang >= 1.2-7


%description
%{summary}

%package master
Summary:          OpenShift SDN Master
Requires:         openshift-sdn = %{version}-%{release}
Requires:         openshift-master
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd

%description master
%{summary}

%package node
Summary:          OpenShift SDN Node
Requires:         openshift-sdn = %{version}-%{release}
Requires:         openshift-node
Requires:         docker-io >= 1.3.2
Requires:         openvswitch >= 2.3.1
Requires:         bridge-utils
Requires:         ethtool
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd

%description node
%{summary}

%prep
%setup -q -n %{name}-%{commit}

%build

# Set up build environment similar to 'hack/build.sh' script
mkdir -p %{osdn_gopath}/src/%{gopkg_base}
mkdir -p %{osdn_gopath}/bin
ln -s $(pwd) $(pwd)/%{osdn_gopath}/src/%{import_path}

export GOPATH=$(pwd)/%{osdn_gopath}:$(pwd)/Godeps/_workspace:%{buildroot}%{gopath}:%{gopath}

# Default to building all of the components
go build %{import_path}
cp $(pwd)/ovssubnet/bin/openshift-sdn-simple-setup-node.sh $(pwd)/openshift-sdn-simple-setup-node.sh
cp $(pwd)/ovssubnet/bin/openshift-sdn-kube-subnet-setup.sh $(pwd)/openshift-sdn-kube-subnet-setup.sh
cp $(pwd)/ovssubnet/bin/openshift-ovs-subnet $(pwd)/openshift-ovs-subnet

%install

install -d %{buildroot}%{_bindir}
for bin in openshift-sdn openshift-sdn-simple-setup-node.sh openshift-sdn-kube-subnet-setup.sh
do
  install -p -m 755 ${bin} %{buildroot}%{_bindir}/${bin}
done


mkdir -p %{buildroot}%{kube_plugin_path}
install -p -m 755 openshift-ovs-subnet %{buildroot}%{kube_plugin_path}/openshift-ovs-subnet

install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-sdn-master.service
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-sdn-node.service

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 rel-eng/openshift-sdn-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-sdn-master
install -m 0644 rel-eng/openshift-sdn-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-sdn-node

install -d -m 0755 %{buildroot}%{_prefix}/lib/systemd/system/docker.service.d
install -p -m 0644 rel-eng/docker-sdn-ovs.conf %{buildroot}%{_prefix}/lib/systemd/system/docker.service.d/

%files
%defattr(-,root,root,-)
# TODO - add LICENSE: %doc README.md LICENSE
%doc README.md
%{_bindir}/openshift-sdn
%{_bindir}/openshift-sdn-simple-setup-node.sh
%{_bindir}/openshift-sdn-kube-subnet-setup.sh
%{kube_plugin_path}/openshift-ovs-subnet

%files master
%defattr(-,root,root,-)
%{_unitdir}/openshift-sdn-master.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-sdn-master

%post master
%systemd_post %{basename:openshift-sdn-master.service}

%preun master
%systemd_preun %{basename:openshift-sdn-master.service}

%postun master
%systemd_postun


%files node
%defattr(-,root,root,-)
%{_unitdir}/openshift-sdn-node.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-sdn-node
%{_prefix}/lib/systemd/system/docker.service.d/

%post node
systemctl daemon-reload
%systemd_post %{basename:openshift-sdn-node.service}

%preun node
%systemd_preun %{basename:openshift-sdn-node.service}

%postun node
%systemd_postun

%changelog
* Fri Jan 30 2015 dobbymoodge <jolamb@redhat.com> 0.4-1
- fix Source0 line in specfile (jolamb@redhat.com)

* Fri Jan 30 2015 dobbymoodge <jolamb@redhat.com> 0.3-1
- Add DOCKER_OPTIONS to sysconfig files (jolamb@redhat.com)
- Automatic commit of package [openshift-sdn] release [0.2-1].
  (sdodson@redhat.com)
- Anything that may be expanded to multiple argv should not be enclosed in
  braces (sdodson@redhat.com)
- EnvironmentFile values should not be quoted (sdodson@redhat.com)
- Enclose environment variables in curly braces (sdodson@redhat.com)
- Fixup requires (sdodson@redhat.com)
- Remove erroneous openshift dependency (jolamb@redhat.com)
- Master/Node subpackages, systemd unit files (jolamb@redhat.com)
- Added missing Requires: (jolamb@redhat.com)
- openshift-sdn-simple-setup-node.sh installed but unpackaged
  (jolamb@redhat.com)
- Minor fixups - Source0 URL, Version, install (jolamb@redhat.com)
- fixed up %%setup line (jolamb@redhat.com)
- Initialized to use tito, added specfile (jolamb@redhat.com)
- Document DOCKER_OPTIONS variable (jolamb@redhat.com)
- Add helpful comment at start of docker sysconfig (jolamb@redhat.com)
- Use $DOCKER_OPTIONS env var for docker settings if present
  (jolamb@redhat.com)

* Fri Jan 30 2015 Scott Dodson <sdodson@redhat.com> 0.2-1
- new package built with tito

