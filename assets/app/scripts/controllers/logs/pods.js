'use strict';

angular.module('openshiftConsole')
  .controller('projects.logs.pods', [
    '$log',
    '$scope',
    'AuthService',
    'DataService',
    'LabelFilter',
    function($log, $scope, AuthService, DataService, LabelFilter) {
      $log.log('project/:project/logs/pods');

        DataService.watch('pods', $scope, function(pods) {
          var unfilteredPods = pods.by('metadata.name');
          angular.extend($scope, {
            // projectName is provided by mixing in ProjectController in the template
            pods: unfilteredPods,
            emptyMessage: 'No pods to show'
          });
        });

    }
  ]);
