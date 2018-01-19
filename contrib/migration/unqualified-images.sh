#!/bin/bash

# List all unqualified images in all pods in all namespaces.

strindex() {
    local x="${1%%$2*}"		# find occurrence of $2 in $1
    [[ "$x" == "$1" ]] && echo -1 || echo "${#x}"
}

# Finds any-of chars in string. Returns 0 on success, otherwise 1.
strchr() {
    local str=$1
    local chars=$2
    for (( i=0; i<${#chars}; i++ )); do
	[[ $(strindex "$str" "${chars:$i:1}") -ge 0 ]] && return 0
    done
    return 1
}

split_image_at_domain() {
    local image=$1
    local index=$(strindex "$image" "/")

    if [[ "$index" == -1 ]] || (! strchr "${image:0:$index}" ".:" && [[ "${image:0:$index}" != "localhost" ]]); then
	echo ""
    else
	echo "${image:0:$index}"
    fi
}

has_domain() {
    local image=$1
    [[ -n $(split_image_at_domain "$image") ]]
}

die() {
    echo "$*" 1>&2
    exit 1
}

self_test() {
    strchr "foo/busybox" "Z"            && die "self-test 1 failed"

    strchr "foo/busybox" "/"            || die "self-test 2 failed"
    strchr "foo/busybox" "Zx"           || die "self-test 3 failed"

    has_domain "foo"			&& die "self-test 4 failed"
    has_domain "foo/busybox"		&& die "self-test 5 failed"
    has_domain "repo/foo/busybox"	&& die "self-test 6 failed"
    has_domain "a/b/c/busybox"          && die "self-test 7 failed"

    has_domain "localhost/busybox"	|| die "self-test 8 failed"
    has_domain "localhost:5000/busybox" || die "self-test 9 failed"
    has_domain "foo.com:5000/busybox"	|| die "self-test 10 failed"
    has_domain "docker.io/busybox"	|| die "self-test 11 failed"
    has_domain "a.b.c.io/busybox"	|| die "self-test 12 failed"
}

[[ -n ${SELF_TEST:-} ]] && self_test

template='
{{- range .items -}}
    {{- $metadata := .metadata -}}
    {{- $containers := .spec.containers -}}
    {{- $container_statuses := .status.containerStatuses -}}
    {{- if and $containers $container_statuses -}}
	{{- if eq (len $containers) (len $container_statuses) -}}
	    {{- range $n, $container := $containers -}}
		{{- printf "%s %s %s %s\n" $metadata.namespace $metadata.name $container.image (index $container_statuses $n).imageID -}}
	    {{- end -}}
	{{- end -}}
    {{- end -}}
{{- end -}}'

kubectl get pods --all-namespaces -o go-template="$template" | while read -r namespace pod image image_id; do
    has_domain "$image" || echo "$namespace $pod $image $image_id"
done
