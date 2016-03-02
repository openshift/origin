'use strict';

angular.module('openshiftConsole')
  .directive('pipelinesView', function() {
    return {
      restrict: 'E',
      scope: {
        imageStreams: '=',
        services: '=',
        deploymentConfigsByService: '=',
        deploymentsByServiceByDeploymentConfig: '='
      },
      templateUrl: 'views/_pipelines-view.html'
    };
  });