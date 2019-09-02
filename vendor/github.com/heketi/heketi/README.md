[![Build Status](https://travis-ci.org/heketi/heketi.svg?branch=master)](https://travis-ci.org/heketi/heketi)
[![Coverage Status](https://coveralls.io/repos/heketi/heketi/badge.svg)](https://coveralls.io/r/heketi/heketi)
[![Go Report Card](https://goreportcard.com/badge/github.com/heketi/heketi)](https://goreportcard.com/report/github.com/heketi/heketi)

# Heketi
Heketi provides a RESTful management interface which can be used to manage the life cycle of GlusterFS volumes.  With Heketi, cloud services like OpenStack Manila, Kubernetes, and OpenShift can dynamically provision GlusterFS volumes with any of the supported durability types.  Heketi will automatically determine the location for bricks across the cluster, making sure to place bricks and its replicas across different failure domains.  Heketi also supports any number of GlusterFS clusters, allowing cloud services to provide network file storage without being limited to a single GlusterFS cluster.

# Workflow
When a request is received to create a volume, Heketi will first allocate the appropriate storage in a cluster, making sure to place brick replicas across failure domains.  It will then format, then mount the storage to create bricks for the volume requested.  Once all bricks have been automatically created, Heketi will finally satisfy the request by creating, then starting the newly created GlusterFS volume.

# Downloads

Heketi source code can be obtained via the
[project's releases page](https://github.com/heketi/heketi/releases)
or by cloning this repository.

# Documentation

Heketi's official documentation is located in the
[docs/ directory](https://github.com/heketi/heketi/tree/master/docs/)
within the repo.

# Demo
Please visit [Vagrant-Heketi](https://github.com/heketi/vagrant-heketi) to try out the demo.

# Community

* Mailing list: [Join our mailing list](http://lists.gluster.org/mailman/listinfo/heketi-devel)
* IRC: #heketi on Freenode

# Talks

* DevNation 2016

[![image](https://img.youtube.com/vi/gmEUnOmDziQ/3.jpg)](https://youtu.be/gmEUnOmDziQ)
[Slides](http://bit.ly/29avBJX)

* Devconf.cz 2016:

[![image](https://img.youtube.com/vi/jpkG4wciy4U/3.jpg)](https://www.youtube.com/watch?v=jpkG4wciy4U) [Slides](https://github.com/lpabon/go-slides)

