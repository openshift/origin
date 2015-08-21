OpenShift 3 Static Assets
=========================
The static assets for OpenShift v3.  This includes the web console.

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

#### Enable / disable console log output

Debug logging can be enabled by opening your browser's JavaScript console, running the commands below, and then refreshing the page.

```
localStorage["OpenShiftLogLevel.main"] = "<log level>";
localStorage["OpenShiftLogLevel.auth"] = "<log level>";
```

Loggers:
* `OpenShiftLogLevel.main` - default logger for OpenShift
* `OpenShiftLogLevel.auth` - auth specific logger, this includes login, logout, and oauth

The supported log levels are:
* OFF (default for all loggers except main)
* INFO
* DEBUG
* WARN
* ERROR (default for main)

Note: currently most of our logging either goes to INFO or ERROR

#### Before opening a pull request
1. If needed, run `hack/build-assets.sh` to update bindata.go
2. Run the spec tests with `hack/test-assets.sh`
3. Run the end to end tests with `TEST_ASSETS=true hack/test-end-to-end.sh`
4. Rebase and squash changes to a single commit

Note: in order to run the end to end tests you must have [Chrome](http://www.google.com/chrome/) and [chromedriver](https://sites.google.com/a/chromium.org/chromedriver/) installed.  The script below will set this up for you on linux systems.

```
# Add signing key for Chrome repo
wget https://dl.google.com/linux/linux_signing_key.pub
rpm --import linux_signing_key.pub

# Add Chrome yum repo
yum-config-manager --add-repo=http://dl.google.com/linux/chrome/rpm/stable/x86_64

# Install chrome
yum install -y google-chrome-stable 

# Install chromedriver
wget https://chromedriver.storage.googleapis.com/2.16/chromedriver_linux64.zip
unzip chromedriver_linux64.zip
mv chromedriver /usr/bin/chromedriver
chown root /usr/bin/chromedriver
chmod 755 /usr/bin/chromedriver
```

#### Production builds
1. Make sure all dev dependencies are up to date by running `hack/install-assets.sh`
2. Run `hack/build-assets.sh`
3. Run `hack/build-go.sh`

The assets served by the OpenShift all-in-one server will now be up to date. By default the assets are served from [http://localhost:8091](http://localhost:8091)

#### Debugging Travis failures
If Travis complains that bindata.go is different than the committed version, ensure the committed version is correct:

1. Run `hack/clean-assets.sh`
2. Run `hack/install-assets.sh`
3. Run `hack/build-assets.sh`
4. If bindata.go is changed, add it to your commit and re-push

Architecture
------------

The OpenShift v3 web console is based on AngularJS and [Hawt.io](https://github.com/hawtio/hawtio-core)

#### Navigation

The v3 console supports a custom context root.  When running as part of the `openshift start` command the console's context root is injected into the `<base>` tag of the index.html file.  In order to support custom context roots, all console URLs must be relative, so they should not contain a leading "/" character.

For example if you want to specify a URL directly in an HTML template to go to the project overview it would look like

```
<a href="project/foo/overview">
```

and would actually resolve to be `/contextroot/project/foo/overview` by the browser.  Similarly, if you want to use JavaScript to change the current page location, you should use the $location service from angular like

```
$location.url("project/foo/overview")
```

Finally, if you want to reference the root of the web console use the path `./`

#### Custom directives and filters

The v3 console relies heavily on custom directives and filters, some of which are intended to be utilties and used throughout the console source. The list below is NOT a complete list of all of our directives and filters.

##### Directives

For more details on the expected scope arguments, see the source under [app/scripts/directives](app/scripts/directives)

* toggle (attribute) - intended for Bootstrap's data-toggle=tooltip and data-toggle=popover, will automatically initialize any tooltips and popovers
* alerts (element) - renders a set of alerts according to the [patternfly style](https://www.patternfly.org/widgets/#alerts)
* relative-timestamp (element) - renders a relative timestamp (ex: '5 minutes ago') based on the current time, auto-updating every 30 seconds
* copy-to-clipboard (element) - creates a copy to clipboard button using ZeroClipboard
* back (attribute) - when the element is clicked a simulated browser back button event occurs (calls history.back)
* select-on-focus (attribute) - when the element is focused, all text within it will be selected
* tile-click (attribute or class) - for use with the `.tile` class, when anything on the tile is clicked, a simulated click to the `a.tile-target` link will be fired.  Recommended use is by adding the `.tile-click` class to get the correct styles on hover.
* click-to-reveal (attribute) - the element will be hidden and a link to show the element will appear instead, link text is customizable
* osc-object (attribute or class) - When the element is clicked it will be shown in the details sidebar.  Using as a class is preferred to pick up hover/active styles
* truncate-long-text (element) - truncates text to a limit, optionally on word boundaries, adding a tooltip and ellipsis when the text is truncated

##### Filters

For more details on the expected arguments, see the source under [app/scripts/filters](app/scripts/filters)

* dateRelative - returns the relative date for a timestamp given the current time (ex: '5 minutes ago')
* ageLessThan - returns whether a timestamp is within a given time amount (ex: 5) and unit (ex: 'minutes').  Refer to the [Moment.js docs](http://momentjs.com/docs/#/manipulating/add/) for the supported units.
* orderObjectsByDate - given an array or hash of k8s or openshift API objects, return an array of the objects sorted by the creationTimestamp.  By default orders with oldest first, optional reverse param will return ordered by newest first.
* annotation - for a k8s or openshift api object, lets you get any annotation by key
* description - shortcut for annotation | 'description'
* tags - shortcut for annotation | 'tags'
* label - for a k8s or openshift api object, lets you get any label by key
* hashSize - returns the number of subobjects on a javascript hash
* helpLink - returns the relevant link in the OpenShift docs for a particular help topic, new help topics should be added to the filter.  DO NOT put URLs to help directly into the source in any location except for this filter

#### Extension points

There are two main ways to extend the v3 OpenShift console.

##### Add primary / secondary navigation tabs to the project nav

We rely on [hawtio-core-navigation](https://github.com/hawtio/hawtio-core-navigation) to build the primary/secondary nav that appears once you are in a project.  We have customized the rendering of the tabs, so refer to [app/scripts/app.js](app/scripts/app.js) to see how we register our out of the box tabs.

##### Inject additional content into the page

We include the [hawtio-extension-service](https://github.com/hawtio/hawtio-extension-service).  Currently we do not render any extension points, but if there are any locations where you would like to see customizable content, this is how we will add a hook to do that.  As hooks are added we will provide a list of them here.
