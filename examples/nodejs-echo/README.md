Node.js `echo` Service on OpenShift!
-----------------

This example will serve an http response of various "machine" stats from the "machine" the Node.js app is running on to [http://host:8080](http://host:8080).

### OpenShift setup ###

I just used the Docker all-in-one launch as described in the [origins project](https://github.com/openshift/origins).

### The project ###

If you don't have a project setup all ready, go ahead and take care of that

        $ osc new-project nodejs-echo --display-name="nodejs-echo" --description="Sample Node.js app"

That's it, project has been created.  Though it would probably be good to set your current project to this, such as:

        $ osc project nodejs-echo

### The app ###

Now let's pull in the app source code from [GitHub repo](https://github.com/openshift/nodejs-example) (fork if you like)

#### create ####

        $ osc new-app https://github.com/openshift/nodejs-example
        
That should be it, `new-app` will take care of creating the right build configuration, deployment configuration and service definition.  Next you'll be able to kick off the build.

Note, you can follow along with the web console (located at https://ip-address:8443/console) to see what new resources have been created and watch the progress of the build and deployment.

#### build ####

        $ osc start-build nodejs-echo --follow

You can alternatively leave off `--follow` and use `osc build-logs nodejs-echo-n` where n is the number of the build (output of start-build).

#### deploy #### 

happens automatically, to monitor its status either watch the web console or `osc get pods` to see when the pod is up.  Another helpful command is

        $ osc status

This will help indicate what IP address the service is running, the default port for it to deploy at is 8080.  

#### enjoy ####

Run/test our app by simply doing an HTTP GET request

        $ curl ip-address:8080

#### update ####

Assuming you used the URL of your own forked report, we can easily push changes to that hosted repo and simply repeat the steps above to build (this is obviously just demonstrating the manually kicking off of builds) which will trigger the new built image to be deployed.
