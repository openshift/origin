'use strict';

angular.module('openshiftConsole')
  .directive('ellipsisLoader', [
    function() {
      return {
        restrict: 'E',
        templateUrl: 'views/directives/_ellipsis-loader.html'
      };
    }
  ]);