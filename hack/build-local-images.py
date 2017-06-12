#!/usr/bin/env python

import sys
from shutil import copyfile, rmtree
from subprocess import call
from tempfile import mkdtemp

from os import getenv, mkdir, remove
from os.path import abspath, dirname, exists, join

os_image_prefix = getenv("OS_IMAGE_PREFIX", "openshift/origin")
image_namespace, image_prefix = os_image_prefix.split("/", 2)

image_config = {
    image_prefix: {
        "directory": "origin",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "deployer": {
        "directory": "deployer",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "recycler": {
        "directory": "recycler",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "docker-builder": {
        "directory": "builder/docker/docker-builder",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "sti-builder": {
        "directory": "builder/docker/sti-builder",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "f5-router": {
        "directory": "router/f5",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "node": {
        "directory": "node",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "openvswitch": {
        "directory": "openvswitch",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    }
}


def image_rebuild_requested(image):
    """
    An image rebuild is requested if the
    user provides the image name or image
    suffix explicitly or does not provide
    any explicit requests.
    """
    return len(sys.argv) == 1 or (
        len(sys.argv) > 1 and (
            image in sys.argv or
            full_name(image) in sys.argv
        )
    )


def full_name(image):
    """
    The full name of the image will contain
    the image namespace as well as the pre-
    fix, if applicable.
    """
    if image in ["node", "openvswitch", image_prefix]:
        return "{}/{}".format(image_namespace, image)

    return "{}/{}-{}".format(image_namespace, image_prefix, image)


def add_to_context(context_dir, source, destination, container_destination):
    """
    Add a file to the context directory
    and add an entry to the Dockerfile
    to place it in the container file-
    sytem at the correct destination.
    """
    debug("Adding file:\n\tfrom {}\n\tto {},\n\tincluding in container at {}".format(
    	source,
    	join(context_dir, destination),
		container_destination)
   	)
    absolute_destination = abspath(join(context_dir, destination))
    if not exists(absolute_destination):
        copyfile(source, absolute_destination)
    with open(join(context_dir, "Dockerfile"), "a") as dockerfile:
        dockerfile.write("ADD {} {}\n".format(destination, container_destination))


def debug(message):
    if getenv("OS_DEBUG"):
        print "[DEBUG] {}".format(message)


os_root = abspath(join(dirname(__file__), ".."))
os_bin_path = join(os_root, "_output", "local", "bin", "linux", "amd64")
os_image_path = join(os_root, "images")

context_dir = mkdtemp()
debug("Created temporary context dir at {}".format(context_dir))
mkdir(join(context_dir, "bin"))
mkdir(join(context_dir, "src"))

for image in image_config:
    if not image_rebuild_requested(image):
        continue

    print "[INFO] Building {}...".format(image)
    with open(join(context_dir, "Dockerfile"), "w+") as dockerfile:
        dockerfile.write("FROM {}\n".format(full_name(image)))

    config = image_config[image]
    for binary in config.get("binaries", []):
        add_to_context(
            context_dir,
            source=join(os_bin_path, binary),
            destination=join("bin", binary),
            container_destination=config["binaries"][binary]
        )

    for file in config.get("files", []):
        add_to_context(
            context_dir,
            source=join(os_image_path, config["directory"], file),
            destination=join("src", image, file),
            container_destination=config["files"][file]
        )

    debug("Initiating Docker build with Dockerfile:\n{}".format(open(join(context_dir, "Dockerfile")).read()))
    call(["docker", "build", "-t", image, "."], cwd=context_dir)

    remove(join(context_dir, "Dockerfile"))
    rmtree(join(context_dir, "src", image))

rmtree(context_dir)
