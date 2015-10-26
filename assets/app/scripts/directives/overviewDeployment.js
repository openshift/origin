'use strict';

angular.module('openshiftConsole')
  .directive('overviewDeployment', function($location, $timeout, $filter, LabelFilter, DeploymentsService, hashSizeFilter) {
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
          if (hashSizeFilter($scope.pods) === 0) {
            return;
          }

          $location.url("/project/" + deployment.metadata.namespace + "/browse/pods");
          $timeout(function() {
            LabelFilter.setLabelSelector(new LabelSelector(deployment.spec.selector, true));
          }, 1);
        };

        $scope.scaleUp = function() {
          $scope.desiredReplicas = $scope.getDesiredReplicas();
          $scope.desiredReplicas++;
          scale();
        };

        $scope.scaleDown = function() {
          $scope.desiredReplicas = $scope.getDesiredReplicas();
          if ($scope.desiredReplicas > 0) {
            $scope.desiredReplicas--;
            scale();
          }
        };

        $scope.getDesiredReplicas = function() {
          // If not null or undefined, use $scope.desiredReplicas.
          if (angular.isDefined($scope.desiredReplicas) && $scope.desiredReplicas !== null) {
            return $scope.desiredReplicas;
          }

          if ($scope.rc && $scope.rc.spec && angular.isDefined($scope.rc.spec.replicas)) {
            return $scope.rc.spec.replicas;
          }

          return 1;
        };
      }
    };
  });
