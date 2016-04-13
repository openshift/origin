'use strict';

angular.module('openshiftConsole')
  .directive('parseError', function() {
    return {
      restrict: 'E',
      scope: {
        error: '='
      },
      templateUrl: 'views/_parse-error.html'
    };
  });
