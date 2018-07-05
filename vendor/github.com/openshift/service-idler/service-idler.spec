#
# This is a template package spec that will support Go builds following the OpenShift conventions.
# It expects a set of standard env vars that define the Git version being built and can also handle
# multi-architecture Linux builds. It has stubs for cross building.

#debuginfo not supported with Go
%global debug_package %{nil}

# modifying the Go binaries breaks the DWARF debugging
%global __os_install_post %{_rpmconfigdir}/brp-compress

# %commit and %os_git_vars are intended to be set by tito custom builders provided
# in the .tito/lib directory. The values in this spec file will not be kept up to date.
%{!?commit: %global commit HEAD }
%global shortcommit %(c=%{commit}; echo ${c:0:7})

%global golang_version 1.9.1
%{!?version: %global version 0.0.1}
%{!?release: %global release 1}
%global package_name origin-service-idler
%global product_name OpenShift Service Idler
%global import_path github.com/openshift/service-idler

Name:           %{package_name}
Version:        %{version}
Release:        %{release}%{?dist}
Summary:        The service idler is a custom resource and controller that can be used to build automated idling and unidling solutions for Kubernetes.
License:        ASL 2.0
URL:            https://%{import_path}

Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz
BuildRequires:  golang >= %{golang_version}

%description
The service idler defines an Idler custom resource and corresponding controller that knows how to scale a group of scalable Kubernetes resources down to zero, and then restore them all back to their previous scales, keeping track of when all of them are available for use.  It does not perform automatic idling and unidling -- instead, it can be used as a building block, so that the automated idling and unidling components simply need to flip a field in an Idler to trigger idling and unidling.

%prep
%setup -q

%build
# need to set up a GOPATH so that go doesn't complain
mkdir -p gopath/src/%{import_path}
rmdir gopath/src/%{import_path}
ln -s $(pwd) gopath/src/%{import_path}
export GOPATH=$(pwd)/gopath

# actually build
go build -o service-idler %{import_path}/cmd/service-idler

%install
install -d %{buildroot}%{_bindir}

echo "+++ INSTALLING service-idler"
install -p -m 755 service-idler %{buildroot}%{_bindir}/service-idler

%files
%doc README.md
%license LICENSE

%{_bindir}/service-idler

%changelog
* Mon Nov 06 2017 Anonymous <anon@nowhere.com> 0.0.1
- Initial example of spec.
