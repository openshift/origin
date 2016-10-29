#!/bin/sh
tmpdir=$(mktemp -d -t rpmspecXXXXXX)
HOME="${tmpdir}" rpmbuild -bb --define "_topdir ${tmpdir}/rpmbuild" golang.spec