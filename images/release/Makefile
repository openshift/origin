# Update the stub RPMs we use in the release images for Golang
build:
	./fake-rpm.sh golang-1.7
	./fake-rpm.sh golang-1.8
	./fake-rpm.sh golang-1.9
.PHONY: build

build-images:
	imagebuilder -t openshift/origin-release:golang-1.7 golang-1.7
	imagebuilder -t openshift/origin-release:golang-1.8 golang-1.7
	imagebuilder -t openshift/origin-release:golang-1.9 golang-1.7
.PHONY: build