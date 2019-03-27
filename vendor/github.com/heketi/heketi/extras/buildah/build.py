#!/usr/bin/env python3
#
# Build heketi containers using the 'buildah' tool either from source
# or from existing compiled binaries.
#

import argparse
import os
import shutil
import subprocess
import tempfile

USE_SUDO = False

# We build on fedora 27 because there are (were) issues
# with glide on fedora 28.
BUILD_IMAGE_BASE = 'fedora:27'

DEPLOY_IMAGE_BASE = 'fedora:latest'

pjoin = os.path.join


def buildah(*args, redirect=False):
    """A lightweight wrapper around buildah command."""
    cmd = []
    if USE_SUDO and os.getuid() != 0:
        cmd.append("sudo")
    cmd.append("buildah")
    cmd.extend(args)
    if redirect:
        return subprocess.check_output(cmd).strip().decode('utf8')
    else:
        subprocess.check_call(cmd)


def buildah_from(src):
    return buildah('from', src, redirect=True)


def build_binaries(outdir, build_image_base):
    pkgs = 'glide golang git make mercurial'
    hdir = '/build/src/github.com/heketi/heketi'
    heketi_branch = 'master'
    heketi_url = 'https://github.com/heketi/heketi.git'

    bc = buildah_from(build_image_base)
    buildah('run', bc, 'dnf', '-y', 'install', *pkgs.split())
    buildah('run', bc, 'mkdir', '-p', os.path.dirname(hdir))
    buildah('run', bc, 'git', '-C', os.path.dirname(hdir),
            'clone', '-b', heketi_branch, heketi_url)
    buildah('config',
            '--workingdir', hdir,
            '--env', 'GOPATH=/build',
            bc)
    buildah('run', bc, 'make')

    mnt = buildah('mount', bc, redirect=True)
    heketi_dir = pjoin(mnt, hdir.strip('/'))
    heketi_bin = pjoin(heketi_dir, 'heketi')
    heketi_cli_bin = pjoin(heketi_dir, 'client/cli/go/heketi-cli')
    shutil.copy(heketi_bin, outdir)
    shutil.copy(heketi_cli_bin, outdir)
    buildah('umount', bc)

    buildah('rm', bc)


def build_container(sources, deploy_image_base):
    hc = buildah_from(deploy_image_base)
    for src, dest in sources.items():
        buildah('copy', hc, src, dest)
    buildah('run', hc, 'mkdir', '/var/lib/heketi')
    buildah('config',
            '--volume', '/etc/heketi',
            '--volume', '/var/lib/heketi',
            '--port', '8080',
            '--entrypoint', '/usr/bin/heketi-start.sh',
            hc)
    buildah('commit', '--format', 'docker', hc, 'heketi:local')
    buildah('rm', hc)


def main():
    p = argparse.ArgumentParser()
    p.add_argument(
        '--bin-dir',
        help='A path with pre-built heketi and heketi-cli binaries')
    p.add_argument(
        '--extras-dir',
        default='extras/docker/fromsource',
        help='A path with container service scripts')
    p.add_argument(
        '--build-image-base', default=BUILD_IMAGE_BASE,
        help='Base image used for build container')
    p.add_argument(
        '--deploy-image-base', default=DEPLOY_IMAGE_BASE,
        help='Base image used for server container')
    cli = p.parse_args()

    sources = {}
    tdir = None
    if cli.bin_dir:
        sources[pjoin(cli.bin_dir, 'heketi')] = '/usr/bin/heketi'
        sources[pjoin(cli.bin_dir, 'heketi-cli')] = '/usr/bin/heketi-cli'
    else:
        print ('Switch --bin-dir not provided, building heketi...')
        tdir = tempfile.mkdtemp()
        build_binaries(tdir, cli.build_image_base)
        sources[pjoin(tdir, 'heketi')] = '/usr/bin/heketi'
        sources[pjoin(tdir, 'heketi-cli')] = '/usr/bin/heketi-cli'

    # these don't need to be built they just sit in the repo
    sources[pjoin(cli.extras_dir, 'heketi.json')] = '/etc/heketi/heketi.json'
    sources[pjoin(cli.extras_dir, 'heketi-start.sh')] = (
        '/usr/bin/heketi-start.sh')

    build_container(sources, cli.deploy_image_base)

    if tdir:
        shutil.rmtree(tdir)


if __name__ == '__main__':
    main()
