#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%{!?commit:
%global commit 21fb40637c4e3507cca1fcab6c4d56b06950a149
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# OpenShift specific ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 0 -X github.com/openshift/origin/pkg/version.minorFromGit 2+ -X github.com/openshift/origin/pkg/version.versionFromGit v0.2.2-134-gc9e7c25aaf0e61-dirty -X github.com/openshift/origin/pkg/version.commitFromGit c9e7c25 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit 72ad4f1 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion v0.10.0-46-g72ad4f1
}
# String used for --images flag
# If you're setting docker_registry make sure it ends in a trailing /
%if "%{dist}" == ".el7ose"
  %global docker_registry registry.access.redhat.com/
  %global docker_namespace openshift3_beta
  %global docker_prefix ose
%else
  %global docker_namespace openshift
  %global docker_prefix origin
%endif
%global docker_images %{?docker_registry}%{docker_namespace}/%{docker_prefix}-${component}:${version}

Name:           openshift
Version:        0.2.2
#Release:        1git%{shortcommit}%{?dist}
Release:        4%{?dist}
Summary:        Open Source Platform as a Service by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz

BuildRequires:  systemd
BuildRequires:  golang >= 1.2-7


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
Requires:       docker-io >= 1.3.2
Requires:       tuned-profiles-openshift-node
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description node
%{summary}

%package -n tuned-profiles-openshift-node
Summary:        Tuned profiles for OpenShift Node hosts
Requires:       tuned >= 2.3
Requires:       %{name} = %{version}-%{release}

%description -n tuned-profiles-openshift-node
%{summary}

%package -n osc-macosx-amd64
Summary:      Openshift Client Packages for Mac OSX
BuildRequires: golang-pkg-darwin-amd64
%description -n osc-macosx-amd64
%{summary}

%package -n osc-windows-386
Summary:      OpenShift Client Packages for Windows
BuildRequires: golang-pkg-windows-386
%description -n osc-windows-386
%{summary}

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
# Default to building all of the components
for OS in linux darwin windows
do
    export GOOS=${OS}
    for cmd in openshift
    do
        if [ $GOOS == 'windows' ]
        then
            export GOARCH='386'
        else
            export GOARCH='amd64'
        fi
        go install -ldflags "%{ldflags}" %{import_path}/cmd/${cmd}
    done
done
# set the IMAGES
sed -i 's|IMAGES=.*|IMAGES=%{docker_images}|' rel-eng/openshift-{master,node}.sysconfig

%install

install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_datadir}/%{name}/macosx
install -d %{buildroot}%{_datadir}/%{name}/windows

for bin in openshift
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 _build/bin/${bin} %{buildroot}%{_bindir}/${bin}
  install -p -m 755 _build/bin/darwin_amd64/${bin} %{buildroot}%{_datadir}/%{name}/macosx/osc
  install -p -m 755 _build/bin/windows_386/${bin}.exe %{buildroot}%{_datadir}/%{name}/windows/osc.exe
done

install -d -m 0755 %{buildroot}/etc/%{name}
install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-master.service
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-node.service

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 rel-eng/openshift-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-master
install -m 0644 rel-eng/openshift-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-node

mkdir -p %{buildroot}%{_sharedstatedir}/%{name}

ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/osc

install -d -m 0755 %{buildroot}%{_prefix}/lib/tuned/openshift-node-{guest,host}
install -m 0644 tuned/openshift-node-guest/tuned.conf %{buildroot}%{_prefix}/lib/tuned/openshift-node-guest/
install -m 0644 tuned/openshift-node-host/tuned.conf %{buildroot}%{_prefix}/lib/tuned/openshift-node-host/
install -d -m 0755 %{buildroot}%{_mandir}/man7
install -m 0644 tuned/man/tuned-profiles-openshift-node.7 %{buildroot}%{_mandir}/man7/tuned-profiles-openshift-node.7


%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/osc
%{_sharedstatedir}/%{name}
/etc/%{name}

%files master
%defattr(-,root,root,-)
%{_unitdir}/openshift-master.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-master

%post master
%systemd_post %{basename:openshift-master.service}

%preun master
%systemd_preun %{basename:openshift-master.service}

%postun master
%systemd_postun


%files node
%defattr(-,root,root,-)
%{_unitdir}/openshift-node.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-node

%post node
%systemd_post %{basename:openshift-node.service}

%preun node
%systemd_preun %{basename:openshift-node.service}

%postun node
%systemd_postun

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

%files -n osc-macosx-amd64
%{_datadir}/%{name}/macosx/osc

%files -n osc-windows-386
%{_datadir}/%{name}/windows/osc.exe


%changelog
* Fri Feb 06 2015 Scott Dodson <sdodson@redhat.com>
- new package built with tito

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

* Thu Aug 14 2014 Adam Miller <admiller@redhat.com> - 0-0.0.1.gitc3839b8
- First package
