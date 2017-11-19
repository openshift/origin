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
    for var_name in ["OS_GIT_COMMIT", "OS_GIT_VERSION", "OS_GIT_MAJOR", "OS_GIT_MINOR",
                     "OS_GIT_CATALOG_VERSION",
                     "ETCD_GIT_VERSION", "ETCD_GIT_COMMIT",
                     "KUBE_GIT_VERSION", "KUBE_GIT_COMMIT"]:
        git_vars[var_name] = run_command(
            "bash -c 'source ./hack/lib/init.sh; os::build::version::git_vars; echo ${}'".format(var_name)
        )
    # we hard-code this to a clean state as tito will have dirtied up the tree
    # but that will not have changed any of the source used for the product
    # release and we therefore don't want that reflected in the release version
    git_vars["OS_GIT_TREE_STATE"] = "clean"

    # To derive the correct tag for images in 3.7 and beyond, 
    # (1) For pre-release builds, with Release=0*, include Release field in oc version.
    # (2) For release candidates, do not include Release field in oc version since 
    #     registry.access cannot be populated with images which exactly match the RPM release.
    if "-0" in git_vars["OS_GIT_VERSION"]:
        # Only remove +metadata, but retain remaining Release field (3.7.0-0.1.1+git9898 => 3.7.0-0.1.1)
        git_vars["OS_GIT_VERSION"] = git_vars["OS_GIT_VERSION"].split('+')[0]
    else:
        # For release candidates, remove entire Release field (3.7.5-1 => 3.7.5)
        git_vars["OS_GIT_VERSION"] = git_vars["OS_GIT_VERSION"].split('-')[0]
    
    # Buildtime variables required to change origin to atomic-openshift
    git_vars["OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS"] = "rhel7"
    git_vars["OS_IMAGE_PREFIX"] = "registry.access.redhat.com/openshift3/ose"
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
