'use strict';

angular.module('openshiftConsole')
  .directive('alerts', function() {
    return {
      restrict: 'E',
      scope: {
        alerts: '=',
        hideCloseButton: '=?'
      },
      templateUrl: 'views/_alerts.html'
    };
  });
