angular.module('openshiftConsole')
  .directive('returnToOverview', function() {
    return {
      restrict: 'E',
      scope: {
        project: '='
      },
      templateUrl: 'views/directives/_return-to-overview.html'
    };
  });

