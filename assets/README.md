OpenShift 3 Static Assets
=========================
The static assets for OpenShift v3.  This includes the web management console.

Contributing
------------

#### Getting started
1. Install [Nodejs](http://nodejs.org/) and [npm](https://www.npmjs.org/)
2. Install [grunt-cli](http://gruntjs.com/installing-grunt) and [bower](http://bower.io/) by running `npm install -g grunt-cli bower` (may need to be run with sudo)
3. From the `assets` directory, run the following commands:
    
    `npm install` (Install the project's dev dependencies)
    
    `bower install` (Install the project's UI dependencies)
    
    `grunt serve` (Launch the console and start watching for asset changes)

    Note: If you see an ENOSPC error running `grunt serve`, you may need to increase the number of files your user can watch by running this command:
    
    ```
    echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf && sudo sysctl -p
    ```

#### Before opening a pull request
1. Run the test suite with `grunt test`
2. Rebase and squash changes to a single commit

#### Production builds
1. From the `assets` directory, run `grunt build`
2. Make sure the go-bindata binary is up to date by running:

    ```
    cd Godeps/_workspace/src/github.com/jteeuwen/go-bindata

    GOPATH=$GOPATH/src/github.com/openshift/origin/Godeps/_workspace go install ./...

    ```

3. From the root of the origin repo, run:

    ```
    Godeps/_workspace/bin/go-bindata -prefix "assets/dist" -pkg "assets" -o "pkg/assets/bindata.go" assets/dist/...

    hack/build-go.sh
    ```

The assets served by the OpenShift all-in-one server will now be up to date. By default the assets are served from [http://localhost:8091](http://localhost:8091)