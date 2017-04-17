# Image Garbage Collection

As a number of containers run on a node grows, disk usage increases
and available disk memory decreases. To avoid consumption of all remaining
memory, kubelet periodically runs image garbage collector.
Its task is to find images that can be deleted and remove them.

Currently, image garbage collector is executed every 5 minutes.
It has two operating parametres: `HighThresholdPercent` and `LowThresholdPercent`.
High threshold determines a maximal percentage of used memory
before the collection starts. Once the threshold is met the collector tries
to delete images and free memory until the usage drops under the low threshold.

## Detection of images to delete
Two lists of images are retrieved in each spin:

1. list of images currently running in at least one pod
2. list of images available on a host

As new containers are run, new images appear. All images are marked
with a timestamp. If the image is running (1.) or is newly detected (2.),
it is marked with the current time. The remaining images are already marked
from the previous spins. All images are then sorted by the timestamp.

Once the collection starts, the oldest images get deleted first until the stopping
criterion is met.

## Configuration
Kubelet binary provides two options that can be used to modify default value of thresholds:

1. `image-gc-high-threshold`: The percent of disk usage after which image garbage collection is always run. Default is 90%.
2. `image-gc-low-threshold`: The percent of disk usage before which image garbage collection is never run. Lowest disk usage to garbage collect to. Default is 80%.
