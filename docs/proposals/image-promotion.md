# Image Promotion

## Abstract

This proposal describes the best practices and defines patterns for promoting Docker
images between different stages (e.g. from *development* to *production*).
This proposal also describes a way how to configure OpenShift to serve as
a Continuous Delivery tool.

## Constraints and Assumptions

This document describes the following ways to promote images:

* **Based on human intervention**
  * using the `oc` and `docker` commands manually
* **Automated by the OpenShift platform**
  * using enhanced *DeploymentConfig* [lifecycle hooks](https://docs.openshift.org/latest/dev_guide/deployments/deployment_strategies.html#lifecycle-hooks)
  * using [ImageChangeTrigger](https://docs.openshift.org/latest/dev_guide/deployments/basic_deployment_operations.html#image-change-trigger)
  * using [Projects](https://docs.openshift.org/latest/dev_guide/projects.html) for different "stages"
* **Based on external Continuous Delivery tool**
  * [Jenkins](https://jenkins-ci.org)
  * [go.cd](http://go.cd)
  * [snap](http://snap-ci.com)

Docker images can be promoted (and the use-cases are valid for):

* **Within a single Project**
* **Between multiple Projects**
* **Between multiple Clusters**

The Docker image promotion can be done using the Docker [image
tags](https://docs.docker.com/engine/reference/commandline/tag/)
or Docker image [labels](https://docs.docker.com/engine/userguide/labels-custom-metadata/).
OpenShift also provides more features to add metadata, such as annotations or labels.
Using them might allow users to develop complex conditional promotion scenarios, but
this document does not describe them.

In some cases, promoting just Docker images might not be enough and you want to promote
the entire deployment or a project as a single unit of promotion. These scenarios
are also not covered by this document.

## Matrix

* **Promotion based on human intervention**
  * By creating ImageStreamTag manually by using the `oc tag` command. For
      example:
    - `oc tag application/image:@sha256:02c104b application/image:qa-ready`
    This will tag the `application/image` with SHA256 tag `02c104b` as `qa-ready`.
  * By importing the Docker image manually using OpenShift CLI after the image
    pass the verification process successfully. For example:
    - `oc import-image application --from=external:application`
    This will import the `external:application` image available in 'external'
    registry to the `application` image stream.
  * By moving the ImageStream from one project to another
    - `oc export is -l name=application | oc create -n stage -f -`
  * By using Docker CLI manually
    - `docker tag application/image qa-registry:application/image:qa-ready`
    - `docker push qa-registry:application/image:qa-ready`
* **Automatic promotion**
  * By using CLI
    - Create cron job that tags the image when the image with watched tag is
      available in registry using the `oc tag` command. This means that the cron
      job is using polling mechanism to determine if a new version of the image
      is available in the internal registry.
  * By using OpenShift internal mechanism
    - Use DeploymentConfig lifecycle hooks to do `oc tag` as a post-deployment
      step.
    - Use DeploymentConfig lifecycle hooks to do `docker tag` and `docker push`
    - Use DeploymentConfig to call remote OpenShift API to do tagging
    - Use DeploymentConfig strategy that deploys a 'testing' image and calls
      the OpenShift API directly from the 'testing' container based on the
      verification results.
  * By using an external continuous-delivery tools
    - Use a Jenkins job to invoke oc tag at the end of a successful build job -
      perhaps one that runs unit tests on a builder image created by another team
    - Use Jenkins job to do `docker tag` and `docker push`
* **Semi-automated (require human intervention for some steps)**
  * By using OpenShift or an external continuous-delivery tool
    - Use Jenkins job to produce and verify the new Docker image, but require human
      intervention to import the image to OpenShift using `oc import` command
    - Use Jenkins job to do `oc tag` but have DeploymentConfig independent of
      the image change (require manual intervention to roll out a new deployment)
    - Use DeploymentConfig lifecycle hook to do `oc tag` but require human to start a new deployment

## Use Cases

### Automatic promotion

As a developer of Rails application, when I update the source code and push it to a remote
source code repository, I want to have my changes tested before deploying them to production.
I also want to have the Docker image that contains applications with my changes to be promoted
based on the verification results.

#### Example of using Jenkins

As a user I have a Jenkins server running in my project by following the instructions from the [Jenkins example](https://github.com/openshift/origin/blob/master/examples/jenkins/README.md).
Then I create the *frontend-production* DeploymentConfig in my *Prod* project.

This DeploymentConfig has the ImageChangeTrigger set to watch changes in the "prod-ready" ImageStreamTag:

```json
{
  "type": "ImageChange",
  "imageChangeParams": {
    "automatic": true,
    "from": {
      "kind": "ImageStreamTag",
      "name": "origin-ruby-example:prod-ready"
    },
    "containerNames": [
      "frontend"
    ]
  }
}
```

I also have a BuildConfig defined for my application image that does not specify any triggers.

When a change is detected in remote source code repository, the Jenkins job will connect to OpenShift and
start a new build.
Based on the build status, Jenkins job then tag the updated application image as *prod-ready*.
When a new image with this tag is available, the *frontend-production* manages to roll out a new deployment.

#### Example of using OpenShift

As a user I create two DeploymentConfig's, both as part of my "Prod" project:

* *frontend-test*
* *frontend-production*

The "frontend-test" DeploymentConfig has the ImageChangeTrigger set to watch changes
in the *qa-ready* ImageStreamTag. This DeploymentConfig also defines the *post* lifecycle
hook that executes a script in the container deployed and performs additional verification
and tests.

*Example of DeploymentConfig post-lifecycle hook*:
```json
"strategy": {
  "type": "Rolling",
  "rollingParams": {
    "post": {
      "failurePolicy": "Abort",
      "execNewPod": {
        "containerName": "test-container",
        "command": [
          "/opt/app-root/bin/verify-deployment"
        ],
      }
    }
  }
}
```

Based on the result of the `verify-deployment` command, which runs in the container based on the
application image, the command will promote the image by tagging it as *production-ready* (e.g. using
the `oc tag` command). The command might also notify me about the verification failure.

The *frontend-production* DeploymentConfig has the ImageChangeTrigger set to watch changes
in the *prod-ready* ImageStreamTag. When a new Docker image with this tag is available,
it can automatically roll out a new production deployment. In case you don't want to roll out
the *frontend-production* automatically, you can avoid specifying the ImageChangeTrigger and
roll out the new deployment by hand.

#### Example of manual image promotion between two projects

As a user I create two projects: 'stage' and 'prod'. Then I create following
resources inside these projects:

* `stage`:
  * BuildConfig 'sample-app-build'
  * ImageStream 'sample-app'
  * DeploymentConfig 'sample-app'

* `prod`:
  * ImageStream 'sample-app'
  * DeploymentConfig 'sample-app'

Now I start the 'sample-app-build' using:

```console
$ oc start-build sample-app-build -n stage
```

This build will produce the 'sample-app:latest' image. This build can be
automated by GitHub or generic triggers to build after every commit.

The "sample-app" DeploymentConfig that I created in the "stage" project is
configured to trigger a new deployment when the "sample-app:stable" ImageStreamTag
is updated. The reason why I use "sample-app:stable" over "sample-app:latest" is
that I don't want every commit cause automatic redeployment of the "stage".

To tag the "latest" image, we can run the following command, which will result
in the current "sample-app:latest" being automatically deployed in "stage":

```console
$ oc tag stage/sample-app:latest stage/sample-app:stable
Tag sample-app:stable set to stage/sample-app@sha256:<sha>
```

Now we have our application deployed in the "stage" project and we can perform
verification of this deployment, or notify the testers to do it.
Once we are happy with this image, we want to promote it for production:

First, we have to allow the the "prod" service account to pull the image from
the "stage" repository in Docker registry:

```console
$ oc policy add-role-to-user edit system:serviceaccount:stage:default -n prod
```

Then we can tag the image to the "prod" project:

```console
$ oc tag stage/sample-app:stable prod/sample-app:v0.0.1
$ oc tag prod/sample-app:v0.0.1 prod/sample-app:latest
```

Since the DeploymentConfig "sample-app" in "prod" is configured to redeploy when
the ImageStreamTag "sample-app:latest" is updated, this will cause the image to be
deployed in the "prod" project. Also we are making sure that the image we deploy in
"prod" is the same image as we tested in "stage".


### Manual promotion using external Docker registry

As a developer of a Rails application, when I update the source code and OpenShift
build my image I want to promote the image by passing it through QA and operation teams.

#### Example

* The image is passed to a remote QA team by pushing the image to a shared Docker registry
* Remote QA team pull the image and drive the testing and verification
* Remote QA team push the verified image back to the shared Docker registry, tagged as "verified"
* Remote devops team pull the image from shared Docker registry and make it available in "stage" cluster
* Remote devops team promotes the image from "stage" to "production"

### Manual promotion of builder image using external Docker registry

As a platform operator, I want to be able to promote the S2I builder image(s) when a new
version is available in Red Hat Docker registry. I want to perform additional verifications
(iow. deploy to "stage" first) before rolling a new image out to production which causes rebuild
of a thousand applications.

#### Example

* OpenShift engineering tag the `openshift/ruby-22-rhel7` image to internal CI registry
  * `oc import-image openshift/ruby-22-rhel7`
* OpenShift QA team pull the image from the internal CI registry and run tests
* OpenShift QA team sign-off the Docker image when tests pass and push it to "integration" registry
* OpenShift operations team pulls the image from "integration" registry and promote it to "stage" environment
* OpenShift operations team manually promote the image to production

### Semi-automatic promotion and testing of builder images

As a developer of the S2I image, I want to make sure that the changes I propose won't break
the applications that are based on this image, when they will be rebuilt.
For that I want to promote the image as "ok-to-merge", only when the sample application
that use this image pass the verification checks.

#### Example of pull request tester

* OpenShift engineering propose a change to `openshift/ruby-22-rhel7` image
* Jenkins Github plugin detects a new pull request and docker build the new testing image
* Jenkins job connects to a running OpenShift server, that runs the example applications which are based on this image
* Jenkins job imports the updated image to a running OpenShift server and tag it as "test"
* A new deployment is triggered for applications that have ImageChangeTrigger set to watch this image
* As part of the deployment post-lifecycle hook, the applications are verified and the ImageStreamTag is sign-off by the application name
* Jenkins job waits until all applications sign-off the updated image as "verified"
* Jenkins Github plugin send notification to original pull request

#### Example of push_images job

* OpenShift engineering merge change to `openshift/ruby-22-rhel7` repository
* OpenShift server detect the change and execute test suite for given image
* If the image is 'base' image, OpenShift server launch multiple tests for all dependent images
* OpenShift server push the image to internal registry and Docker Hub (see: "Manual promotion of builder image")

## Areas of improvement

The first use-case describes a typical Continuous Delivery example, where the developer uses
both the Jenkins server and the OpenShift to deliver updates to the application.

### Jenkins

* There is no support for enabling the Jenkins when creating a new application
* Users have to manually tweak the Jenkins job parameters to provide informations about the Project/Namespace or the BuildConfig Jenkins should watch.

### Projects

* Having to promote images from between multiple projects (or clusters) has security implication

### Deployments

* The "post" lifecycle hook can be executed only in the image that is being deployed
* The images must have OpenShift CLI tools installed in order to do tagging
* The container must have OpenShift API secrets mounted
* The container must allow to allow promotion in a different project or cluster
* There is no way to say "scale this deployment down" after verification finishes
  (must be done in verification script)

### CLI

* Allow to export the ImageStreamTag together with the Docker image (`oc export-image`)
* Allow to import the ImageStreamTag together with the Docker image (`oc import-image`)
  * This will import the Docker image image to the Docker registry
  * Create ImageStream for the image that is being imported, with all annotations
  * Create ImageStreamTag for the image
