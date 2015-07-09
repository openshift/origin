OpenShift 3 Jenkins Example
=========================
This sample walks through the process of starting up an OpenShift cluster and deploying a Jenkins Pod in it.
It also configures a simple application and then creates a Jenkins job to trigger a build of that application.

The Jenkins job will trigger OpenShift to build+deploy a test version of the application, validate that
the deployment works, and then tag the test version into production.

Steps
-----

1. Follow steps 1-7 from the [sample-app](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md)

    At this point you should be logged in to openshift as a normal user and working with a project named `test`.

2. Add the `edit` role to the `default` service account in the `test` project:

        $ oc policy add-role-to-user edit system:serviceaccount:test:default

    This will allow the service account Jenkins uses to run builds and tag images.

3. Optional:  If you are going to run Jenkins somewhere other than as a deployment within your same project you will need to provide an access token to Jenkins to access your project.

    1. Identify the service account token secret:

            $ oc describe serviceaccount default

        Example output:

            Name:       default
            Labels:     <none>
            Secrets:    {  default-token-uyswp    }
                        {  default-dockercfg-xcr3d    }
            Tokens:     default-token-izv1u
                        default-token-uyswp

        In this case the secret name is `default-token-uyswp`

    2. Retrieve the token from the secret:

            $ oc describe secret <secret name from above> # e.g. default-token-izv1u

        Example output:

            Name:       default-token-izv1u
            Labels:     <none>
            Annotations:    kubernetes.io/service-account.name=default,kubernetes.io/service-account.uid=32f5b661-2a8f-11e5-9528-3c970e3bf0b7
            Type:   kubernetes.io/service-account-token
            Data
            ====
            ca.crt: 1066 bytes
            token:  eyJhbGc..<content cut>....wRA

        Copy the value from the `token` field, it will be used later.

4. Create and deploy the Jenkins service

        $ oc new-app jenkins-ephemeral-template.json
    
    Note the ip and port of the Jenkins service reported by this command, you will need it later.

        $ oc deploy jenkins --latest

    **Note**: This template uses an EmptyDir type volume.  If you want to ensure your jenkins configuration/job information is persisted through pod restarts and deployments, you will want to use a [PersistentVolume](https://docs.openshift.org/latest/admin_guide/persistent_storage_nfs.html) instead.
    
5. Create the sample application configuration

        $ oc new-app application-template.json
 
6. Open the Jenkins service ip:port from step 4 in your browser.  Once it is available, login using username `admin` and password `password`.
   
7. Select the the `OpenShift Sample` job and click `Configure`

8. Optional (if the default value does not resolve to your OpenShift api endpoint): Fill in the `OPENSHIFT_API_URL` value with `https://hostname:port` where hostname/ip and port are for your OpenShift api endpoint.

9. Optional (if not running Jenkins in the same OpenShift deployment as your application project): Fill in the `AUTH_TOKEN` value with the token value retrieved in step 3.

10. Save your changes to the job and click `Build with Parameters` and then `Build`.

11. Watch the job output

   It will trigger an OpenShift build of the application, wait for the build to result in a deployment,
   confirm the new deployment works, and then tag the image for production.  This tagging will trigger
   another deployment, this time creating/updating the production service.

12. Confirm both the test and production services are available by browsing to both services:

        $ oc get services -n test | grep frontend
