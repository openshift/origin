#!/bin/bash
export PATH=${GOPATH%%:*}/bin:${PATH}
export GIT_VALIDATION=tests/tools/build/git-validation
if [ ! -x "$GIT_VALIDATION" ]; then
	echo git-validation is not installed.
	echo Try installing it with \"make install.tools\"
	exit 1
fi
if test "$TRAVIS" != true ; then
	#GITVALIDATE_EPOCH=":/git-validation epoch"
	GITVALIDATE_EPOCH="f835dce9c4fe435d0e7df797bacb0a8ee78e180a"
fi
exec "$GIT_VALIDATION" -q -run DCO,short-subject ${GITVALIDATE_EPOCH:+-range "${GITVALIDATE_EPOCH}""..${GITVALIDATE_TIP:-@}"} ${GITVALIDATE_FLAGS}
