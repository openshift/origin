/*
  This file contains extensions being used by the OpenShift Online Developer Preview
  They can be used as reference examples.
*/

/*
  Override the CLI download locations
*/
window.OPENSHIFT_CONSTANTS.CLI = {
  "Linux (32 bits)": "https://openshift.com/tbd_url/32bitlinux.tar.gz",
  "Linux (64 bits)": "https://openshift.com/tbd_url/64bitlinux.tar.gz",
  "Windows":         "https://openshift.com/tbd_url/win.zip",
  "Mac OS X":        "https://openshift.com/tbd_url/mac.zip"
};

/*
  Override the documentation links
*/
window.OPENSHIFT_CONSTANTS.HELP = {
  "cli":                     "https://docs.openshift.com/online/latest/cli_reference/overview.html",
  "get_started_cli":         "https://docs.openshift.com/online/latest/cli_reference/get_started_cli.html",
  "basic_cli_operations":    "https://docs.openshift.com/online/latest/cli_reference/basic_cli_operations.html",
  "webhooks":                "https://docs.openshift.com/online/latest/dev_guide/builds.html#webhook-triggers",
  "new_app":                 "https://docs.openshift.com/online/latest/dev_guide/new_app.html",
  "start-build":             "https://docs.openshift.com/online/latest/dev_guide/builds.html#starting-a-build",
  "deployment-operations":   "https://docs.openshift.com/online/latest/cli_reference/basic_cli_operations.html#build-and-deployment-cli-operations",
  "route-types":             "https://docs.openshift.com/online/latest/architecture/core_concepts/routes.html#route-types",
  "persistent_volumes":      "https://docs.openshift.com/online/latest/dev_guide/persistent_volumes.html",
  "compute_resources":       "https://docs.openshift.com/online/latest/dev_guide/compute_resources.html",
  "default":                 "https://docs.openshift.com/online/latest/welcome/index.html"
};

/*
  Custom angular module 
*/
angular.module('openshiftOnlineConsoleExtensions', ['openshiftConsole'])
  .run(function(HawtioExtension) {

    /*
      Request system status from statuspage.io
    */
    var system_status_elem = $('<a href="http://status.openshift.com/" target="_blank" class="nav-item-iconic system-status project-action-btn" style="display: none;">');
    var system_status_elem_mobile = $('<div row flex class="navbar-flex-btn system-status-mobile" style="display: none;">');

    $.getJSON("https://m0sg3q4t415n.statuspage.io/api/v2/summary.json", function (data) {
      var n = (data.incidents || [ ]).length;
    
      if (n > 0) {
        var issueStr = n + ' open issue';
        if (n !== 1) {
          issueStr += "s";
        }
        $('<span title="System Status" class="fa status-icon pficon-warning-triangle-o"></span>').appendTo(system_status_elem);
        $('<span class="status-issue">' + issueStr + '</span>').appendTo(system_status_elem);
        system_status_elem.css('display', '');

        system_status_elem_mobile.append(system_status_elem.clone());
        system_status_elem_mobile.css('display', '');
      }
    });

    /*
      Render system status into the extension point in the top navigation bar
    */
    HawtioExtension.add('nav-system-status', function ($scope) {
      return system_status_elem;
    });

    HawtioExtension.add('nav-system-status-mobile', function ($scope) {
      return system_status_elem_mobile;
    });    

    /*
      Add additional items to the help dropdown in the top navigation bar
    */
    HawtioExtension.add('nav-help-dropdown', function ($scope) {
      var li = $('<li>');
        $('<a href="https://bugzilla.redhat.com/enter_bug.cgi?product=OpenShift%20Online" target="_blank">Report a Bug</a>')
          .appendTo(li);
      return li;
    });

    HawtioExtension.add('nav-help-dropdown', function ($scope) {
      var li = $('<li>');
        $('<a href="https://stackoverflow.com/tags/openshift" target="_blank">Stack Overflow</a>')
          .appendTo(li);
      return li;
    });    

    HawtioExtension.add('nav-help-dropdown', function ($scope) {
      return $('<li class="divider">');
    });

    HawtioExtension.add('nav-help-dropdown', function ($scope) {
      var li = $('<li>');
        $('<a href="http://status.openshift.com/" target="_blank">System Status</a>')
          .appendTo(li);
      return li;
    });    
  });

/*
  Register the custom angular module
*/
hawtioPluginLoader.addModule('openshiftOnlineConsoleExtensions');