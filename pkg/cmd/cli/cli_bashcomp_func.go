package cli

const (
	bashCompletionFunc = `# call oc get $1,
__osc_parse_get()
{

    local template
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
    local osc_out
    if osc_out=$(oc get -o template --template="${template}" "$1" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${osc_out[*]}" -- "$cur" ) )
    fi
}

__osc_get_resource()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        return 1
    fi
    __osc_parse_get ${nouns[${#nouns[@]} -1]}
}

# $1 is the name of the pod we want to get the list of containers inside
__osc_get_containers()
{
    local template
    template="{{ range .spec.containers  }}{{ .name }} {{ end }}"
    __debug ${FUNCNAME} "nouns are ${nouns[@]}"

    local len="${#nouns[@]}"
    if [[ ${len} -ne 1 ]]; then
        return
    fi
    local last=${nouns[${len} -1]}
    local osc_out
    if osc_out=$(oc get -o template --template="${template}" pods "${last}" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${osc_out[*]}" -- "$cur" ) )
    fi
}

# Require both a pod and a container to be specified
__osc_require_pod_and_container()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        __osc_parse_get pods
        return 0
    fi;
    __osc_get_containers
    return 0
}

__custom_func() {
    case ${last_command} in
        osc_get | osc_describe | osc_delete)
	    __osc_get_resource
            return
            ;;
	osc_log)
	    __osc_require_pod_and_container
	    return
	    ;;
        *)
            ;;
    esac
}
`
)
