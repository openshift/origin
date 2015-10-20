'use strict';

angular.module('openshiftConsole')
  .directive('overviewDeployment', function($location, $timeout, $filter, LabelFilter, DeploymentsService) {
    return {
      restrict: 'E',
      scope: {
        // Replication controller / deployment fields
        rc: '=',
        deploymentConfigId: '=',
        deploymentConfigMissing: '=',
        deploymentConfigDifferentService: '=',

        // Nested podTemplate fields
        imagesByDockerReference: '=',
        builds: '=',

        // Pods
        pods: '=',

        // To display scaling errors
        alerts: '='
      },
      templateUrl: 'views/_overview-deployment.html',
      controller: function($scope) {
        $scope.$watch("rc.spec.replicas", function() {
          $scope.desiredReplicas = null;
        });

        // Debounce scaling so multiple clicks within 500 milliseconds only
        // result in one request.
        var scale = _.debounce(function () {
          if (!angular.isNumber($scope.desiredReplicas)) {
            return;
          }

          DeploymentsService.scale($scope.rc, $scope.desiredReplicas, $scope).then(
            // success, no need for a message since the UI updates immediately
            _.noop,
            // failure
            function(result) {
              $scope.alerts = $scope.alerts || {};
              $scope.desiredReplicas = null;
              $scope.alerts["scale"] =
                {
                  type: "error",
                  message: "An error occurred scaling the deployment.",
                  details: $filter('getErrorDetails')(result)
                };
            });
        }, 500);

        $scope.viewPodsForDeployment = function(deployment) {
          $location.url("/project/" + deployment.metadata.namespace + "/browse/pods");
          $timeout(function() {
            LabelFilter.setLabelSelector(new LabelSelector(deployment.spec.selector, true));
          }, 1);
        };

        $scope.scaleUp = function() {
          if (!$scope.desiredReplicas) {
            $scope.desiredReplicas = $scope.rc.spec.replicas;
          }
          $scope.desiredReplicas++;
          scale();
        };

        $scope.scaleDown = function() {
          if (!$scope.desiredReplicas) {
            $scope.desiredReplicas = $scope.rc.spec.replicas;
          }

          if ($scope.desiredReplicas > 0) {
            $scope.desiredReplicas--;
            scale();
          }
        };
      }
    };
  });

