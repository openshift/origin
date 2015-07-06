'use strict';

angular.module('openshiftConsole')
  .directive('overviewDeployment', function() {
    return {
      restrict: 'E',
      scope: {
      	// Replication controller / deployment fields
        rc: '=',
        deploymentConfigId: '=',
        deploymentConfigMissing: '=',
        deploymentConfigDifferentService: '=',

        // Nested podTemplate fields
        imagesByDockerReference: '=',
        builds: '=',

        // Pods
        pods: '='
      },
      templateUrl: 'views/_overview-deployment.html'
    };
  })
  .directive('overviewMonopod', function() {
    return {
      restrict: 'E',
      scope: {
        pod: '='
      },
      templateUrl: 'views/_overview-monopod.html'
    };
  })
  .directive('podTemplate', function() {
    return {
      restrict: 'E',
      scope: {
        podTemplate: '=',
        imagesByDockerReference: '=',
        builds: '='
      },
      templateUrl: 'views/_pod-template.html'
    };
  })
  .directive('pods', function($rootScope) {
    return {
      restrict: 'E',
      scope: {
        pods: '='
      },
      templateUrl: 'views/_pods.html'
    };
  })
  .directive('triggers', function() {
    var hideBuildKey = function(build) {
      return 'hide/build/' + build.metadata.namespace + '/' + build.metadata.name;
    };
    return {
      restrict: 'E',
      scope: {
        triggers: '='
      },
      link: function(scope, elem, attrs) {
        scope.isBuildHidden = function(build) {
          var key = hideBuildKey(build);
          return sessionStorage.getItem(key) === 'true';
        };
        scope.hideBuild = function(build) {
          var key = hideBuildKey(build);
          sessionStorage.setItem(key, 'true');
        };
      },
      templateUrl: 'views/_triggers.html'
    };
  })
  .directive('deploymentConfigMetadata', function() {
    return {
      restrict: 'E',
      scope: {
        deploymentConfigId: '=',
        exists: '=',
        differentService: '='
      },
      templateUrl: 'views/_deployment-config-metadata.html'
    };
  });
