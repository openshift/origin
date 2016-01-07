'use strict';

angular.module('openshiftConsole')
  .directive('overviewDeployment', function($location, $uibModal, $timeout, $filter, LabelFilter, DeploymentsService, hashSizeFilter, isDeploymentFilter) {
    return {
      restrict: 'E',
      scope: {
        // Replication controller / deployment fields
        rc: '=',
        deploymentConfigId: '=',
        deploymentConfigMissing: '=',
        deploymentConfigDifferentService: '=',
        deploymentConfig: '=',
        scalable: '=',

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

        // Debounce scaling so multiple clicks within 500 milliseconds only result in one request.
        var scale = _.debounce(function () {
          if (!angular.isNumber($scope.desiredReplicas)) {
            return;
          }

          var showScalingError = function(result) {
            $scope.alerts = $scope.alerts || {};
            $scope.desiredReplicas = null;
            $scope.alerts["scale"] =
              {
                type: "error",
                message: "An error occurred scaling the deployment.",
                details: $filter('getErrorDetails')(result)
              };
          };

          if ($scope.deploymentConfig) {
            DeploymentsService.scaleDC($scope.deploymentConfig, $scope.desiredReplicas).then(_.noop, showScalingError);
          } else {
            DeploymentsService.scaleRC($scope.rc, $scope.desiredReplicas).then(_.noop, showScalingError);
          }
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
          if (!$scope.scalable) {
            return;
          }

          $scope.desiredReplicas = $scope.getDesiredReplicas();
          $scope.desiredReplicas++;
          scale();
        };

        $scope.scaleDown = function() {
          if (!$scope.scalable) {
            return;
          }

          $scope.desiredReplicas = $scope.getDesiredReplicas();
          if ($scope.desiredReplicas === 0) {
            return;
          }

          // Prompt before scaling to 0.
          if ($scope.desiredReplicas === 1) {
            var modalInstance = $uibModal.open({
              animation: true,
              templateUrl: 'views/modals/confirmScale.html',
              controller: 'ConfirmScaleController',
              resolve: {
                resource: function() {
                  return $scope.rc;
                },
                type: function() {
                  if (isDeploymentFilter($scope.rc)) {
                    return "deployment";
                  }

                  return "replication controller";
                }
              }
            });

            modalInstance.result.then(function() {
              // It's possible $scope.desiredReplicas was set to null if
              // rc.spec.replicas changed since the dialog was shown, so call
              // getDesiredReplicas() again.
              $scope.desiredReplicas = $scope.getDesiredReplicas() - 1;
              scale();
            });

            return;
          }

          $scope.desiredReplicas--;
          scale();
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
