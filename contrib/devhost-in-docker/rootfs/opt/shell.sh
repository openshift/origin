#!/bin/bash -efu

. /opt/env.sh

[ -z "${CURL_CA_BUNDLE-}" ] || printf 'INFO: CURL_CA_BUNDLE exported\n'
[ -z "${KUBECONFIG-}"     ] || printf 'INFO: KUBECONFIG exported\n'

export PS1='[\u@docker \W]\$ '
/bin/bash
