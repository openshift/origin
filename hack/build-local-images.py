#!/usr/bin/env python

import sys
from shutil import copy, rmtree
import distutils.dir_util as dir_util
from subprocess import call
from tempfile import mkdtemp

from atexit import register
from os import getenv, mkdir, remove
from os.path import abspath, dirname, isdir, join

if len(sys.argv) > 1 and sys.argv[1] in ['-h', '--h', '-help', '--help']:
    print """Quickly re-build images depending on OpenShift Origin build artifacts.

This script re-builds OpenShift Origin images quickly. It is intended
to be used by developers for quick, iterative development of images
that depend on binaries, RPMs, or other artifacts that the Origin build
process generates. The script works by creating a temporary context
directory for a Docker build, adding a simple Dockerfile FROM the image
you wish to rebuild, ADDing in static files to overwrite, and building.

The script supports ADDing binaries from origin/_output/local/bin/linux/amd64/
and ADDing static files from the original context directories under the
origin/images/ directories.

Usage:
  [OS_DEBUG=true] [OS_IMAGE_PREFIX=prefix] build-local-images.py [IMAGE...]

  Specific images can be specified to be built with either the full name
  of the image (e.g. openshift3/ose-haproxy-router) or the name sans prefix
  (e.g. haproxy-router).

  The following environment variables are honored by this script:
   - $OS_IMAGE_PREFIX: one of [openshift/origin, docker.io/openshift/origin, openshift3/ose]
   - $OS_DEBUG: if set, debugging information will be printed

Examples:
  # build all images
  build-local-images.py

  # build only the f5-router image
  build-local-images.py f5-router

  # build with a different image prefix
  OS_IMAGE_PREFIX=openshift3/ose build-local-images.sh

Options:
  -h,--h, -help,--help: show this help-text
"""
    exit(2)

os_image_prefix = getenv("OS_IMAGE_PREFIX", "openshift/origin")
image_namespace, _, image_prefix = os_image_prefix.rpartition("/")

# "enable_default: True" can be added to skip the image build
# with no arguments
image_config = {
    "cli": {
        "directory": "cli",
        "binaries": {
            "oc": "/usr/bin/oc",
        },
        "files": {}
    },
    "control-plane": {
        "directory": "origin",
        "binaries": {
            "openshift": "/usr/bin/openshift",
            "oc": "/usr/bin/oc",
        },
        "files": {}
    },
    "hyperkube": {
        "directory": "hyperkube",
        "binaries": {
            "hyperkube": "/usr/bin/hyperkube",
        },
        "files": {}
    },
    "hypershift": {
        "directory": "hypershift",
        "binaries": {
            "hypershift": "/usr/bin/hypershift",
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
    "f5-router": {
        "directory": "router/f5",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {}
    },
    "nginx-router": {
        "directory": "router/nginx",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {
            ".": "/var/lib/nginx"
        }
    },
    "haproxy-router": {
        "directory": "router/haproxy",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {
            ".": "/var/lib/haproxy"
        }
    },
    "keepalived-ipfailover": {
        "directory": "ipfailover/keepalived",
        "binaries": {
            "openshift": "/usr/bin/openshift"
        },
        "files": {
            ".": "/var/lib/ipfailover/keepalived"
        }
    },
    "node": {
        "directory": "node",
        "binaries": {
            "openshift": "/usr/bin/openshift",
            "openshift-node-config": "/usr/bin/openshift-node-config",
            "hyperkube": "/usr/bin/hyperkube"
        },
        "files": {}
    },
    "template-service-broker": {
        "directory": "template-service-broker",
        "binaries": {
            "template-service-broker": "/usr/bin/template-service-broker"
        },
        "files": {}
    },
}


def image_rebuild_requested(image):
    """
    An image rebuild is requested if the
    user provides the image name or image
    suffix explicitly or does not provide
    any explicit requests.
    """
    implicitly_triggered = len(sys.argv) == 1 and image_config[image].get("enable_default", True)
    explicitly_triggered = len(sys.argv) > 1 and (image in sys.argv or full_name(image) in sys.argv)
    return implicitly_triggered or explicitly_triggered


def full_name(image):
    """
    The full name of the image will contain
    the image namespace as well as the pre-
    fix, if applicable.
    """
    return "{}/{}-{}".format(image_namespace, image_prefix, image)


def add_to_context(context_dir, source, destination, container_destination):
    """
    Add a file to the context directory
    and add an entry to the Dockerfile
    to place it in the container file-
    sytem at the correct destination.
    """
    debug("Adding file:\n\tfrom {}\n\tto {}\n\tincluding in container at {}".format(
        source,
        join(context_dir, destination),
        container_destination)
    )
    absolute_destination = abspath(join(context_dir, destination))
    if isdir(source):
        dir_util.copy_tree(source, absolute_destination)
    else:
        copy(source, absolute_destination)
    with open(join(context_dir, "Dockerfile"), "a") as dockerfile:
        dockerfile.write("ADD {} {}\n".format(destination, container_destination))


def debug(message):
    if getenv("OS_DEBUG"):
        print "[DEBUG] {}".format(message)


os_root = abspath(join(dirname(__file__), ".."))
os_image_path = join(os_root, "images")

context_dir = mkdtemp()
register(rmtree, context_dir)

debug("Created temporary context dir at {}".format(context_dir))
mkdir(join(context_dir, "bin"))
mkdir(join(context_dir, "src"))

build_occurred = False
for image in image_config:
    if not image_rebuild_requested(image):
        continue

    build_occurred = True
    print "[INFO] Building {}...".format(image)
    with open(join(context_dir, "Dockerfile"), "w+") as dockerfile:
        dockerfile.write("FROM {}\n".format(full_name(image)))

    binary_dir_args = ["_output", "local", "bin", "linux", "amd64"]
    config = image_config[image]
    for binary in config.get("binaries", []):
        if "vendor_dir" in config:
            os_bin_path = join(os_root, config.get("vendor_dir"), *binary_dir_args)
        else:
            os_bin_path = join(os_root, *binary_dir_args)

        add_to_context(
            context_dir,
            source=join(os_bin_path, binary),
            destination=join("bin", binary),
            container_destination=config["binaries"][binary]
        )

    mkdir(join(context_dir, "src", image))
    for file in config.get("files", []):
        add_to_context(
            context_dir,
            source=join(os_image_path, config["directory"], file),
            destination=join("src", image, file),
            container_destination=config["files"][file]
        )

    debug("Initiating Docker build with Dockerfile:\n{}".format(open(join(context_dir, "Dockerfile")).read()))
    call(["docker", "build", "-t", full_name(image), "."], cwd=context_dir)

    remove(join(context_dir, "Dockerfile"))
    rmtree(join(context_dir, "src", image))

if not build_occurred and len(sys.argv) > 1:
    print "[ERROR] The provided image names ({}) did not match any buildable images.".format(
        ", ".join(sys.argv[1:])
    )
    print "[ERROR] This script knows how to build:\n\t{}".format(
        "\n\t".join(map(full_name, image_config.keys()))
    )
    exit(1)
