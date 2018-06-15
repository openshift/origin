#!/bin/bash

set -iuo pipefail
IFS=$'\n\t'

readonly AUDITOR_ROLE='system:image-auditor'
readonly CLUSTER_READER_ROLE='cluster-reader'
readonly REGISTRY_VIEWER_ROLE='registry-viewer'

readonly USAGE="Usage: $(basename ${BASH_SOURCE[0]}) [OPTIONS]

It fetches manifests of images from OpenShift integrated registry forcing it to
store the manifests on the local storage and remove them from etcd. This will
work as long as PR https://github.com/openshift/origin/pull/11925 is compiled
in the regisry binary.

The script doesn't make any changes unless -a flag is given.

See bug https://bugzilla.redhat.com/show_bug.cgi?id=1378180 for more details.

Options:
  -h                 Print this message and exit.
  -a                 Apply. Do the changes.
  -r <registry_url>  Specify registry url to connect to.
                     If not given, script will try to determine from
                     cluster status.
  -s                 Registry is secured.
  -c <cacert>        CA certificate for use with registry if the registry is
                     secured.
  -t <token>         Token to use to query the registry.
                     Its user or service account must be able to
                     get imagestreams/layers in all the namespaces.
                     If not given, token of current user will be used.
                     Run this command as cluster admin to give particular
                     user enough rights to query the images:

                       \$ oc adm policy add-cluster-role-to-user ${REGISTRY_VIEWER_ROLE} <user>
                       \$ oc adm policy add-cluster-role-to-user ${CLUSTER_READER_ROLE} <user>

  -f                 Force migration of externally managed images (those imported
                     from remote registries). A migration attempt is done by
                     default. If it fails (manifest cannot be stored on
                     registry's storage because of missing dependencies), the
                     manifest will be kept in image object (and in etcd). This
                     option causes a removal of the manifest in any way. It
                     will still be available only on the remote registry. If
                     pullthrough feature is enabled, registry will serve such
                     manifests from the external locations.

                     For this to work, the user must be an image auditor:

                       \$ oc adm policy add-cluster-role-to-user ${AUDITOR_ROLE} <user>
                       \$ oc adm policy add-cluster-role-to-user ${CLUSTER_READER_ROLE} <user>
"

registry_address=""
secured=0
cacert=""
token=""
force_removal_of_manifests=0
dry_run=1

function get_docker_registry_url() {
    local service_ip ports
    local tmpl=$'{{.spec.clusterIP}}%{{range $i, $port := .spec.ports}}{{$port.targetPort}}@{{$port.port}},{{end}}'
    IFS=% read -r service_ip ports <<<"$(oc get -o go-template="${tmpl}" -n default svc/docker-registry)"
    if [[ -z "${service_ip:-}" ]]; then
        echo "failed to get service ip of svc/docker-registry" >&2
        return 1
    fi

    # first, try to get a port with targetPort == 5000
    local port="$(echo "${ports}" | tr ',' '\n' | sed -n 's/^5000@\([0-9]\+\)/\1/p')"
    if [[ -z "${port:-}" ]]; then
        # if no such port, get the first port
        port=$(echo "${ports}" | sed 's/^[^@]\+@\([^,]\+\).*/\1/')
    fi
    if [[ -z "${port:-}" ]]; then
        echo "failed to get port of svc/docker-registry" >&2
        return 1
    fi
    echo "${service_ip}:${port}"
}

function check_permissions() {
    declare -a verbs
    local authorized=1 verb resource
    for resource in images projects; do
        if [[ "${resource}" == "images" ]]; then
            verbs=( get list update )
        else
            verbs=( get list )
        fi
        for verb in "${verbs[@]}"; do
            if ! oc policy can-i -q --all-namespaces "${verb}" "${resource}"; then
                echo "The user isn't authorized to ${verb} ${resource}!" >&2
                authorized=0
            fi
        done
    done
    if [[ "${authorized}" == 0 ]]; then
        echo "Ask your admin to give you permissions to work with images, e.g.:" >&2
        echo "  oc adm policy add-cluster-role-to-user ${AUDITOR_ROLE} $(oc whoami)" >&2
        echo "  oc adm policy add-cluster-role-to-user ${CLUSTER_READER_ROLE} $(oc whoami)" >&2
        return 1
    fi

    if ! oc policy can-i -q get --all-namespaces imagestreams/layers --token="${token}"; then
        echo "The registry user isn't authorized to get imagestreams/layers!" >&2
        echo "  oc adm policy add-cluster-role-to-user registry-viewer <user>" >&2
        echo "  oc adm policy add-cluster-role-to-user ${CLUSTER_READER_ROLE} <user>" >&2
        return 1
    fi
}

function manifest_removed() {
    local ref="$1"
    local tmpl_manifest_removed=$'{{if .dockerImageManifest}}present{{else}}removed{{end}},'
    tmpl_manifest_removed+=$'{{if .metadata.annotations}}'
    tmpl_manifest_removed+=$'{{if index .metadata.annotations "openshift.io/image.managed"}}'
    tmpl_manifest_removed+=$'managed{{else}}external{{end}}{{else}}external{{end}}\n'

    oc get image -o go-template="${tmpl_manifest_removed}" "${ref}"
}

function force_manifest_removal() {
    local reference="$1"

    echo "Forcibly removing manifest from external image ${reference}".
    if ! oc patch image "${reference}" -p 'dockerImageManifest: ""'; then
        echo "Failed to remove manifest from image ${reference}!" >&2
    fi
    # config can be safely removed as well
    oc patch image "${reference}" -p 'dockerImageConfig: ""' >/dev/null 2>&1 || :
}

function migrate() {
    local tmpl_istags=$'{{range $isi, $is := .items}}{{range $tagname, $tag := $is.status.tags}}'
    tmpl_istags+=$'{{range $i, $item := $tag.items}}{{$is.metadata.name}}@{{$item.image}}\n'
    tmpl_istags+=$'{{end}}{{end}}{{end}}'

    declare -A processed_images
    local total=0
    local to_migrate=0
    local proj isimage isname reference removed managed

    for proj in $(oc get -o jsonpath=$'{range .items[*]}{.metadata.name}\n{end}' project | sort -u); do
        echo "Processing project '${proj}'"
        for isimage in $(oc get -n "${proj}" -o go-template="${tmpl_istags}" is 2>/dev/null); do
            IFS='@' read -r isname reference <<<"${isimage}"
            [[ -n "${processed_images[${reference}]+set}" ]] && continue
            processed_images[${reference}]=1
            total="$((${total} + 1))"

            IFS=',' read -r removed managed <<<"$(manifest_removed "${reference}")"
            [[ "${removed}" == 'removed' || -z "${removed}" ]] && continue
            to_migrate="$((${to_migrate} + 1))"

            if [[ "${dry_run}" = 1 ]]; then
                echo "Would migrate ${managed} isimage '${proj}/${isname}@${reference}'"
                continue
            fi

            echo "Migrating ${managed} isimage '${proj}/${isname}@${reference}'"
            if ! curl --fail -k ${curlargs[@]-} -u "unused:${token}" -i -s \
                    "${url}/v2/${proj}/${isname}/manifests/${reference}" | \
                    sed -e '/^[[:space:]]*$/,$d' | grep '^HTTP'; then
                echo "Failed to fetch '${url}/v2/${proj}/${isname}/manifests/${reference}'!"
            fi

            if [[ "${force_removal_of_manifests}" == 1 && "${managed}" == 'external' ]]; then
                force_manifest_removal "${reference}"
            fi
        done
    done

    if [[ "${dry_run}" = 1 ]]; then
        printf '\nWould migrate %d out of %d images.\n' "${to_migrate}" "${total}"
    fi
}

while getopts 'hr:sc:t:fa' opt; do
    case "${opt}" in
        h)
            echo "${USAGE}";
            exit 0
            ;;
        a)
            dry_run=0
            ;;
        s)
            secured=1
            ;;
        c)
            cacert="${OPTARG}"
            ;;
        r)
            registry_address="${OPTARG}"
            ;;
        t)
            token="${OPTARG}"
            ;;
        f)
            force_removal_of_manifests=1
            ;;
        *)
            echo "${USAGE}" >&2
            exit 1
            ;;
    esac
done

if [[ -z "${registry_address:-}" ]]; then
    registry_address="$(get_docker_registry_url)"
fi

case "${registry_address},${secured}" in
    https://*)
        secured=1
        url="${registry_address}"
        ;;
    http://*)
        secured=0
        url="${registry_address}"
        ;;
    *,1)
        url="https://${registry_address}"
        ;;
    *,0)
        url="http://${registry_address}"
        ;;
esac

curlargs=()
if [[ -n "${cacert}" ]]; then
    curlargs+=( "--cacert" "${cacert}" )
fi

if [[ -z "${token:-}" ]]; then
    token="$(oc whoami -t)"
fi

if [[ -z "${token:-}" ]]; then
    echo "Please, provide a token of user authorized to get imagestreams/layers in all namespaces." >&2
    exit 1
fi

if ! curl --fail ${curlargs[@]-} --max-time 15 "${url}/healthz"; then
    echo "Please, provide endpoint of integrated docker registry." >&2
    exit 1
fi

check_permissions || exit 1
migrate
