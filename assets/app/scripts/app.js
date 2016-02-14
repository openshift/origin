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
    'kubernetesUI',
    'ui.bootstrap',
    'patternfly.charts',
    'patternfly.sort',
    'openshiftConsoleTemplates',
    'ui.ace',
    'extension-registry',
    'as.sortable'
  ])
  .constant("mainNavTabs", [])  // even though its not really a "constant", it has to be created as a constant and not a value
                         // or it can't be referenced during module config
  // configure our tabs and routing
  .config(['mainNavTabs','$routeProvider', 'HawtioNavBuilderProvider', function(tabs, $routeProvider, builder) {
    var template = function() {
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
      .subPath("Routes", "routes", builder.join(templatePath, 'browse/routes.html'))
      .subPath("Services", "services", builder.join(templatePath, 'services.html'))
      .subPath("Storage", "storage", builder.join(templatePath, 'storage.html'))
      .build();
    tab.icon = "sitemap";
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
      .when('/create-project', {
        templateUrl: 'views/create-project.html',
        controller: 'CreateProjectController'
      })
      .when('/project/:project', {
        redirectTo: function(params) {
          return '/project/' + encodeURIComponent(params.project) + "/overview";
        }
      })
      .when('/project/:project/overview', {
        templateUrl: 'views/project.html',
        controller: 'OverviewController'
      })
      .when('/project/:project/settings', {
        templateUrl: 'views/settings.html',
        controller: 'SettingsController'
      })
      .when('/project/:project/browse', {
        redirectTo: function(params) {
          return '/project/' + encodeURIComponent(params.project) + "/browse/pods";  // TODO decide what subtab to default to here
        }
      })
      .when('/project/:project/browse/builds', {
        templateUrl: 'views/builds.html',
        controller: 'BuildsController'
      })
      .when('/project/:project/browse/builds/:buildconfig', {
        templateUrl: 'views/browse/build-config.html',
        controller: 'BuildConfigController'
      })
      .when('/project/:project/edit/builds/:buildconfig', {
        templateUrl: 'views/edit/build-config.html',
        controller: 'EditBuildConfigController'
      })
      .when('/project/:project/browse/builds/:buildconfig/:build', {
        templateUrl: function(params) {
          if (params.view === 'chromeless') {
            return 'views/logs/chromeless-build-log.html';
          }

          return 'views/browse/build.html';
        },
        controller: 'BuildController'
      })
      // For when a build is missing a buildconfig label
      // Needs to still be prefixed with browse/builds so the secondary nav active state is correct
      .when('/project/:project/browse/builds-noconfig/:build', {
        templateUrl: 'views/browse/build.html',
        controller: 'BuildController'
      })
      .when('/project/:project/browse/deployments', {
        templateUrl: 'views/deployments.html',
        controller: 'DeploymentsController'
      })
      .when('/project/:project/browse/deployments/:deploymentconfig', {
        templateUrl: 'views/browse/deployment-config.html',
        controller: 'DeploymentConfigController'
      })
      .when('/project/:project/browse/deployments/:deploymentconfig/:deployment', {
        templateUrl: function(params) {
          if (params.view === 'chromeless') {
            return 'views/logs/chromeless-deployment-log.html';
          }

          return 'views/browse/deployment.html';
        },
        controller: 'DeploymentController'
      })
      // Needs to still be prefixed with browse/deployments so the secondary nav active state is correct
      .when('/project/:project/browse/deployments-replicationcontrollers/:replicationcontroller', {
        templateUrl: 'views/browse/replication-controller.html',
        controller: 'DeploymentController'
      })
      .when('/project/:project/browse/events', {
        templateUrl: 'views/events.html',
        controller: 'EventsController'
      })
      .when('/project/:project/browse/images', {
        templateUrl: 'views/images.html',
        controller: 'ImagesController'
      })
      .when('/project/:project/browse/images/:image', {
        templateUrl: 'views/browse/image.html',
        controller: 'ImageController'
      })
      .when('/project/:project/browse/pods', {
        templateUrl: 'views/pods.html',
        controller: 'PodsController'
      })
      .when('/project/:project/browse/pods/:pod', {
        templateUrl: function(params) {
          if (params.view === 'chromeless') {
            return 'views/logs/chromeless-pod-log.html';
          }

          return 'views/browse/pod.html';
        },
        controller: 'PodController'
      })
      .when('/project/:project/browse/services', {
        templateUrl: 'views/services.html',
        controller: 'ServicesController'
      })
      .when('/project/:project/browse/services/:service', {
        templateUrl: 'views/browse/service.html',
        controller: 'ServiceController'
      })
      .when('/project/:project/browse/storage', {
        templateUrl: 'views/storage.html',
        controller: 'StorageController'
      })
      .when('/project/:project/browse/persistentvolumeclaims/:pvc', {
        templateUrl: 'views/browse/persistent-volume-claim.html',
        controller: 'PersistentVolumeClaimController'
      })
      .when('/project/:project/browse/routes', {
        templateUrl: 'views/browse/routes.html',
        controller: 'RoutesController'
      })
      .when('/project/:project/browse/routes/:route', {
        templateUrl: 'views/browse/route.html',
        controller: 'RouteController'
      })
      .when('/project/:project/create-route', {
        templateUrl: 'views/create-route.html',
        controller: 'CreateRouteController'
      })
      .when('/project/:project/attach-pvc', {
        templateUrl: 'views/attach-pvc.html',
        controller: 'AttachPVCController'
      })
      .when('/project/:project/create', {
        templateUrl: 'views/create.html',
        controller: 'CreateController'
      })
      .when('/project/:project/create/fromtemplate', {
        templateUrl: 'views/newfromtemplate.html',
        controller: 'NewFromTemplateController'
      })
      .when('/project/:project/create/fromimage', {
        templateUrl: 'views/create/fromimage.html',
        controller: 'CreateFromImageController'
      })
      .when('/project/:project/create/next', {
        templateUrl: 'views/create/next-steps.html',
        controller: 'NextStepsController'
      })
      .when('/project/:project/set-limits', {
        templateUrl: 'views/set-limits.html',
        controller: 'SetLimitsController'
      })
      .when('/project/:project/edit/autoscaler', {
        templateUrl: 'views/edit/autoscaler.html',
        controller: 'EditAutoscalerController'
      })
      .when('/project/:project/edit/health-checks', {
        templateUrl: 'views/edit/health-checks.html',
        controller: 'EditHealthChecksController'
      })
      .when('/about', {
        templateUrl: 'views/about.html',
        controller: 'AboutController'
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
      // legacy redirects
      .when('/createProject', {
        redirectTo: '/create-project'
      })
      .when('/project/:project/createRoute', {
        redirectTo: '/project/:project/create-route'
      })
      .when('/project/:project/attachPVC', {
        redirectTo: '/project/:project/attach-pvc'
      })
      .otherwise({
        redirectTo: '/'
      });
  })
  .constant("API_CFG", angular.extend({}, (window.OPENSHIFT_CONFIG || {}).api))
  .constant("APIS_CFG", angular.extend({}, (window.OPENSHIFT_CONFIG || {}).apis))
  .constant("AUTH_CFG", angular.extend({}, (window.OPENSHIFT_CONFIG || {}).auth))
  .constant("LOGGING_URL", (window.OPENSHIFT_CONFIG || {}).loggingURL)
  .constant("METRICS_URL", (window.OPENSHIFT_CONFIG || {}).metricsURL)
  .constant("LIMIT_REQUEST_OVERRIDES", _.get(window.OPENSHIFT_CONFIG, "limitRequestOverrides"))
  // Sometimes we need to know the css breakpoints, make sure to update this
  // if they ever change!
  .constant("BREAKPOINTS", {
    screenXsMin:  480,   // screen-xs
    screenSmMin:  768,   // screen-sm
    screenMdMin:  992,   // screen-md
    screenLgMin:  1200,  // screen-lg
    screenXlgMin: 1600   // screen-xlg
  })
  .constant('SOURCE_URL_PATTERN', /^((ftp|http|https|git):\/\/(\w+:{0,1}[^\s@]*@)|git@)?([^\s@]+)(:[0-9]+)?(\/|\/([\w#!:.?+=&%@!\-\/]))?$/ )
  // http://stackoverflow.com/questions/9038625/detect-if-device-is-ios
  .constant('IS_IOS', /iPad|iPhone|iPod/.test(navigator.userAgent) && !window.MSStream)
  .config(function($httpProvider, AuthServiceProvider, RedirectLoginServiceProvider, AUTH_CFG, API_CFG, kubernetesContainerSocketProvider) {
    $httpProvider.interceptors.push('AuthInterceptor');

    AuthServiceProvider.LoginService('RedirectLoginService');
    AuthServiceProvider.LogoutService('DeleteTokenLogoutService');
    // TODO: fall back to cookie store when localStorage is unavailable (see known issues at http://caniuse.com/#feat=namevalue-storage)
    AuthServiceProvider.UserStore('LocalStorageUserStore');

    RedirectLoginServiceProvider.OAuthClientID(AUTH_CFG.oauth_client_id);
    RedirectLoginServiceProvider.OAuthAuthorizeURI(AUTH_CFG.oauth_authorize_uri);
    RedirectLoginServiceProvider.OAuthRedirectURI(URI(AUTH_CFG.oauth_redirect_base).segment("oauth").toString());

    // Configure the container terminal
    kubernetesContainerSocketProvider.WebSocketFactory = "ContainerWebSocket";
  })
  .config(function($compileProvider){
    $compileProvider.aHrefSanitizationWhitelist(/^\s*(https?|mailto|git):/i);
  })
  .run(['mainNavTabs', "HawtioNav", function (tabs, HawtioNav) {
    for (var i = 0; i < tabs.length; i++) {
      HawtioNav.add(tabs[i]);
    }
  }])
  .run(function($rootScope, LabelFilter){
    $rootScope.$on('$locationChangeSuccess', function(event) {
      LabelFilter.setLabelSelector(new LabelSelector({}, true), true);
    });
  })
  .run(function(dateRelativeFilter, durationFilter) {
    // Use setInterval instead of $interval because we're directly manipulating the DOM and don't want scope.$apply overhead
    setInterval(function() {
      // Set by relative-timestamp directive.
      $('.timestamp[data-timestamp]').text(function(i, existing) {
        return dateRelativeFilter($(this).attr("data-timestamp"), $(this).attr("data-drop-suffix")) || existing;
      });
    }, 30 * 1000);
    setInterval(function() {
      // Set by duration-until-now directive.
      $('.duration[data-timestamp]').text(function(i, existing) {
        var timestamp = $(this).data("timestamp");
        var omitSingle = $(this).data("omit-single");
        var precision = $(this).data("precision");
        return durationFilter(timestamp, null, omitSingle, precision) || existing;
      });
    }, 1000);
  })
  // prepopulating extension points with some of our own data
  // (ensures a single interface for extending the UI)
  .run(function(extensionRegistry) {
    extensionRegistry
      .add('nav-help-dropdown', function() {
        return [
          {
            type: 'dom',
            node: '<li><a href="https://docs.openshift.org/latest/welcome/index.html">Documentation</a></li>'
          },
          {
            type: 'dom',
            node: '<li><a href="about">About</a></li>'
          }
        ];
      });
    extensionRegistry
      .add('nav-user-dropdown', function() {
        return [{
          type: 'dom',
          node: '<li><a href="logout">Log out</a></li>'
        }];
      });
  });

hawtioPluginLoader.addModule('openshiftConsole');
