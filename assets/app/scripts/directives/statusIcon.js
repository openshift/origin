'use strict';

angular.module('openshiftConsole')
  .directive('statusIcon', [
    function() {
      return {
        restrict: 'E',
        templateUrl: 'views/directives/_status-icon.html',
        scope: {
          status: '=',
          disableAnimation: "@"
        },
        link: function($scope, $elem, $attrs) {
          $scope.spinning = !angular.isDefined($attrs.disableAnimation);
        }
      };
    }
  ]);
