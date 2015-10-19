'use strict';

angular.module('openshiftConsole')
  .directive("trExpanded", function() {
    return {
      restrict: 'E',
      transclude: true,
      replace: true,
      scope: {
        header: '@',
        headerUrl: '@',
        iconClass: '@?',
        close: '&onClose'
      },
      templateUrl: 'views/directives/tr-expanded.html',
    };
  });
