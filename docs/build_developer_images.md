# OpenShift Router Images

There are situations in which you only need to recreate 
only the router image instead of all the images.


## Build Origin

### Clone the origin repo (or your own fork)

git clone https://github.com/openshift/origin.git

cd origin

**In order to speed up the build time we just build for one platform:**

export OS_ONLY_BUILD_PLATFORMS="linux/amd64"

### Build Openshift origin and router images

OS_RELEASE=n hack/build-go.sh

cp _output/local/bin/linux/amd64/openshift images/origin/bin/

docker build -t openshift/origin images/origin

docker build -t openshift/origin-haproxy-router images/router/haproxy/

## Optional: Use local registry


docker run -d -p 5000:5000 --restart=always --name registry registry:2

### In /etc/sysconfig/docker modify:

INSECURE_REGISTRY='--insecure-registry <hosts IP>:5000'

or

echo "INSECURE_REGISTRY='--insecure-registry <host IP>:5000'"  >> /etc/sysconfig/docker

### Restart docker

systemctl restart docker

### Push images

docker tag openshift/origin localhost:5000/openshift/origin
docker push localhost:5000/openshift/origin

docker tag origin-haproxy-router localhost:5000/openshift/origin-haproxy-router
docker push localhost:5000/openshift/origin-haproxy-router

## NOTES:

There are some other tasks that could be performed in order to use the image.

### Example: Restart router

If the router is already running we can delete it so it will restart using the
new image:

docker rm -f $(docker ps | grep haproxy-router | cut -d' ' -f 1)

