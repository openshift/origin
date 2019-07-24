#!/usr/bin/env bats

load helpers

@test "idmaps-create-apply-layer" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac

	n=5
	host=2
	# Create some temporary files.
	for i in $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
		chown ${i}:${i} "$TESTDIR"/file$i
		ln -s . $TESTDIR/subdir$i
	done
	# Use them to create some diffs.
	pushd $TESTDIR > /dev/null
	for i in $(seq $n) ; do
		tar cf diff${i}.tar subdir$i/
	done
	popd > /dev/null
	# Select some ID ranges.
	for i in $(seq $n) ; do
		uidrange[$i]=$((($RANDOM+32767)*65536))
		gidrange[$i]=$((($RANDOM+32767)*65536))
	done
	# Create a layer using the host's mappings.
	run storage --debug=false create-layer --hostuidmap --hostgidmap
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Copy the files in (host mapping, so it's fine), and set ownerships on them.
	cp -p "$TESTDIR"/file1 ${lowermount}
	cp -p "$TESTDIR"/file2 ${lowermount}
	cp -p "$TESTDIR"/file3 ${lowermount}
	cp -p "$TESTDIR"/file4 ${lowermount}
	cp -p "$TESTDIR"/file5 ${lowermount}
	# Create new layers.
	for i in $(seq $n) ; do
		if test $host -ne $i ; then
			run storage --debug=false create-layer --uidmap 0:${uidrange[$i]}:$(($n+1)) --gidmap 0:${gidrange[$i]}:$(($n+1)) $lowerlayer
		else
			uidrange[$host]=0
			gidrange[$host]=0
			run storage --debug=false create-layer --hostuidmap --hostgidmap $lowerlayer
		fi
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		upperlayer="$output"
		run storage --debug=false mount $upperlayer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		uppermount="$output"
		run storage --debug=false apply-diff $upperlayer < ${TESTDIR}/diff${i}.tar
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" = "" ]
		for j in $(seq $n) ; do
			# Check the inherited original files.
			cmp ${uppermount}/file$j "$TESTDIR"/file$j
			uid=$(stat -c %u ${uppermount}/file$j)
			gid=$(stat -c %g ${uppermount}/file$j)
			echo test found ${uid}:${gid} = expected $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j)) for file$j
			test ${uid}:${gid} = $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j))
			# Check the inherited/current layer's diff files.
			for k in $(seq $i) ; do
				cmp ${uppermount}/subdir$k/file$j "$TESTDIR"/file$j
				uid=$(stat -c %u ${uppermount}/subdir$k/file$j)
				gid=$(stat -c %g ${uppermount}/subdir$k/file$j)
				echo test found ${uid}:${gid} = expected $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j)) for subdir$k/file$j
				test ${uid}:${gid} = $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j))
			done
		done
		lowerlayer=$upperlayer
	done
}

@test "idmaps-create-diff-layer" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac
	n=5
	host=2
	# Create some temporary files.
	for i in $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
	done
	# Select some ID ranges.
	for i in 0 $(seq $n) ; do
		uidrange[$i]=$((($RANDOM+32767)*65536))
		gidrange[$i]=$((($RANDOM+32767)*65536))
	done
	# Create a layer using some random mappings.
	run storage --debug=false create-layer --uidmap 0:${uidrange[0]}:$(($n+1)) --gidmap 0:${gidrange[0]}:$(($n+1))
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Copy the files in, and set ownerships on them.
	for i in $(seq $n) ; do
		cp "$TESTDIR"/file$i ${lowermount}
		chown $((${uidrange[0]}+$i)):$((${gidrange[0]}+$i)) ${lowermount}/file$i
	done
	# Create new layers.
	for i in $(seq $n) ; do
		if test $host -ne $i ; then
			run storage --debug=false create-layer --uidmap 0:${uidrange[$i]}:$(($n+1)) --gidmap 0:${gidrange[$i]}:$(($n+1)) $lowerlayer
		else
			uidrange[$host]=0
			gidrange[$host]=0
			run storage --debug=false create-layer --hostuidmap --hostgidmap $lowerlayer
		fi
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		upperlayer="$output"
		run storage --debug=false mount $upperlayer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		uppermount="$output"
		# Change the owner of one file to 0:0.
		chown ${uidrange[$i]}:${gidrange[$i]} ${uppermount}/file$i
		# Verify that the file is the only thing that shows up in a change list.
		run storage --debug=false changes $upperlayer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		[ "${#lines[*]}" -eq 1 ]
		[ "$output" = "Modify \"/file$i\"" ]
		# Verify that the file is the only thing that shows up in a diff.
		run storage --debug=false diff -f $TESTDIR/diff.tar $upperlayer
		echo "$output"
		[ "$status" -eq 0 ]
		run tar tf $TESTDIR/diff.tar
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		[ "${#lines[*]}" -eq 1 ]
		[ "$output" = file$i ]
		# Check who owns that file, according to the diff.
		mkdir "$TESTDIR"/subdir$i
		pushd "$TESTDIR"/subdir$i > /dev/null
		run tar xf $TESTDIR/diff.tar
		[ "$status" -eq 0 ]
		run stat -c %u:%g file$i
		[ "$status" -eq 0 ]
		[ "$output" = 0:0 ]
		popd > /dev/null
		lowerlayer=$upperlayer
	done
}

@test "idmaps-create-container" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac
	n=5
	host=2
	# Create some temporary files.
	for i in $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
	done
	# Select some ID ranges.
	for i in 0 $(seq $(($n+1))) ; do
		uidrange[$i]=$((($RANDOM+32767)*65536))
		gidrange[$i]=$((($RANDOM+32767)*65536))
	done
	# Create a layer using some random mappings.
	run storage --debug=false create-layer --uidmap 0:${uidrange[0]}:$(($n+1)) --gidmap 0:${gidrange[0]}:$(($n+1))
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Copy the files in, and set ownerships on them.
	for i in $(seq $n) ; do
		cp "$TESTDIR"/file$i ${lowermount}
		chown $((${uidrange[0]}+$i)):$((${gidrange[0]}+$i)) ${lowermount}/file$i
	done
	# Create new layers.
	for i in $(seq $n) ; do
		if test $host -ne $i ; then
			run storage --debug=false create-layer --uidmap 0:${uidrange[$i]}:$(($n+1)) --gidmap 0:${gidrange[$i]}:$(($n+1)) $lowerlayer
		else
			uidrange[$host]=0
			gidrange[$host]=0
			run storage --debug=false create-layer --hostuidmap --hostgidmap $lowerlayer
		fi
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		upperlayer="$output"
		run storage --debug=false mount $upperlayer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		uppermount="$output"
		# Change the owner of one file to 0:0.
		chown ${uidrange[$i]}:${gidrange[$i]} ${uppermount}/file$i
		# Verify that the file is the only thing that shows up in a change list.
		run storage --debug=false changes $upperlayer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		[ "${#lines[*]}" -eq 1 ]
		[ "$output" = "Modify \"/file$i\"" ]
		lowerlayer=$upperlayer
	done
	# Create new containers based on the layer.
	imagename=idmappedimage
	storage create-image --name=$imagename $lowerlayer

	run storage --debug=false create-container --hostuidmap --hostgidmap $imagename
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	hostcontainer="$output"
	run storage --debug=false mount $hostcontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	hostmount="$output"
	for i in $(seq $n) ; do
		run stat -c %u:%g "$hostmount"/file$i
		[ "$status" -eq 0 ]
		[ "$output" = 0:0 ]
	done

	run storage --debug=false create-container --hostuidmap --hostgidmap $imagename
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	hostcontainer="$output"
	run storage --debug=false mount $hostcontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	hostmount="$output"
	for i in $(seq $n) ; do
		run stat -c %u:%g "$hostmount"/file$i
		[ "$status" -eq 0 ]
		[ "$output" = 0:0 ]
	done

	run storage --debug=false create-container --uidmap 0:${uidrange[$(($n+1))]}:$(($n+1)) --gidmap 0:${gidrange[$(($n+1))]}:$(($n+1)) $imagename
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	newmapcontainer="$output"
	run storage --debug=false mount $newmapcontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	newmapmount="$output"
	for i in $(seq $n) ; do
		run stat -c %u:%g "$newmapmount"/file$i
		[ "$status" -eq 0 ]
		[ "$output" = ${uidrange[$(($n+1))]}:${gidrange[$(($n+1))]} ]
	done

	run storage --debug=false create-container $imagename
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	defmapcontainer="$output"
	run storage --debug=false mount $defmapcontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	defmapmount="$output"
	for i in $(seq $n) ; do
		run stat -c %u:%g "$defmapmount"/file$i
		[ "$status" -eq 0 ]
		[ "$output" = ${uidrange[$n]}:${gidrange[$n]} ]
	done
}

@test "idmaps-parent-owners" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac
	n=5
	# Create some temporary files.
	for i in $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
	done
	# Select some ID ranges.
	uidrange=$((($RANDOM+32767)*65536))
	gidrange=$((($RANDOM+32767)*65536))
	# Create a layer using some random mappings.
	run storage --debug=false create-layer --uidmap 0:${uidrange}:$(($n+1)) --gidmap 0:${gidrange}:$(($n+1))
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer="$output"
	# Mount the layer.
	run storage mount $layer
	echo "$output"
	[ "$status" -eq 0 ]
	# Check who owns the parent directories.
	run storage --debug=false layer-parent-owners $layer
	echo "$output"
	[ "$status" -eq 0 ]
	# Assume that except for root and maybe us, there are no other owners of parent directories of our layer.
	if ! fgrep -q 'UIDs: [0]' <<< "$output" ; then
		fgrep -q 'UIDs: [0, '$(id -u)']' <<< "$output"
	fi
	if ! fgrep -q 'GIDs: [0]' <<< "$output" ; then
		fgrep -q 'GIDs: [0, '$(id -g)']' <<< "$output"
	fi
	# Create a new container based on the layer.
	imagename=idmappedimage
	storage create-image --name=$imagename $layer
	run storage --debug=false create-container $imagename
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container="$output"
	# Mount the container.
	run storage mount $container
	echo "$output"
	[ "$status" -eq 0 ]
	# Check who owns the parent directories.
	run storage --debug=false container-parent-owners $container
	echo "$output"
	[ "$status" -eq 0 ]
	# Assume that except for root and maybe us, there are no other owners of parent directories of our container's layer.
	if ! fgrep -q 'UIDs: [0]' <<< "$output" ; then
		fgrep -q 'UIDs: [0, '$(id -u)']' <<< "$output"
	fi
	if ! fgrep -q 'GIDs: [0]' <<< "$output" ; then
		fgrep -q 'GIDs: [0, '$(id -g)']' <<< "$output"
	fi
}

@test "idmaps-copy" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac
	n=5
	host=2
	# Create some temporary files.
	mkdir -p "$TESTDIR"/subdir/subdir2
	for i in 0 $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
		chown "$i":"$i" "$TESTDIR"/file$i
		createrandom "$TESTDIR"/subdir/subdir2/file$i
		chown "$i":"$i" "$TESTDIR"/subdir/subdir2/file$i
	done
	chown "$(($n+1))":"$(($n+1))" "$TESTDIR"/subdir/subdir2
	# Select some ID ranges for ID mappings.
	for i in 0 $(seq $(($n+1))) ; do
		uidrange[$i]=$((($RANDOM+32767)*65536))
		gidrange[$i]=$((($RANDOM+32767)*65536))
		# Create a layer using some those mappings.
		run storage --debug=false create-layer --uidmap 0:${uidrange[$i]}:101 --gidmap 0:${gidrange[$i]}:101
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		layer[$i]="$output"
	done
	# Copy the file in, and check that its ownerships get mapped correctly.
	for i in 0 $(seq $n) ; do
		run storage copy "$TESTDIR"/file$i ${layer[$i]}:/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		run storage --debug=false mount ${layer[$i]}
		echo "$output"
		[ "$status" -eq 0 ]
		mnt[$i]="$output"
		run stat -c "%u:%g" ${mnt[$i]}/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+$i))
		gid=$((${gidrange[$i]}+$i))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]
		# Try copying with --chown.
		run storage copy --chown 100:100 "$TESTDIR"/file$i ${layer[$i]}:/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		run storage --debug=false mount ${layer[$i]}
		echo "$output"
		[ "$status" -eq 0 ]
		mnt[$i]="$output"
		run stat -c "%u:%g" ${mnt[$i]}/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+100))
		gid=$((${gidrange[$i]}+100))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]
	done
	# Copy the subdirectory, and check that its ownerships and that of its contents get mapped correctly.
	for i in 0 $(seq $n) ; do
		run storage copy "$TESTDIR"/file$i ${layer[$i]}:/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		run stat -c "%u:%g" ${mnt[$i]}/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+$i))
		gid=$((${gidrange[$i]}+$i))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]

		# Try copying with --chown.
		run storage copy --chown 100:100 "$TESTDIR"/file$i ${layer[$i]}:/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		run stat -c "%u:%g" ${mnt[$i]}/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+100))
		gid=$((${gidrange[$i]}+100))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]

		# Try copying a directory tree.
		run storage copy "$TESTDIR"/subdir ${layer[$i]}:/subdir
		echo "$output"
		[ "$status" -eq 0 ]
		run stat -c "%u:%g" ${mnt[$i]}/subdir/subdir2/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+$i))
		gid=$((${gidrange[$i]}+$i))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]
		run stat -c "%u:%g" ${mnt[$i]}/subdir/subdir2
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+$n+1))
		gid=$((${gidrange[$i]}+$n+1))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]

		# Try copying a directory tree with --chown.
		run storage copy --chown 100:100 "$TESTDIR"/subdir ${layer[$i]}:/subdir2
		echo "$output"
		[ "$status" -eq 0 ]
		run stat -c "%u:%g" ${mnt[$i]}/subdir2/subdir2/file$i
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+100))
		gid=$((${gidrange[$i]}+100))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]
		run stat -c "%u:%g" ${mnt[$i]}/subdir2/subdir2
		echo "$output"
		[ "$status" -eq 0 ]
		uid=$((${uidrange[$i]}+100))
		gid=$((${gidrange[$i]}+100))
		echo comparing "$output" and "$uid:$gid"
		[ "$output" == "$uid:$gid" ]
	done

	# Copy a file out of a layer, into another one.
	run storage copy "$TESTDIR"/file$n ${layer[0]}:/file$n
	echo "$output"
	[ "$status" -eq 0 ]
	run storage copy ${layer[0]}:/file$n ${layer[$n]}:/file$n
	echo "$output"
	[ "$status" -eq 0 ]
	run stat -c "%u:%g" ${mnt[$n]}/file$n
	echo "$output"
	[ "$status" -eq 0 ]
	uid=$((${uidrange[$n]}+$n))
	gid=$((${gidrange[$n]}+$n))
	echo comparing "$output" and "$uid:$gid"
	[ "$output" == "$uid:$gid" ]

	# Try copying a directory tree.
	run storage copy ${layer[0]}:/subdir ${layer[$n]}:/subdir
	echo "$output"
	[ "$status" -eq 0 ]
	run stat -c "%u:%g" ${mnt[$n]}/subdir/subdir2/file$n
	echo "$output"
	[ "$status" -eq 0 ]
	uid=$((${uidrange[$n]}+$n))
	gid=$((${gidrange[$n]}+$n))
	echo comparing "$output" and "$uid:$gid"
	[ "$output" == "$uid:$gid" ]
	run stat -c "%u:%g" ${mnt[$n]}/subdir/subdir2
	echo "$output"
	[ "$status" -eq 0 ]
	uid=$((${uidrange[$n]}+$n+1))
	gid=$((${gidrange[$n]}+$n+1))
	echo comparing "$output" and "$uid:$gid"
	[ "$output" == "$uid:$gid" ]
}

@test "idmaps-create-mapped-image" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac
	n=5
	host=2
	# Create some temporary files.
	for i in $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
		chown ${i}:${i} "$TESTDIR"/file$i
	done
	# Select some ID ranges.
	for i in $(seq $n) ; do
		uidrange[$i]=$((($RANDOM+32767)*65536))
		gidrange[$i]=$((($RANDOM+32767)*65536))
	done
	# Create a layer using the host's mappings.
	run storage --debug=false create-layer --hostuidmap --hostgidmap
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Copy the files in (host mapping, so it's fine), and set ownerships on them.
	cp -p "$TESTDIR"/file1 ${lowermount}
	cp -p "$TESTDIR"/file2 ${lowermount}
	cp -p "$TESTDIR"/file3 ${lowermount}
	cp -p "$TESTDIR"/file4 ${lowermount}
	cp -p "$TESTDIR"/file5 ${lowermount}
	# Create an image record for this layer.
	run storage --debug=false create-image $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image="$output"
	echo image:$image
	# Check that we can compute the size of the image.
	run storage --debug=false image $image
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	size=$(grep ^Size: <<< "$output" | sed 's,^Size: ,,g')
	[ "$size" -ne 0 ]
	echo size:$size
	# Create containers using this image.
	containers=
	for i in $(seq $n) ; do
		if test $host -ne $i ; then
			run storage --debug=false create-container --uidmap 0:${uidrange[$i]}:$(($n+1)) --gidmap 0:${gidrange[$i]}:$(($n+1)) $image
		else
			uidrange[$host]=0
			gidrange[$host]=0
			run storage --debug=false create-container --hostuidmap --hostgidmap $image
		fi
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		container=${lines[0]}
		containers[$i-1]="$container"

		# Check that the ownerships came out right.
		run storage --debug=false mount "$container"
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		mount="$output"

		for j in $(seq $n) ; do
			ownerids=$(stat -c %u:%g ${mount}/file$j)
			echo on-disk IDs: "$ownerids"
			echo expected IDs: $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j))
			[ "$ownerids" = $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j)) ]
		done
		run storage --debug=false unmount "$container"
		[ "$status" -eq 0 ]
	done
	# Each of the containers' layers should have a different parent layer,
	# all of which should be a top layer for the image.  The containers
	# themselves have no contents at this point.
	declare -a parents
	echo containers list is \"${containers[*]}\"
	for container in "${containers[@]}" ; do
		run storage --debug=false container $container
		echo container "$container":"$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		baseimage=$(grep ^Image: <<< "$output" | sed 's,^Image: ,,g')
		echo baseimage:"$baseimage"
		[ "$baseimage" = "$image" ]
		layer=$(grep ^Layer: <<< "$output" | sed 's,^Layer: ,,g')
		echo layer:"$layer"
		size=$(grep ^Size: <<< "$output" | sed 's,^Size: ,,g')
		[ "$size" -eq 0 ]
		echo size:$size

		run storage --debug=false layer $layer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		parent=$(grep ^Parent: <<< "$output" | sed 's,^Parent: ,,g')
		echo parent:"$parent"

		parents[${#parents[*]}]="$parent"

		run storage --debug=false image $baseimage
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		grep "^Top Layer: $parent" <<< "$output"
	done
	nparents=$(for p in ${parents[@]} ; do echo $p ; done | sort -u | wc -l)
	echo nparents:$nparents
	[ $nparents -eq $n ]

	# The image should have five top layers at this point.
	run storage --debug=false image $image
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	tops=$(grep '^Top Layer:' <<< "$output" | sed 's,^Top Layer: ,,g')
	ntops=$(for p in $tops; do echo $p ; done | sort -u | wc -l)
	echo ntops:$ntops
	[ $ntops -eq $n ]
}

@test "idmaps-create-mapped-container" {
	case "$STORAGE_DRIVER" in
	btrfs|devicemapper|overlay*|vfs|zfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac
	n=5
	host=2
	# Create some temporary files.
	for i in $(seq $n) ; do
		createrandom "$TESTDIR"/file$i
		chown ${i}:${i} "$TESTDIR"/file$i
	done
	# Select some ID ranges.
	for i in $(seq $n) ; do
		uidrange[$i]=$((($RANDOM+32767)*65536))
		gidrange[$i]=$((($RANDOM+32767)*65536))
	done
	# Create a layer using the host's mappings.
	run storage --debug=false --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot create-layer --hostuidmap --hostgidmap
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot mount $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Copy the files in (host mapping, so it's fine), and set ownerships on them.
	cp -p "$TESTDIR"/file1 ${lowermount}
	cp -p "$TESTDIR"/file2 ${lowermount}
	cp -p "$TESTDIR"/file3 ${lowermount}
	cp -p "$TESTDIR"/file4 ${lowermount}
	cp -p "$TESTDIR"/file5 ${lowermount}
	# Unmount the layer.
	run storage --debug=false --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot unmount $lowerlayer
	[ "$status" -eq 0 ]
	# Create an image record for this layer.
	run storage --debug=false --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot create-image $lowerlayer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image="$output"
	echo image:$image
	# Check that we can compute the size of the image.
	run storage --debug=false --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot image $image
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	size=$(grep ^Size: <<< "$output" | sed 's,^Size: ,,g')
	[ "$size" -ne 0 ]
	echo size:$size
	# Done using this location directly.
	storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot shutdown
	# Create containers using this image.
	containers=
	for i in $(seq $n) ; do
		if test $host -ne $i ; then
			run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root create-container --uidmap 0:${uidrange[$i]}:$(($n+1)) --gidmap 0:${gidrange[$i]}:$(($n+1)) $image
		else
			uidrange[$host]=0
			gidrange[$host]=0
			run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root create-container --hostuidmap --hostgidmap $image
		fi
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		container=${lines[0]}
		containers[$i-1]="$container"

		# Check that the ownerships came out right.
		run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root mount "$container"
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		mount="$output"

		for j in $(seq $n) ; do
			ownerids=$(stat -c %u:%g ${mount}/file$j)
			echo on-disk IDs: "$ownerids"
			echo expected IDs: $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j))
			[ "$ownerids" = $((${uidrange[$i]}+$j)):$((${gidrange[$i]}+$j)) ]
		done
		run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root unmount "$container"
		[ "$status" -eq 0 ]
	done
	# Each of the containers' layers should have the same parent layer,
	# which should be the lone top layer for the image.  The containers
	# themselves have no contents at this point.
	declare -a parents
	echo containers list is \"${containers[*]}\"
	for container in "${containers[@]}" ; do
		run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root container $container
		echo container "$container":"$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		baseimage=$(grep ^Image: <<< "$output" | sed 's,^Image: ,,g')
		echo baseimage:"$baseimage"
		[ "$baseimage" = "$image" ]
		layer=$(grep ^Layer: <<< "$output" | sed 's,^Layer: ,,g')
		echo layer:"$layer"
		size=$(grep ^Size: <<< "$output" | sed 's,^Size: ,,g')
		[ "$size" -eq 0 ]
		echo size:$size

		run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root layer $layer
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		parent=$(grep ^Parent: <<< "$output" | sed 's,^Parent: ,,g')
		echo parent:"$parent"

		parents[${#parents[*]}]="$parent"

		run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root image $baseimage
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		grep "^Top Layer: $parent" <<< "$output"
	done
	nparents=$(for p in ${parents[@]} ; do echo $p ; done | sort -u | wc -l)
	echo nparents:$nparents
	[ $nparents -eq 1 ]

	# The image should still have only one top layer at this point.
	run storage --debug=false --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root image $image
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	tops=$(grep '^Top Layer:' <<< "$output" | sed 's,^Top Layer: ,,g')
	ntops=$(for p in $tops; do echo $p ; done | sort -u | wc -l)
	echo ntops:$ntops
	[ $ntops -eq 1 ]
}

@test "idmaps-create-mapped-container-shifting" {
	case "$STORAGE_DRIVER" in
	overlay*)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	case "$STORAGE_OPTION" in
	*mount_program*)
		skip "test not supported when using mount_program"
		;;
	esac

	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer="$output"
	# Mount the layer.
	run storage --debug=false mount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Put a file in the layer.
	createrandom "$lowermount"/file
	storage unmount $layer

	imagename=idmappedimage-shifting
	storage create-image --name=$imagename $layer

	_TEST_FORCE_SUPPORT_SHIFTING=yes-please run storage --debug=false create-container --uidmap 0:1000:1000 --gidmap 0:1000:1000 $imagename
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]

	container="$output"

	# Mount the container.
	_TEST_FORCE_SUPPORT_SHIFTING=yes-please run storage --debug=false mount $container
	echo "$output"
	[ "$status" -eq 0 ]
	dir="$output"
	test "$(stat -c%u:%g $dir/file)" == "0:0"
	_TEST_FORCE_SUPPORT_SHIFTING=yes-please run storage --debug=false unmount "$container"
	[ "$status" -eq 0 ]
}
