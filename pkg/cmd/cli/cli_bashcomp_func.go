package cli

const (
	bashCompletionFunc = `# call oc get $1,
__oc_parse_get()
{

    local template
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
    local oc_out
    if oc_out=$(oc get -o template --template="${template}" "$1" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${oc_out[*]}" -- "$cur" ) )
    fi
}

__oc_get_resource()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        return 1
    fi
    __oc_parse_get ${nouns[${#nouns[@]} -1]}
}

# $1 is the name of the pod we want to get the list of containers inside
__oc_get_containers()
{
    local template
    template="{{ range .spec.containers  }}{{ .name }} {{ end }}"
    __debug ${FUNCNAME} "nouns are ${nouns[@]}"

    local len="${#nouns[@]}"
    if [[ ${len} -ne 1 ]]; then
        return
    fi
    local last=${nouns[${len} -1]}
    local oc_out
    if oc_out=$(oc get -o template --template="${template}" pods "${last}" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${oc_out[*]}" -- "$cur" ) )
    fi
}

# Require both a pod and a container to be specified
__oc_require_pod_and_container()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        __oc_parse_get pods
        return 0
    fi;
    __oc_get_containers
    return 0
}

__custom_func() {
    case ${last_command} in
        oc_get | oc_describe | oc_delete)
            __oc_get_resource
            return
            ;;
        oc_log)
            __oc_require_pod_and_container
            return
            ;;
        oc_secrets_new)
            # Complete args other than the first as filenames
            if [[ ${#nouns[@]} -gt 0 ]]; then
                _filedir
            fi;
            return
            ;;
        *)
            ;;
    esac
}
`
)
