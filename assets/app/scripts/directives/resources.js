'use strict';

angular.module('openshiftConsole')
  .directive('podTemplate', function() {
    return {
      restrict: 'E',
      templateUrl: 'views/_pod-template.html'
    };
  })
  .directive('pods', function() {
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
