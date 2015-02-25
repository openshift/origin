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

if [ "$1" = "-h" ]; then
	# If the '{{.ImageName}}' assemble script is executed with '-h' flag,
	# print the usage.
	exec /usr/local/sti/usage
fi

# Restore artifacts from the previous build (if exists).
#
if [ "$(ls /tmp/artifacts/ 2>/dev/null)" ]; then
  echo "---> Restoring build artifacts"
  mv /tmp/artifacts/* ./
fi

echo "---> Installing application source"
cp -Rf /tmp/src/* ./

echo "---> Building application from source"
# TODO: Add build steps for your application, for example npm install or
#       bundle install, etc...
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
# tar cf - <list of files and folders>
`
