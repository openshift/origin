'use strict';

/**
 * @ngdoc overview
 * @name openshiftConsole
 * @description
 * # openshiftConsole
 *
 * Main module of the application.
 */
angular
  .module('openshiftConsole', [
    'ngAnimate',
    'ngCookies',
    'ngResource',
    'ngRoute',
    'ngSanitize',
    'ngTouch'
  ])
  .constant("mainNavTabs", [])  // even though its not really a "constant", it has to be created as a constant and not a value
                         // or it can't be referenced during module config
  // configure our tabs and routing
  .config(['mainNavTabs','$routeProvider', 'HawtioNavBuilderProvider', function(tabs, $routeProvider, builder) {
    var template = function() {
      // TODO - Don't love triggering the show/hide drawer here, would prefer if 
      // we could listen for an event that the nav was being redrawn and 
      // check HawtioNav.selected()
      if (this.isSelected()) {
        if (this.tabs && this.tabs.length > 0) {
          $("body").addClass("show-drawer");
        }
        else {
          $("body").removeClass("show-drawer");
        }
      }
      return "<sidebar-nav-item></sidebar-nav-item>";        
    };

    var projectHref = function(path) {
      return function() {
        var injector = HawtioCore.injector;
        if (injector) {
          var routeParams = injector.get("$routeParams");
          if (routeParams.project) {
            return "/project/" + encodeURIComponent(routeParams.project) + "/" + path;
          }
        }
        return "/project/:project/" + path; 
      }
    };

    var templatePath = "views";
    var pluginName = "openshiftConsole";
    var tab = builder.create()
     .id(builder.join(pluginName, "overview"))
     .title(function () { return "Overview"; })
     .template(template)
     .href(projectHref("overview"))
      .page(function () { return builder.join(templatePath, 'project.html'); })                     
     .build();
    tab.icon = "dashboard";
    tabs.push(tab);


    tab = builder.create()
      .id(builder.join(pluginName, "browse"))
      .title(function () { return "Browse"; })
      .template(template)
      .href(projectHref("browse"))
      .subPath("Builds", "builds", builder.join(templatePath, 'builds.html'))
      .subPath("Deployments", "deployments", builder.join(templatePath, 'deployments.html'))
      .subPath("Images", "images", builder.join(templatePath, 'images.html'))
      .subPath("Pods", "pods", builder.join(templatePath, 'pods.html'))
      .subPath("Services", "services", builder.join(templatePath, 'services.html'))
      .build();
    tab.icon = "sitemap";
    tabs.push(tab);

  }])
  .config(function ($routeProvider) {
    $routeProvider
      .when('/', {
        templateUrl: 'views/projects.html',
        controller: 'ProjectsController'
      })
      .when('/project/:project', {
        redirectTo: function(params) {
          return '/project/' + encodeURIComponent(params.project) + "/overview";
        }
      })      
      .when('/project/:project/overview', {
        templateUrl: 'views/project.html'
      })
      .when('/project/:project/browse', {
        redirectTo: function(params) {
          return '/project/' + encodeURIComponent(params.project) + "/browse/pods";  // TODO decide what subtab to default to here
        }
      })      
      .when('/project/:project/browse/builds', {
        templateUrl: 'views/builds.html'
      })      
      .when('/project/:project/browse/deployments', {
        templateUrl: 'views/deployments.html'
      })            
      .when('/project/:project/browse/images', {
        templateUrl: 'views/images.html'
      })      
      .when('/project/:project/browse/pods', {
        templateUrl: 'views/pods.html'
      }) 
      .when('/project/:project/browse/services', {
        templateUrl: 'views/services.html'
      })       
      .otherwise({
        redirectTo: '/'
      });
  })
  .run(['mainNavTabs', "HawtioNav", function (tabs, HawtioNav) {
    for (var i = 0; i < tabs.length; i++) {
      HawtioNav.add(tabs[i]);
    }
  }]);

hawtioPluginLoader.addModule('openshiftConsole');