#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%global commit      d3f40eafae8ae7bbca61981e33f384375307fafa
%global shortcommit %(c=%{commit}; echo ${c:0:7})

Name:           origin
Version:        0.2
#Release:        1git%{shortcommit}%{?dist}
Release:        2%{?dist}
Summary:        Open Source Platform as a Service by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz

# FIXME - Need to add a -devel subpackage to etcd that provides the golang
#         libraries/packages, but this will work for now.
BuildRequires:  systemd
BuildRequires:  golang >= 1.2-7

Requires:       /usr/bin/docker

%description
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
for cmd in openshift
do
    #go build %{import_path}/cmd/${cmd}
    go build -ldflags \
        "-X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit 
            %{shortcommit}
        -X github.com/openshift/origin/pkg/version.commitFromGit 
            %{shortcommit}" %{import_path}/cmd/${cmd}
done

%install

install -d %{buildroot}%{_bindir}
for bin in openshift
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 ${bin} %{buildroot}%{_bindir}/${bin}
done

install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift.service

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 rel-eng/openshift.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift

mkdir -p %{buildroot}/var/log/%{name}

ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/osc

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%dir /var/log/%{name}
%{_bindir}/openshift
%{_bindir}/osc
%{_unitdir}/*.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift

%post
%systemd_post %{basename:openshift.service}

%preun
%systemd_preun %{basename:openshift.service}

%postun
%systemd_postun

%changelog
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
