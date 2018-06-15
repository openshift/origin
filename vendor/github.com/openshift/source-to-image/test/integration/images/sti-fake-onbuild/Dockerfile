FROM sti_test/sti-fake

ONBUILD RUN touch /sti-fake/src/onbuild

# The ONBUILD strategy only works with the application source dir so we need
# to manually specify to copy to another location.
#
# This is a little hack-ish given that we know our assemble script requires files to be in /tmp/src
# we will copy there, and also set our WORKDIR to be the same location so it has access to the scripts
ONBUILD COPY . /tmp/src/
WORKDIR /tmp/src