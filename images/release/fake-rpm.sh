#!/bin/sh
tmpdir=$(mktemp -d -t rpmspecXXXXXX)
if [[ -n "${1-}" ]]; then
  cd $1
fi
rm *.rpm
HOME="${tmpdir}" rpmbuild -bb --define "_topdir ${tmpdir}/rpmbuild" golang.spec
cp "${tmpdir}/rpmbuild/RPMS/noarch/"*.rpm .
rm -rf "${tmpdir}"
