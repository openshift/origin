'use strict';
/* jshint unused: false */

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
    'ngTouch',
    'openshiftUI',
    'kubernetesUI'
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
            return "project/" + encodeURIComponent(routeParams.project) + "/" + path;
          }
        }
        return "project/:project/" + path;
      };
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
      .subPath("Events", "events", builder.join(templatePath, 'events.html'))
      .subPath("Image Streams", "images", builder.join(templatePath, 'images.html'))
      .subPath("Pods", "pods", builder.join(templatePath, 'pods.html'))
      .subPath("Services", "services", builder.join(templatePath, 'services.html'))
      .build();
    tab.icon = "sitemap";
    tabs.push(tab);

    tab = builder
            .create()
            .id(builder.join(pluginName, 'logs'))
            .title(function() { return 'Logs'; })
            .template(template)
            .href(projectHref('logs'))
            .subPath('Builds', 'builds' )
            //.subPath('Deployments', 'deployments' )
            .subPath('Pods', 'pods')
            .build();
    tab.icon = 'list-ol';
    tabs.push(tab);

    tab = builder.create()
     .id(builder.join(pluginName, "settings"))
     .title(function () { return "Settings"; })
     .template(template)
     .href(projectHref("settings"))
     .page(function () { return builder.join(templatePath, 'settings.html'); })
     .build();
    tab.icon = "sliders";
    tabs.push(tab);

  }])
  .config(function ($routeProvider) {
    $routeProvider
      .when('/', {
        templateUrl: 'views/projects.html',
        controller: 'ProjectsController'
      })
      .when('/createProject', {
        templateUrl: 'views/createProject.html',
        controller: 'CreateProjectController'
      })
      .when('/project/:project', {
        redirectTo: function(params) {
          return '/project/' + encodeURIComponent(params.project) + "/overview";
        }
      })
      .when('/project/:project/overview', {
        templateUrl: 'views/project.html'
      })
      .when('/project/:project/settings', {
        templateUrl: 'views/settings.html'
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
      .when('/project/:project/browse/events', {
        templateUrl: 'views/events.html'
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
      .when('/project/:project/catalog/templates', {
        templateUrl: 'views/catalog/templates.html'
      })
      .when('/project/:project/catalog/images', {
        templateUrl: 'views/catalog/images.html'
      })
      .when('/project/:project/create', {
        templateUrl: 'views/create.html'
      })
      .when('/project/:project/create/fromtemplate', {
        templateUrl: 'views/newfromtemplate.html'
      })
      .when('/project/:project/create/fromimage', {
        templateUrl: 'views/create/fromimage.html'
      })
      .when('/project/:project/logs', {
        redirectTo: function(params) {
          return '/project/' + encodeURIComponent(params.project) + "/logs/builds";
        }
      })
      .when('/project/:project/logs/builds', {
        templateUrl: 'views/logs/builds.html'
      })
      .when('/project/:project/logs/builds/:build/log', {
        controller: 'BuildLog',
        templateUrl: function(params) {
          return params.view ?
                    'views/logs/'+params.view+'_log.html' :
                    'views/logs/build_log.html';
        }
      })
      .when('/project/:project/logs/pods', {
        templateUrl: 'views/logs/pods.html'
      })
      .when('/project/:project/logs/pods/:pod/log', {
        controller: 'PodLog',
        templateUrl: function(params) {
          return params.view ?
                    'views/logs/'+params.view+'_log.html' :
                    'views/logs/pod_log.html';
        }
      })
      .when('/oauth', {
        templateUrl: 'views/util/oauth.html',
        controller: 'OAuthController'
      })
      .when('/error', {
        templateUrl: 'views/util/error.html',
        controller: 'ErrorController'
      })
      .when('/logout', {
        templateUrl: 'views/util/logout.html',
        controller: 'LogoutController'
      })

      .otherwise({
        redirectTo: '/'
      });
  })
  .constant("API_CFG", angular.extend({}, (window.OPENSHIFT_CONFIG || {}).api))
  .constant("AUTH_CFG", angular.extend({}, (window.OPENSHIFT_CONFIG || {}).auth))
  .config(function($httpProvider, AuthServiceProvider, RedirectLoginServiceProvider, AUTH_CFG, API_CFG) {
    $httpProvider.interceptors.push('AuthInterceptor');

    AuthServiceProvider.LoginService('RedirectLoginService');
    AuthServiceProvider.LogoutService('DeleteTokenLogoutService');
    // TODO: fall back to cookie store when localStorage is unavailable (see known issues at http://caniuse.com/#feat=namevalue-storage)
    AuthServiceProvider.UserStore('LocalStorageUserStore');

    RedirectLoginServiceProvider.OAuthClientID(AUTH_CFG.oauth_client_id);
    RedirectLoginServiceProvider.OAuthAuthorizeURI(AUTH_CFG.oauth_authorize_uri);
    RedirectLoginServiceProvider.OAuthRedirectURI(URI(AUTH_CFG.oauth_redirect_base).segment("oauth").toString());
  })
  .config(function($compileProvider){
    $compileProvider.aHrefSanitizationWhitelist(/^\s*(https?|mailto|git):/i);
  })
  .run(['mainNavTabs', "HawtioNav", function (tabs, HawtioNav) {
    for (var i = 0; i < tabs.length; i++) {
      HawtioNav.add(tabs[i]);
    }
  }])
  .run(function(dateRelativeFilter, durationFilter) {
    // Use setInterval instead of $interval because we're directly manipulating the DOM and don't want scope.$apply overhead
    setInterval(function() {
      $('.timestamp[data-timestamp]').text(function(i, existing) {
        return dateRelativeFilter($(this).attr("data-timestamp")) || existing;
      });
    }, 30 * 1000);
    setInterval(function() {
      $('.duration[data-timestamp]').text(function(i, existing) {
        return durationFilter($(this).attr("data-timestamp")) || existing;
      });
    }, 1000);
  });

hawtioPluginLoader.addModule('openshiftConsole');
