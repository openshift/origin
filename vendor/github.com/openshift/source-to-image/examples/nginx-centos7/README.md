
# Creating a basic S2I builder image  

## Getting started  

### Files and Directories  
| File                   | Required? | Description                                                  |
|------------------------|-----------|--------------------------------------------------------------|
| Dockerfile             | Yes       | Defines the base builder image                               |
| s2i/bin/assemble       | Yes       | Script that builds the application                           |
| s2i/bin/usage          | No        | Script that prints the usage of the builder                  |
| s2i/bin/run            | Yes       | Script that runs the application                             |
| s2i/bin/save-artifacts | No        | Script for incremental builds that saves the built artifacts |
| test/run               | No        | Test script for the builder image                            |
| test/test-app          | Yes       | Test application source code                                 |

#### Dockerfile
Create a *Dockerfile* that installs all of the necessary tools and libraries that are needed to build and run our application.  This file will also handle copying the s2i scripts into the created image.

#### S2I scripts

##### assemble
Create an *assemble* script that will build our application, e.g.:
- build python modules
- bundle install ruby gems
- setup application specific configuration

The script can also specify a way to restore any saved artifacts from the previous image.   

##### run
Create a *run* script that will start the application. 

##### save-artifacts (optional)
Create a *save-artifacts* script which allows a new build to reuse content from a previous version of the application image.

##### usage (optional) 
Create a *usage* script that will print out instructions on how to use the image.

##### Make the scripts executable 
Make sure that all of the scripts are executable by running *chmod +x s2i/bin/**

#### Create the builder image
The following command will create a builder image named nginx-centos7 based on the Dockerfile that was created previously.
```
docker build -t nginx-centos7 .
```
The builder image can also be created by using the *make* command since a *Makefile* is included.

Once image has finished building, the command *s2i usage nginx-centos7* will print out the help info that was defined in the *usage* script.

#### Testing the builder image
The builder image can be tested using the following commands:
```
docker build -t nginx-centos7-candidate .
IMAGE_NAME=nginx-centos7-candidate test/run
```
The builder image can also be tested by using the *make test* command since a *Makefile* is included.

#### Creating the application image
The application image combines the builder image with your applications source code, which is served using whatever application is installed via the *Dockerfile*, compiled using the *assemble* script, and run using the *run* script.
The following command will create the application image:
```
s2i build test/test-app nginx-centos7 nginx-centos7-app
---> Building and installing application from source...
```
Using the logic defined in the *assemble* script, s2i will now create an application image using the builder image as a base and including the source code from the test/test-app directory. 

#### Running the application image
Running the application image is as simple as invoking the docker run command:
```
docker run -d -p 8080:8080 nginx-centos7-app
```
The application, which consists of a simple static web page, should now be accessible at  [http://localhost:8080](http://localhost:8080).

#### Using the saved artifacts script
Rebuilding the application using the saved artifacts can be accomplished using the following command:
```
s2i build --incremental=true test/test-app nginx-centos7 nginx-app
---> Restoring build artifacts...
---> Building and installing application from source...
```
This will run the *save-artifacts* script which includes the custom code to backup the currently running application source, rebuild the application image, and then re-deploy the previously saved source using the *assemble* script.

#### Configuring nginx
It is possible to configure nginx server itself via this s2i builder image. To do so, simply provide an `nginx.conf` file in the root directory of your application. The `s2i/bin/assemble` will find this configuration file and make nginx use it. See the example configuration file in [test/test-app-redirect/](test/test-app-redirect/) directory. You can build and run it by using these commands:

```
s2i build test/test-app-redirect nginx-centos7 nginx-centos7-redirect
docker run -d -p 8080:8080 nginx-centos7-redirect
```

Going to [http://localhost:8080](http://localhost:8080) should now redirect you to [https://openshift.com](https://openshift.com).

More generally, it is possible to configure any kind of service using s2i, assuming that the provided `assemble` and/or `run` scripts support this and can recognize and use provided configuration files. It's also possible, assuming the s2i image supports it, to provide scripts that get executed by the s2i `run` script to further customize the functionality. Some more advanced examples of this can be found at https://github.com/sclorg/mongodb-container and https://github.com/sclorg/mariadb-container/.
