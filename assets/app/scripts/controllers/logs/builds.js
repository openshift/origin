'use strict';

angular.module('openshiftConsole')
  .controller('projects.logs.builds', [
    '$log',
    '$routeParams',
    '$scope',
    'DataService',
    function($log, $routeParams, $scope, DataService) {

      DataService.watch('builds', $scope, function(builds) {
        angular.extend($scope, {
          builds: builds.by('metadata.name'),
          emptyMessage: 'No builds to show'
        });
      });
    }
  ]);
