package admin

const (
	BashCompletionFunc = `
__custom_func() {
    case ${last_command} in
        oadm_validate_master-config | oadm_validate_node-config)
            _filedir
            return
            ;;
        *)
            ;;
    esac
}
`
)
