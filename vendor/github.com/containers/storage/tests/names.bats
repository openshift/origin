#!/usr/bin/env bats

load helpers

# Helper function to scan the list of names of an item for a particular value.
check-for-name() {
	name="$1"
	shift
	run storage --debug=false get-names "$@"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$name" ]]
}

@test "names at creation: layers" {
	# Create a layer with no name.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer=$output

	# Verify that the layer exists and can be found by ID.
	run storage exists -l $lowerlayer
	[ "$status" -eq 0 ]
	# Verify that these three names don't appear to be assigned.
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -ne 0 ]
	run storage exists -l barlayer
	[ "$status" -ne 0 ]

	# Create a new layer and give it two of the above-mentioned names.
	run storage --debug=false create-layer -n foolayer -n barlayer $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer=${output%%	*}

	# Verify that the new layer exists and can be found by its ID.
	run storage exists -l $upperlayer
	[ "$status" -eq 0 ]
	# Verify that two of the names we checked earlier are now assigned, and to the new layer.
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -eq 0 ]
	run storage exists -l barlayer
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $upperlayer
	[ "$status" -ne 0 ]
	run check-for-name foolayer $upperlayer
	[ "$status" -eq 0 ]
	run check-for-name barlayer $upperlayer
	[ "$status" -eq 0 ]
}

@test "add-names: layers" {
	# Create a layer with no name.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer=$output

	# Verify that we can find the layer by its ID.
	run storage exists -l $lowerlayer
	[ "$status" -eq 0 ]

	# Check that these three names are not currently assigned.
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -ne 0 ]
	run storage exists -l barlayer
	[ "$status" -ne 0 ]

	# Create a new layer with names.
	run storage --debug=false create-layer -n foolayer -n barlayer $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer=${output%%	*}

	# Add names to the new layer.
	run storage add-names -n newlayer -n otherlayer $upperlayer
	[ "$status" -eq 0 ]

	# Verify that we can find the new layer by its ID.
	run storage exists -l $upperlayer
	[ "$status" -eq 0 ]
	# Verify that the name we didn't assign is still unassigned, and that the two names we
	# started with, along with the two we added, are assigned, to the new layer.
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -eq 0 ]
	run storage exists -l barlayer
	[ "$status" -eq 0 ]
	run storage exists -l newlayer
	[ "$status" -eq 0 ]
	run storage exists -l otherlayer
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $upperlayer
	[ "$status" -ne 0 ]
	run check-for-name foolayer $upperlayer
	[ "$status" -eq 0 ]
	run check-for-name barlayer $upperlayer
	[ "$status" -eq 0 ]
	run check-for-name newlayer $upperlayer
	[ "$status" -eq 0 ]
	run check-for-name otherlayer $upperlayer
	[ "$status" -eq 0 ]
}

@test "set-names: layers" {
	# Create a layer with no name.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer=$output

	# Verify that we can find the layer by its ID.
	run storage exists -l $lowerlayer
	[ "$status" -eq 0 ]

	# Check that these three names are not currently assigned.
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -ne 0 ]
	run storage exists -l barlayer
	[ "$status" -ne 0 ]

	# Create a new layer with two names.
	run storage --debug=false create-layer -n foolayer -n barlayer $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer=${output%%	*}

	# Assign a list of two names to the layer, which should remove its other names.
	run storage set-names -n newlayer -n otherlayer $upperlayer
	[ "$status" -eq 0 ]

	# Check that the old names are not assigned at all, but the new names are, to it.
	run storage exists -l $upperlayer
	[ "$status" -eq 0 ]
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -ne 0 ]
	run storage exists -l barlayer
	[ "$status" -ne 0 ]
	run storage exists -l newlayer
	[ "$status" -eq 0 ]
	run storage exists -l otherlayer
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $upperlayer
	[ "$status" -ne 0 ]
	run check-for-name foolayer $upperlayer
	[ "$status" -ne 0 ]
	run check-for-name barlayer $upperlayer
	[ "$status" -ne 0 ]
	run check-for-name newlayer $upperlayer
	[ "$status" -eq 0 ]
	run check-for-name otherlayer $upperlayer
	[ "$status" -eq 0 ]
}

@test "move-names: layers" {
	# Create a layer with no name.
	run storage --debug=false create-layer -n foolayer -n barlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer=${output%%	*}

	# Verify that we can find the layer by its ID.
	run storage exists -l $lowerlayer
	[ "$status" -eq 0 ]

	# Check that these three names are not currently assigned.
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -eq 0 ]
	run storage exists -l barlayer
	[ "$status" -eq 0 ]

	# Create another layer with no names.
	run storage --debug=false create-layer $lowerlayer
	[ "$status" -eq 0 ]
	upperlayer=${output%%	*}

	# Set names on that new layer, which should remove the names from the old one.
	run storage set-names -n foolayer -n barlayer $upperlayer
	[ "$status" -eq 0 ]

	# Verify that we can find the layer by its ID, and that the two names exist.
	run storage exists -l $upperlayer
	[ "$status" -eq 0 ]
	run storage exists -l no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -l foolayer
	[ "$status" -eq 0 ]
	run storage exists -l barlayer
	[ "$status" -eq 0 ]

	# Check that the names are attached to the new layer and not the old one.
	run check-for-name foolayer $lowerlayer
	[ "$status" -ne 0 ]
	run check-for-name barlayer $lowerlayer
	[ "$status" -ne 0 ]
	run check-for-name foolayer $upperlayer
	[ "$status" -eq 0 ]
	run check-for-name barlayer $upperlayer
	[ "$status" -eq 0 ]
}

@test "names at creation: images" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image with names that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Check that we can find that image by ID and by its names.
	run storage exists -i $image
	[ "$status" -eq 0 ]
	run storage exists -i no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -i fooimage
	[ "$status" -eq 0 ]
	run storage exists -i barimage
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $image
	[ "$status" -ne 0 ]
	run check-for-name fooimage $image
	[ "$status" -eq 0 ]
	run check-for-name barimage $image
	[ "$status" -eq 0 ]
}

@test "add-names: images" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image with names that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Check that we can find that image by ID and by its names.
	run storage exists -i $image
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $image
	[ "$status" -ne 0 ]
	run check-for-name fooimage $image
	[ "$status" -eq 0 ]
	run check-for-name barimage $image
	[ "$status" -eq 0 ]

	# Add two names to the image.
	run storage add-names -n newimage -n otherimage $image
	[ "$status" -eq 0 ]

	# Check that all of the names are now assigned.
	run storage exists -i no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -i fooimage
	[ "$status" -eq 0 ]
	run storage exists -i barimage
	[ "$status" -eq 0 ]
	run storage exists -i newimage
	[ "$status" -eq 0 ]
	run storage exists -i otherimage
	[ "$status" -eq 0 ]

	# Check that all of the names are now assigned to this image.
	run check-for-name no-such-thing-as-this-name $image
	[ "$status" -ne 0 ]
	run check-for-name fooimage $image
	[ "$status" -eq 0 ]
	run check-for-name barimage $image
	[ "$status" -eq 0 ]
	run check-for-name newimage $image
	[ "$status" -eq 0 ]
	run check-for-name otherimage $image
	[ "$status" -eq 0 ]
}

@test "set-names: images" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image with names that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Check that we can find that image by ID and by its names.
	run storage exists -i $image
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $image
	[ "$status" -ne 0 ]
	run check-for-name fooimage $image
	[ "$status" -eq 0 ]
	run check-for-name barimage $image
	[ "$status" -eq 0 ]

	# Set the names for the image to two new names.
	run storage set-names -n newimage -n otherimage $image
	[ "$status" -eq 0 ]

	# Check that the two new names are the only ones assigned.
	run storage exists -i no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -i fooimage
	[ "$status" -ne 0 ]
	run storage exists -i barimage
	[ "$status" -ne 0 ]
	run storage exists -i newimage
	[ "$status" -eq 0 ]
	run storage exists -i otherimage
	[ "$status" -eq 0 ]

	# Check that the two new names are the only ones on this image.
	run check-for-name no-such-thing-as-this-name $image
	[ "$status" -ne 0 ]
	run check-for-name fooimage $image
	[ "$status" -ne 0 ]
	run check-for-name barimage $image
	[ "$status" -ne 0 ]
	run check-for-name newimage $image
	[ "$status" -eq 0 ]
	run check-for-name otherimage $image
	[ "$status" -eq 0 ]
}

@test "move-names: images" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image with names that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	firstimage=${output%%	*}

	# Create another image with no names.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Check that we can find the first image by ID and by its names.
	run storage exists -i $firstimage
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $firstimage
	[ "$status" -ne 0 ]
	run check-for-name fooimage $firstimage
	[ "$status" -eq 0 ]
	run check-for-name barimage $firstimage
	[ "$status" -eq 0 ]

	# Set a name list on the new image that includes the names of the old one.
	run storage set-names -n fooimage -n barimage -n newimage -n otherimage $image
	[ "$status" -eq 0 ]

	# Check that all of the names are assigned.
	run storage exists -i no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -i fooimage
	[ "$status" -eq 0 ]
	run storage exists -i barimage
	[ "$status" -eq 0 ]
	run storage exists -i newimage
	[ "$status" -eq 0 ]
	run storage exists -i otherimage
	[ "$status" -eq 0 ]

	# Check that all of the names are assigned to the new image.
	run check-for-name no-such-thing-as-this-name $image
	[ "$status" -ne 0 ]
	run check-for-name fooimage $image
	[ "$status" -eq 0 ]
	run check-for-name barimage $image
	[ "$status" -eq 0 ]
	run check-for-name newimage $image
	[ "$status" -eq 0 ]
	run check-for-name otherimage $image
	[ "$status" -eq 0 ]
}

@test "names at creation: containers" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image that uses that layer.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container with two names, based on that image.
	run storage --debug=false create-container -n foocontainer -n barcontainer $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Check that we can find the container using either its ID or names.
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run storage exists -c no-such-thing-as-this-name
	[ "$status" -ne 0 ]
	run storage exists -c foocontainer
	[ "$status" -eq 0 ]
	run storage exists -c barcontainer
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $container
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -eq 0 ]
	run check-for-name barcontainer $container
	[ "$status" -eq 0 ]
}

@test "add-names: containers" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container with two names, based on that image.
	run storage --debug=false create-container -n foocontainer -n barcontainer $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Check that we can find the container using either its ID or names.
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $container
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -eq 0 ]
	run check-for-name barcontainer $container
	[ "$status" -eq 0 ]

	# Add two names to the container.
	run storage add-names -n newcontainer -n othercontainer $container
	[ "$status" -eq 0 ]

	# Verify that all of those names are assigned to the container.
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $container
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -eq 0 ]
	run check-for-name barcontainer $container
	[ "$status" -eq 0 ]
	run check-for-name newcontainer $container
	[ "$status" -eq 0 ]
	run check-for-name othercontainer $container
	[ "$status" -eq 0 ]
}

@test "set-names: containers" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container with two names, based on that image.
	run storage --debug=false create-container -n foocontainer -n barcontainer $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Check that we can find the container using either its ID or names.
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $container
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -eq 0 ]
	run check-for-name barcontainer $container
	[ "$status" -eq 0 ]

	# Set the list of names for the container to just these two values.
	run storage set-names -n newcontainer -n othercontainer $container
	[ "$status" -eq 0 ]

	# Check that these are the only two names attached to the container.
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $container
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -ne 0 ]
	run check-for-name barcontainer $container
	[ "$status" -ne 0 ]
	run check-for-name newcontainer $container
	[ "$status" -eq 0 ]
	run check-for-name othercontainer $container
	[ "$status" -eq 0 ]
}

@test "move-names: containers" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image that uses that layer.
	run storage --debug=false create-image -n fooimage -n barimage $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container with two names, based on that image.
	run storage --debug=false create-container -n foocontainer -n barcontainer $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	firstcontainer=${output%%	*}

	# Create another container with two different names, based on that image.
	run storage --debug=false create-container -n newcontainer -n othercontainer $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Check that we can access both containers by ID, and that they have the right names.
	run storage exists -c $firstcontainer
	[ "$status" -eq 0 ]
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $firstcontainer
	[ "$status" -eq 0 ]
	run check-for-name barcontainer $firstcontainer
	[ "$status" -eq 0 ]
	run check-for-name newcontainer $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name othercontainer $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -ne 0 ]
	run check-for-name barcontainer $container
	[ "$status" -ne 0 ]
	run check-for-name newcontainer $container
	[ "$status" -eq 0 ]
	run check-for-name othercontainer $container
	[ "$status" -eq 0 ]

	# Set the names on the new container to the names we gave the old one.
	run storage set-names -n foocontainer -n barcontainer $container
	[ "$status" -eq 0 ]

	# Check that the containers can still be found, and that the names are correctly set.
	run storage exists -c $firstcontainer
	[ "$status" -eq 0 ]
	run storage exists -c $container
	[ "$status" -eq 0 ]
	run check-for-name no-such-thing-as-this-name $container
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name barcontainer $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name newcontainer $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name othercontainer $firstcontainer
	[ "$status" -ne 0 ]
	run check-for-name foocontainer $container
	[ "$status" -eq 0 ]
	run check-for-name barcontainer $container
	[ "$status" -eq 0 ]
	run check-for-name newcontainer $container
	[ "$status" -ne 0 ]
	run check-for-name othercontainer $container
	[ "$status" -ne 0 ]
}
