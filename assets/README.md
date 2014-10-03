OpenShift 3 Static Assets
=========================
The static assets for OpenShift v3.  This includes the web management console.

Contributing
------------

#### Getting started
1. Install [Nodejs](http://nodejs.org/) and [npm](https://www.npmjs.org/)
2. Install [grunt-cli](http://gruntjs.com/installing-grunt) and [bower](http://bower.io/) by running `npm install -g grunt-cli bower` (may need to be run with sudo)
3. Install [ruby](https://www.ruby-lang.org/en/)
4. Install bundler `gem install bundler`
5. Install dev dependencies by running `hack/install-assets.sh`
6. Launch the console and start watching for asset changes by running `hack/serve-local-assets.sh`

    Note: If you see an ENOSPC error, you may need to increase the number of files your user can watch by running this command:
    
    ```
    echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf && sudo sysctl -p
    ```

#### Before opening a pull request
1. If needed, run `hack/build-assets.sh` to update bindata.go
2. Run the test suite with `hack/test-assets.sh`
3. Rebase and squash changes to a single commit

#### Production builds
1. Make sure all dev dependencies are up to date by running `hack/install-assets.sh`
2. Run `hack/build-assets.sh`
3. Run `hack/build-go.sh`

The assets served by the OpenShift all-in-one server will now be up to date. By default the assets are served from [http://localhost:8091](http://localhost:8091)

#### Debugging Travis failures
If Travis complains that bindata.go is different than the committed version, ensure the committed version is correct:

1. Run `hack/install-assets.sh`
2. Run `hack/build-assets.sh`
3. If bindata.go is changed, add it to your commit and re-push

If Travis still complains that bindata.go is different, do the following to get details about what is different:

1. Run `hack/debug-asset-diff-local.sh` locally
2. Add the generated debug.zip file to a commit and push it to your branch
3. View the diff in the Travis log
4. Once the issue is resolved, remove the commit containing the debug.zip
