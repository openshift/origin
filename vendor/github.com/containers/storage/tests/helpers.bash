#!/bin/bash

STORAGE_BINARY=${STORAGE_BINARY:-$(dirname ${BASH_SOURCE})/../containers-storage}
TESTSDIR=${TESTSDIR:-$(dirname ${BASH_SOURCE})}
STORAGE_DRIVER=${STORAGE_DRIVER:-vfs}
STORAGE_OPTION=${STORAGE_OPTION:-}
PATH=$(dirname ${BASH_SOURCE})/..:${PATH}

# Create a unique root directory and a runroot directory.
function setup() {
	suffix=$(dd if=/dev/urandom bs=12 count=1 status=none | base64 | tr +/ABCDEFGHIJKLMNOPQRSTUVWXYZ _.abcdefghijklmnopqrstuvwxyz)
	TESTDIR=${BATS_TMPDIR}/tmp.${suffix}
	rm -fr ${TESTDIR}
	mkdir -p ${TESTDIR}/{root,runroot}
}

# Delete the unique root directory and a runroot directory.
function teardown() {
	storage wipe
	storage shutdown
	rm -fr ${TESTDIR}
}

# Create a file "$1" with random contents of length $2, or 256.
function createrandom() {
	dd if=/dev/urandom bs=1 count=${2:-256} of=${1:-${BATS_TMPDIR}/randomfile} status=none
	# Set the mtime to the epoch so it won't be different once it is deduplicated with OSTree
	touch -t 7001010000.00 ${1:-${BATS_TMPDIR}/randomfile}
}

# Run the CLI with the specified options.
function storage() {
	${STORAGE_BINARY} --debug --graph ${TESTDIR}/root --run ${TESTDIR}/runroot --storage-driver ${STORAGE_DRIVER} ${STORAGE_OPTION:+--storage-opt=${STORAGE_OPTION}} "$@"
}

# Run the CLI with the specified options, and sort its output lines.
function storagewithsorting() {
	storage "$@" | LC_ALL=C sort
}

# Run the CLI with the specified options, and sort its output lines using the second field.
function storagewithsorting2() {
	storage "$@" | LC_ALL=C sort -k2
}

# Create a few layers with files and directories added and removed at each
# layer.  Their IDs are set to $lowerlayer, $midlayer, and $upperlayer.
populate() {
	# Create a base layer.
	run storage --debug=false create-layer
	echo $output
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	local lowermount="$output"
	# Create three files, and nine directories: three empty, three with subdirectories, three with files.
	createrandom "$lowermount"/layer1file1
	createrandom "$lowermount"/layer1file2
	createrandom "$lowermount"/layer1file3
	mkdir "$lowermount"/layerdir1
	mkdir "$lowermount"/layerdir2
	mkdir "$lowermount"/layerdir3
	mkdir "$lowermount"/layerdir4
	mkdir "$lowermount"/layerdir4/layer1subdir
	mkdir "$lowermount"/layerdir5
	mkdir "$lowermount"/layerdir5/layer1subdir
	mkdir "$lowermount"/layerdir6
	mkdir "$lowermount"/layerdir6/layer1subdir
	mkdir "$lowermount"/layerdir7
	createrandom "$lowermount"/layerdir7/layer1file4
	mkdir "$lowermount"/layerdir8
	createrandom "$lowermount"/layerdir8/layer1file5
	mkdir "$lowermount"/layerdir9
	createrandom "$lowermount"/layerdir9/layer1file6
	# Unmount the layer.
	storage unmount $lowerlayer

	# Create a second layer based on the first.
	run storage --debug=false create-layer "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"
	# Mount the second layer.
	run storage --debug=false mount $midlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	local midmount="$output"
	# Check that the files and directories from the first layer are present.
	test -s "$midmount"/layer1file1
	test -s "$midmount"/layer1file2
	test -s "$midmount"/layer1file3
	test -d "$midmount"/layerdir1
	test -d "$midmount"/layerdir2
	test -d "$midmount"/layerdir3
	test -d "$midmount"/layerdir4
	test -d "$midmount"/layerdir4/layer1subdir
	test -d "$midmount"/layerdir5
	test -d "$midmount"/layerdir5/layer1subdir
	test -d "$midmount"/layerdir6
	test -d "$midmount"/layerdir6/layer1subdir
	test -d "$midmount"/layerdir7
	test -s "$midmount"/layerdir7/layer1file4
	test -d "$midmount"/layerdir8
	test -s "$midmount"/layerdir8/layer1file5
	test -d "$midmount"/layerdir9
	test -s "$midmount"/layerdir9/layer1file6
	# Now remove some of those files and directories.
	rm "$midmount"/layer1file1
	rm "$midmount"/layer1file2
	rmdir "$midmount"/layerdir1
	rmdir "$midmount"/layerdir2
	rmdir "$midmount"/layerdir4/layer1subdir
	rmdir "$midmount"/layerdir4
	rmdir "$midmount"/layerdir5/layer1subdir
	rmdir "$midmount"/layerdir5
	rm "$midmount"/layerdir7/layer1file4
	rmdir "$midmount"/layerdir7
	rm "$midmount"/layerdir8/layer1file5
	rmdir "$midmount"/layerdir8
	# Add a couple of new files and directories.
	createrandom "$midmount"/layer2file1
	mkdir "$midmount"/layerdir10
	mkdir "$midmount"/layerdir11
	mkdir "$midmount"/layerdir11/layer2subdir
	mkdir "$midmount"/layerdir12
	createrandom "$midmount"/layerdir12/layer2file2
	# Unmount the layer.
	storage unmount $midlayer

	# Create a third layer based on the second.
	run storage --debug=false create-layer "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"
	# Mount the third layer.
	run storage --debug=false mount $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	local uppermount="$output"
	# Check that contents of the second layer are present.
	test -s "$uppermount"/layer1file3
	test -d "$uppermount"/layerdir3
	test -d "$uppermount"/layerdir6
	test -d "$uppermount"/layerdir6/layer1subdir
	test -d "$uppermount"/layerdir9
	test -s "$uppermount"/layerdir9/layer1file6
	test -s "$uppermount"/layer2file1
	test -d "$uppermount"/layerdir10
	test -d "$uppermount"/layerdir11
	test -d "$uppermount"/layerdir11/layer2subdir
	test -d "$uppermount"/layerdir12
	test -s "$uppermount"/layerdir12/layer2file2
	# Re-add some contents for this layer that were removed earlier.
	createrandom "$uppermount"/layerfile1
	mkdir "$uppermount"/layerdir1
	mkdir "$uppermount"/layerdir4
	mkdir "$uppermount"/layerdir4/layer1subdir
	mkdir "$uppermount"/layerdir7
	# Add some new contents, too.
	mkdir "$uppermount"/layerdir3/layer3subdir
	mkdir "$uppermount"/layerdir3/layer3subdir/layer3subsubdir
	createrandom "$uppermount"/layerdir7/layer1file4
	# Unmount the layer.
	storage unmount $upperlayer
}

# Check that the changes list for layers created by populate() correspond to
# what naive diff methods would generate.
checkchanges() {
	# The first layer should all be additions.
	storage changes $lowerlayer
	run storagewithsorting2 --debug=false changes $lowerlayer
	[ "$status" -eq 0 ]
	echo Changes for layer 1:
	echo "$output"
	[ "${#lines[*]}" -eq 18 ]
	[ "${lines[0]}" = 'Add "/layer1file1"' ]
	[ "${lines[1]}" = 'Add "/layer1file2"' ]
	[ "${lines[2]}" = 'Add "/layer1file3"' ]
	[ "${lines[3]}" = 'Add "/layerdir1"' ]
	[ "${lines[4]}" = 'Add "/layerdir2"' ]
	[ "${lines[5]}" = 'Add "/layerdir3"' ]
	[ "${lines[6]}" = 'Add "/layerdir4"' ]
	[ "${lines[7]}" = 'Add "/layerdir4/layer1subdir"' ]
	[ "${lines[8]}" = 'Add "/layerdir5"' ]
	[ "${lines[9]}" = 'Add "/layerdir5/layer1subdir"' ]
	[ "${lines[10]}" = 'Add "/layerdir6"' ]
	[ "${lines[11]}" = 'Add "/layerdir6/layer1subdir"' ]
	[ "${lines[12]}" = 'Add "/layerdir7"' ]
	[ "${lines[13]}" = 'Add "/layerdir7/layer1file4"' ]
	[ "${lines[14]}" = 'Add "/layerdir8"' ]
	[ "${lines[15]}" = 'Add "/layerdir8/layer1file5"' ]
	[ "${lines[16]}" = 'Add "/layerdir9"' ]
	[ "${lines[17]}" = 'Add "/layerdir9/layer1file6"' ]
	# Check the second layer.
	storage changes $midlayer
	run storagewithsorting2 --debug=false changes $midlayer
	[ "$status" -eq 0 ]
	echo Changes for layer 2:
	echo "$output"
	[ "${#lines[*]}" -eq 14 ]
	[ "${lines[0]}" = 'Delete "/layer1file1"' ]
	[ "${lines[1]}" = 'Delete "/layer1file2"' ]
	[ "${lines[2]}" = 'Add "/layer2file1"' ]
	[ "${lines[3]}" = 'Delete "/layerdir1"' ]
	[ "${lines[4]}" = 'Add "/layerdir10"' ]
	[ "${lines[5]}" = 'Add "/layerdir11"' ]
	[ "${lines[6]}" = 'Add "/layerdir11/layer2subdir"' ]
	[ "${lines[7]}" = 'Add "/layerdir12"' ]
	[ "${lines[8]}" = 'Add "/layerdir12/layer2file2"' ]
	[ "${lines[9]}" = 'Delete "/layerdir2"' ]
	[ "${lines[10]}" = 'Delete "/layerdir4"' ]
	[ "${lines[11]}" = 'Delete "/layerdir5"' ]
	[ "${lines[12]}" = 'Delete "/layerdir7"' ]
	[ "${lines[13]}" = 'Delete "/layerdir8"' ]
	# Check the third layer.
	storage changes $upperlayer
	run storagewithsorting2 --debug=false changes $upperlayer
	[ "$status" -eq 0 ]
	echo Changes for layer 3:
	echo "$output"
	[ "${#lines[*]}" -eq 9 ]
	[ "${lines[0]}" = 'Add "/layerdir1"' ]
	[ "${lines[1]}" = 'Modify "/layerdir3"' ]
	[ "${lines[2]}" = 'Add "/layerdir3/layer3subdir"' ]
	[ "${lines[3]}" = 'Add "/layerdir3/layer3subdir/layer3subsubdir"' ]
	[ "${lines[4]}" = 'Add "/layerdir4"' ]
	[ "${lines[5]}" = 'Add "/layerdir4/layer1subdir"' ]
	[ "${lines[6]}" = 'Add "/layerdir7"' ]
	[ "${lines[7]}" = 'Add "/layerdir7/layer1file4"' ]
	[ "${lines[8]}" = 'Add "/layerfile1"' ]
}

# Check that the diff contents for layers created by populate() correspond to
# what naive diff methods would generate.
checkdiffs() {
	# The first layer should all be additions.
	storage diff -u -f $TESTDIR/lower.tar $lowerlayer
	tar tf $TESTDIR/lower.tar > $TESTDIR/lower.txt
	run env LC_ALL=C sort $TESTDIR/lower.txt
	[ "$status" -eq 0 ]
	echo Diff contents for layer 1:
	echo "$output"
	[ "${#lines[*]}" -eq 18 ]
	[ "${lines[0]}" = 'layer1file1' ]
	[ "${lines[1]}" = 'layer1file2' ]
	[ "${lines[2]}" = 'layer1file3' ]
	[ "${lines[3]}" = 'layerdir1/' ]
	[ "${lines[4]}" = 'layerdir2/' ]
	[ "${lines[5]}" = 'layerdir3/' ]
	[ "${lines[6]}" = 'layerdir4/' ]
	[ "${lines[7]}" = 'layerdir4/layer1subdir/' ]
	[ "${lines[8]}" = 'layerdir5/' ]
	[ "${lines[9]}" = 'layerdir5/layer1subdir/' ]
	[ "${lines[10]}" = 'layerdir6/' ]
	[ "${lines[11]}" = 'layerdir6/layer1subdir/' ]
	[ "${lines[12]}" = 'layerdir7/' ]
	[ "${lines[13]}" = 'layerdir7/layer1file4' ]
	[ "${lines[14]}" = 'layerdir8/' ]
	[ "${lines[15]}" = 'layerdir8/layer1file5' ]
	[ "${lines[16]}" = 'layerdir9/' ]
	[ "${lines[17]}" = 'layerdir9/layer1file6' ]
	# Check the second layer.
	storage diff -c -f $TESTDIR/middle.tar $midlayer
	tar tzf $TESTDIR/middle.tar > $TESTDIR/middle.txt
	run env LC_ALL=C sort $TESTDIR/middle.txt
	[ "$status" -eq 0 ]
	echo Diff contents for layer 2:
	echo "$output"
	[ "${#lines[*]}" -eq 14 ]
	[ "${lines[0]}" = '.wh.layer1file1' ]
	[ "${lines[1]}" = '.wh.layer1file2' ]
	[ "${lines[2]}" = '.wh.layerdir1' ]
	[ "${lines[3]}" = '.wh.layerdir2' ]
	[ "${lines[4]}" = '.wh.layerdir4' ]
	[ "${lines[5]}" = '.wh.layerdir5' ]
	[ "${lines[6]}" = '.wh.layerdir7' ]
	[ "${lines[7]}" = '.wh.layerdir8' ]
	[ "${lines[8]}" = 'layer2file1' ]
	[ "${lines[9]}" = 'layerdir10/' ]
	[ "${lines[10]}" = 'layerdir11/' ]
	[ "${lines[11]}" = 'layerdir11/layer2subdir/' ]
	[ "${lines[12]}" = 'layerdir12/' ]
	[ "${lines[13]}" = 'layerdir12/layer2file2' ]
	# Check the third layer.
	storage diff -u -f $TESTDIR/upper.tar $upperlayer
	tar tf $TESTDIR/upper.tar > $TESTDIR/upper.txt
	run env LC_ALL=C sort $TESTDIR/upper.txt
	[ "$status" -eq 0 ]
	echo Diff contents for layer 3:
	echo "$output"
	[ "${#lines[*]}" -eq 9 ]
	[ "${lines[0]}" = 'layerdir1/' ]
	[ "${lines[1]}" = 'layerdir3/' ]
	[ "${lines[2]}" = 'layerdir3/layer3subdir/' ]
	[ "${lines[3]}" = 'layerdir3/layer3subdir/layer3subsubdir/' ]
	[ "${lines[4]}" = 'layerdir4/' ]
	[ "${lines[5]}" = 'layerdir4/layer1subdir/' ]
	[ "${lines[6]}" = 'layerdir7/' ]
	[ "${lines[7]}" = 'layerdir7/layer1file4' ]
	[ "${lines[8]}" = 'layerfile1' ]
}
