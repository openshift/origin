#!/usr/bin/env bats

load helpers

IMAGE=kubernetes/pause
SIGNED_IMAGE=registry.access.redhat.com/rhel7-atomic:latest
UNSIGNED_IMAGE=docker.io/library/hello-world:latest

function teardown() {
	cleanup_test
}

@test "run container in pod with image ID" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	sed -e "s/%VALUE%/$REDIS_IMAGEID/g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imageid.json
	run crictl create "$pod_id" "$TESTDIR"/ctr_by_imageid.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "container status when created by image ID" {
	start_crio

	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	sed -e "s/%VALUE%/$REDIS_IMAGEID/g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imageid.json

	run crictl create "$pod_id" "$TESTDIR"/ctr_by_imageid.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl inspect "$ctr_id" --output yaml
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "image: docker.io/library/redis:alpine" ]]
	[[ "$output" =~ "imageRef: $REDIS_IMAGEREF" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "container status when created by image tagged reference" {
	start_crio

	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	sed -e "s/%VALUE%/redis:alpine/g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imagetag.json

	run crictl create "$pod_id" "$TESTDIR"/ctr_by_imagetag.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl inspect "$ctr_id" --output yaml
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "image: docker.io/library/redis:alpine" ]]
	[[ "$output" =~ "imageRef: $REDIS_IMAGEREF" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "container status when created by image canonical reference" {
	start_crio

	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	sed -e "s|%VALUE%|$REDIS_IMAGEREF|g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imageref.json

	run crictl create "$pod_id" "$TESTDIR"/ctr_by_imageref.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl inspect "$ctr_id" --output yaml
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "image: docker.io/library/redis:alpine" ]]
	[[ "$output" =~ "imageRef: $REDIS_IMAGEREF" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "image pull and list" {
	start_crio "" "" --no-pause-image
	run crictl pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl images --quiet "$IMAGE"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	imageid="$output"

	run crictl images @"$imageid"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$IMAGE" ]]

	run crictl images --quiet "$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	cleanup_images
	stop_crio
}

@test "image pull with signature" {
	start_crio "" "" --no-pause-image
	run crictl pull "$SIGNED_IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_images
	stop_crio
}

@test "image pull without signature" {
	start_crio "" "" --no-pause-image
	run crictl image pull "$UNSIGNED_IMAGE"
	echo "$output"
	[ "$status" -ne 0 ]
	cleanup_images
	stop_crio
}

@test "image pull and list by tag and ID" {
	start_crio "" "" --no-pause-image
	run crictl pull "$IMAGE:go"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl images --quiet "$IMAGE:go"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	imageid="$output"

	run crictl images --quiet @"$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	run crictl images --quiet "$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	cleanup_images
	stop_crio
}

@test "image pull and list by digest and ID" {
	start_crio "" "" --no-pause-image
	run crictl pull nginx@sha256:33eb1ed1e802d4f71e52421f56af028cdf12bb3bfff5affeaf5bf0e328ffa1bc
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl images --quiet nginx@sha256:33eb1ed1e802d4f71e52421f56af028cdf12bb3bfff5affeaf5bf0e328ffa1bc
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	imageid="$output"

	run crictl images --quiet @"$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	run crictl images --quiet "$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	cleanup_images
	stop_crio
}

@test "image list with filter" {
	start_crio "" "" --no-pause-image
	run crictl pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl images --quiet "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		run crictl rmi "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	run crictl images --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
		status=1
	done
	cleanup_images
	stop_crio
}

@test "image list/remove" {
	start_crio "" "" --no-pause-image
	run crictl pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl images --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		run crictl rmi "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	run crictl images --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
		status=1
	done
	cleanup_images
	stop_crio
}

@test "image status/remove" {
	start_crio "" "" --no-pause-image
	run crictl pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl images --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		run crictl images -v "$id"
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		run crictl rmi "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	run crictl images --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
		status=1
	done
	cleanup_images
	stop_crio
}
