Quickstart
----------
This section describes how you can quickly test that your OpenShift environment is setup properly, and allows you to watch the flow of an application creation and build in OpenShift.

To do this, run:

        $ ./run.sh

This will:

1. Launch the openshift server
 * Logs are available in logs/openshift.log
 
2. Submit the template/template.json for parameterization

3. Store the resulting config json in processed/template.processed.json

4. Submit the config json to openshift for creation

5. Confirm the application is created/accessible via curl

6. Trigger a new build of the application

7. Show the new docker image created on your local system as a result of the build
 * Normally the next step would be to push this image to a docker registry and then create a new openshift application based on it, or cause the existing application to be redeployed with the new image.  See the next section to exercise this flow.

To reset your system after running this example, you can run:
        
    $ ./cleanup.sh
        
This will stop the openshift process, remove the etcd storage, and kill all docker containers running on your host system.  (**Use with caution!**   Docker containers unrelated to openshift will also be killed by this script)
