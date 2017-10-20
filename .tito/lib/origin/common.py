from tito.common import run_command, get_latest_commit


def inject_os_git_vars(spec_file):
    """
    Determine the OpenShift version variables as dictated by the Origin
    shell utilities and overwrite the specfile to reflect them. A line
    with the following syntax is expected in the specfile:

    %global os_git_vars

    This line will be overwritten to add the git tree state, the full
    "git version", the last source commit in the release, and the major
    and minor versions of the current product release.
    """
    os_git_vars = get_os_git_vars()
    for var_name in os_git_vars:
        print("{}::{}".format(var_name, os_git_vars[var_name]))

    update_os_git_vars = \
        "sed -i 's|^%global os_git_vars .*$|%global os_git_vars {}|' {}".format(
            " ".join(["{}={}".format(key, value) for key, value in os_git_vars.items()]),
            spec_file
        )
    run_command(update_os_git_vars)


def get_os_git_vars():
    """
    Determine the OpenShift version variables as dictated by the Origin
    shell utilities. The git tree state is spoofed.
    """
    git_vars = {}
    for var in ["COMMIT", "VERSION", "MAJOR", "MINOR", "CATALOG_VERSION"]:
        var_name = "OS_GIT_{}".format(var)
        git_vars[var_name] = run_command(
            "bash -c 'source ./hack/lib/init.sh; os::build::version::openshift_vars; echo ${}'".format(var_name)
        )

    # we hard-code this to a clean state as tito will have dirtied up the tree
    # but that will not have changed any of the source used for the product
    # release and we therefore don't want that reflected in the release version
    git_vars["OS_GIT_TREE_STATE"] = "clean"
    git_vars["OS_GIT_VERSION"] = git_vars["OS_GIT_VERSION"].replace("-dirty", "")
    return git_vars


def update_global_hash(spec_file):
    """
    Update the specfile to reflect the latest commit. A line
    with the following syntax is expected in the specfile:

    %global commit

    This line will be overwritten to add the git commit.
    """
    git_hash = get_latest_commit()
    update_commit = \
        "sed -i 's/^%global commit .*$/%global commit {0}/' {1}".format(
            git_hash,
            spec_file
        )
    run_command(update_commit)
