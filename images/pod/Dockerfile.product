#
# This is the official OpenShift Origin pod infrastructure image. It will stay running
# until terminated by a signal and is the heart of each running pod. It holds on to
# the network and IPC namespaces as containers come and go during the lifetime of the
# pod.
#
# The standard name for this image is openshift3/ose-pod
#
# Pulp can't handle the scratch later currently
#FROM scratch
FROM registry.access.redhat.com/rhel

ADD bin/pod /pod

ENTRYPOINT ["/pod"]
