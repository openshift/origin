Name:		golang
Version:	v1.7.3
Release:	1%{?dist}
Summary:	Go install from source
Group:		Fake
License:	BSD
BuildRoot:	%(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)
Provides:	golang
BuildArch:	noarch
%description
%{summary}
%prep
%setup -c -T
%build
%install
%files
%defattr(-,root,root,-)
%changelog
* Thu Oct 27 2016 Clayton Coleman <ccoleman@redhat.com> 0.0-1
- Initial change
EOF