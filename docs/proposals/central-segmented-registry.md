# Central Segmented Registry

## Introduction and Use Case

Customers have expressed a need for multiple OpenShift deployments to be able to reference a central registry. They wish to deploy this registry globally to serve a distributed workforce.

Customers have also asked for recommendations for how to segment this registry so images are separated for different *application* lifecycle environments (e.g. dev, test, prod). A common but unsatisfactory approach is to prefix project namespaces with these environment labels. The [Image Promotion](image-promotion.md#areas-of-improvement) proposal acknowledges gaps in this approach.

Today, OpenShift deployments support a single integrated registry, referenced as service **docker-registry** in the **default** namespace. All other registries are "external". This proposal suggests a way to achieve more flexibility in referencing registries.

## Goals

* **Consistent Image Reference** We want users to consistently reference the same image name across all of their clusters as part of a promotion workflow. We want to do this through controlling environment assignment at the cluster level. In other words, the production cluster should only have access to images in the production registry.
* **External Registry Abstraction** We also want users to be able to reference "external" registries as an integrated service abstraction, regardless of the actual cluster the registry is hosted.
* **Optional with Backward Compatibility** This proposes significant complexity. Using it should be optional. Administrators and users who do not require this feature should not have to do anything differently.
* **On-Premise** This proposal is focused on solving issues unique to private, on-premise users of OpenShift. Untrusted, multi-tenant use is not in scope.

## Central Registry Support

For the purposes of discussion, a "central registry" is a simply a dedicated OpenShift cluster for serving images. As described below, this "registry cluster" hosts one or more registry deployments to support environment segmentation. The registry cluster may serve one or many OpenShift clusters.

## Segmenting Registries

The OpenShift registry is reference by `<project>/<registry-service>`. The default registry deployment is `default/docker-registry`. Multiple registries may be deployed on the same cluster using unique project namespaces. Each registry is addressed using a unique route.

### Managing Images

ImageStreamTags are annotated (annotations or labels?) with a list of approved registry environments. Users may promote (or demote) images by managing the environment whitelist. For backward compatibility, the default environment is "default". For example:

        kind: ImageStreamTag
          annotations:
            environment:
              - test
              - uat

To promote to the production environment, a user or process adds an annotation to the ImageStreamTag object. This action would trigger a job to manage ImageStream metadata, registry and storage so the image could be served from the desired registry environment.

        kind: ImageStreamTag
          annotations:
            environment:
              - test
              - uat
              - prod

### Managing the Registries

The central registry cluster may have one or many registries deployed. Each registry is deployed into a project matching the environment name (e.g. test, uat, prod). For example:

        $ oc new-project test
        $ oadm registry --namespace test
        $ oc expose service docker-registry --namespace test

The exposed route provides a unique endpoint for referencing the registry.

Each registry has its own storage backend.

### Managing the Clusters

The central registry is referenced using external service abstraction. Instead of deploying a registry on each cluster, Service and Route objects are created in the "default" project namespace on each cluster.

        - kind: Service
          apiVersion v1
          metadata:
            name: docker-registry
          spec:
            ports:
              - name: docker-registry
                protocol: TCP
                port: 443
                targetPort: 443
                nodePort: 0
          selector: {}
        - kind: Route
          apiVersion: v1
          metadata:
            labels:
              app: docker-registry
            name: docker-registry
          spec:
            tls:
              ...
            to:
              kind: Service
              name: docker-registry
          status: {}

An Endpoint object created in the "default" project namespace on each cluster. In this example, the test OpenShift cluster references the test registry instance in the central registry cluster.

        kind: Endpoints
        apiVersion: v1
        metadata:
          name: docker-registry
        subsets:
          - addresses:
              - IP: docker-registry-test.apps.example.com
            ports:
              - port: 443
                name: docker-registry

### Managing Authorization

Cross-cluster authorization is a significant challenge. Automating or synchronizing cross-cluster authorization is ultimately desired. Until that can be achieved, manual processes may need to bridge the gap.

**Deployments**

Initially, a cluster administrator configures a service account with pull access to the appropriate registry environment so deployments "just work". This service account would have access to *all images* in the central registry irregardless of the environment. We are relying on each cluster registry service object to prevent pulling images from unintended environments. Additional engineering work would be required if restricting pulling images is required.

**Builds**

Build and push authorization is very important. Certain users or projects on certain clusters need access to certain registries. Initially, users could retrieve a "builder" service account token from the registry cluster for their namespace and create a *.dockercfg* build secret. This entails managing namespaces on two clusters and is not ideal.

For organizations that maintain separate build clusters, the registry cluster could be part of this infrastructure cluster. This solves several authorization and namespace challenges.

**Image Scans**

As we look towards implementing integrated image scanning, the central registry cluster is a good candidate for running scanning workloads that check for vulnerabilities at the registry.

## Background

### Actions referencing registries in some way

* deploy -> authenticated pull from registry
* build -> authenticated push to registry
* tag -> updating image metadata, importing image

### Image Objects that reference registry in some way [FIXME]

#### Images

* ImageStreamImage
* ImageStreamMapping
* ImageStreamTag
* ImageSource

#### Other Objects

* BuildConfigs
* DeploymentConfigs

## Questions and Challenges

1. Should registry abstraction be a first-class object? How might this help?
1. Can we delete a string from an imageStreamTag list?
1. How can we make manipulating images from the cluster (not the registry cluster) feel native? How might image pull-through help?
