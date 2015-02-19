package templates

const AssembleScript = `
#!/bin/bash -e
#
# STI assemble script for the '{{.ImageName}}' image.
# The 'assemble' script build your application source.
#
# For more informations see the documentation:
#	https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md
#

# The $HOME variable points to the application root
HOME=${HOME-"/app-root"}

# The application sources are mounted during the STI build into /tmp/src
APP_SRC_DIR="/tmp/src"

# The directory where the sources should be installed
APP_RUNTIME_DIR="${HOME}/src"

# Display assemble script usage
#
if [ "$1" = "-h" ]; then
	# If the '{{.ImageName}}' assemble script is executed with '-h' flag,
	# print the usage.
	exit 0
fi

# Restore artifacts from the previous build (if exists).
#
if [ "$(ls /tmp/artifacts/ 2>/dev/null)" ]; then
	echo "Restoring build artifacts"
	mv /tmp/artifacts/* $HOME/.
fi

echo "---> Installing application source"
mkdir -p ${APP_RUNTIME_DIR}
cp -Rf ${APP_SRC_DIR}/* ${APP_RUNTIME_DIR}/

pushd "$APP_RUNTIME_DIR/${APP_ROOT}" >/dev/null
echo "---> Building application from source"
# TODO: Add build steps for your application
popd >/dev/null
`

const RunScript = `
#!/bin/bash -e
#
# STI run script for the '{{.ImageName}}' image.
# The run script execute the server that runs your application.
#
# For more informations see the documentation:
#	https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md
#
APP_ROOT_DIR="${HOME}/src/${APP_ROOT:-.}"

cd $APP_ROOT_DIR
exec <start your server here>
`

const UsageScript = `
#!/bin/bash -e
cat <<EOF
This is a STI {{.ImageName}} image:
To use it, install STI: https://github.com/openshift/source-to-image

Sample invocation:

sti build git://<source code> {{.ImageName}} <application image>

You can then run the resulting image via:
docker run <application image>
EOF
`

const SaveArtifactsScript = `
#!/bin/sh -e
#
# STI save-artifacts script for the '{{.ImageName}}' image.
# The save-artifacts script streams a tar archive to standard output.
# The archive contains files and folder you want to move from a previous build
# to a new build.
#
# For more informations see the documentation:
#	https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md
#
pushd ${HOME} >/dev/null
# tar cf - <list of files and folders>
popd >/dev/null
`
