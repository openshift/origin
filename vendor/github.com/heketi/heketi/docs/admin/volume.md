To create a volume you can use the [Volume Create API](../api/api.md#create-a-volume) or the command line client.

From the command line client, you can type the following to create a 1TB volume with replica 3 durability:

```
$ heketi-cli volume create --size=1024
```

Once the request has been completed, which could take some time depending on the hardware, information about the volume will be displayed on standard out, including the mount point of the volume.

# Example
There is an example of how to create a volume during the [Gluster.Next](https://www.youtube.com/watch?v=iBFfHv4bne8&t=2750) talk.

# heketi-cli usage
```
Create a GlusterFS volume

USAGE
  heketi-cli volume create [options]

OPTIONS
  -clusters string
    	
	Optional: Comma separated list of cluster ids where this volume
	must be allocated. If ommitted, Heketi will allocate the volume
	on any of the configured clusters which have the available space.
	Providing a set of clusters will ensure Heketi allocates storage
	for this volume only in the clusters specified.
  -disperse-data int
    	
	Optional: Dispersion value for durability type 'disperse'.
	Default is 4 (default 4)
  -durability string
    	
	Optional: Durability type.  Values are:
		none: No durability.  Distributed volume only.
		replicate: (Default) Distributed-Replica volume.
		disperse: Distributed-Erasure Coded volume. (default "replicate")
  -name string
    	
	Optional: Name of volume. Only set if really necessary
  -redundancy int
    	
	Optional: Redundancy value for durability type 'disperse'.
	Default is 2 (default 2)
  -replica int
    	
	Replica value for durability type 'replicate'.
	Default is 3 (default 3)
  -size int
    	
	Size of volume in GiB (default -1)
  -snapshot-factor float
    	
	Optional: Amount of storage to allocate for snapshot support.
	Must be greater 1.0.  For example if a 10TiB volume requires 5TiB of
	snapshot storage, then snapshot-factor would be set to 1.5.  If the
	value is set to 1, then snapshots will not be enabled for this volume (default 1)

Note:
The volume size created depends upon the underlying brick size.
For example, for a 2 way/3 way replica volume, the minimum volume size is 1GiB as the
underlying minimum brick size is constrained to 1GiB.

So, it is not possible create a volume of size less than 1GiB.

EXAMPLES
  * Create a 100GiB replica 3 volume:
      $ heketi-cli volume create -size=100

  * Create a 100GiB replica 3 volume specifying two specific clusters:
      $ heketi-cli volume create -size=100 \
          -clusters=0995098e1284ddccb46c7752d142c832,60d46d518074b13a04ce1022c8c7193c

  * Create a 100GiB replica 2 volume with 50GiB of snapshot storage:
      $ heketi-cli volume create -size=100 -snapshot-factor=1.5 -replica=2 

  * Create a 100GiB distributed volume
      $ heketi-cli volume create -size=100 -durabilty=none

  * Create a 100GiB erasure coded 4+2 volume with 25GiB snapshot storage:
      $ heketi-cli volume create -size=100 -durability=disperse -snapshot-factor=1.25

  * Create a 100GiB erasure coded 8+3 volume with 25GiB snapshot storage:
      $ heketi-cli volume create -size=100 -durability=disperse -snapshot-factor=1.25 \
          -disperse-data=8 -redundancy=3
```
