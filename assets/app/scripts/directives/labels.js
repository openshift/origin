'use strict';

angular.module('openshiftConsole')
  .directive('labels', function() {
    return {
      restrict: 'E',
      scope: {
        labels: "="
      },
      templateUrl: 'views/directives/labels.html'
    };
  });
