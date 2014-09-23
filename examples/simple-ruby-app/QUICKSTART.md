Quickstart
----------
This section describes how you can quickly test that your OpenShift environment is setup properly, and allows you to watch the flow of an application creation and build in OpenShift.

To do this, run:

        $ ./run.sh

This will:

1. Launch the OpenShift server
 * Logs are available in logs/openshift.log
 
2. Create a private Docker registry running in OpenShift

3. Define a build configuration for the application

4. Trigger a new build of the application

5. Submit the template/template.json for parameterization

6. Submit the config json to OpenShift for creation

7. Confirm the application is created/accessible via curl

To reset your system after running this example, you can run:
        
    $ ./cleanup.sh
        
This will stop the openshift process, remove the etcd storage, and kill all docker containers running on your host system.  (**Use with caution!**   Docker containers unrelated to openshift will also be killed by this script)
