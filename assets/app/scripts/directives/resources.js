'use strict';
/* jshint unused:false */

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
    return {
      restrict: 'E',
      scope: {
        triggers: '='
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
