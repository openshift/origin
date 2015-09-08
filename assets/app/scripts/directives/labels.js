'use strict';

angular.module('openshiftConsole')
  .directive('labels', function() {
    return {
      restrict: 'E',
      scope: {
        labels: "=",
        expand: "=?",
        canToggle: "=?"
      },
      templateUrl: 'views/directives/labels.html',
      link: function(scope, element, attrs) {
        if (!angular.isDefined(attrs.canToggle)) {
          scope.canToggle = true;
        }
      }
    };
  });
