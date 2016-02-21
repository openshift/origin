'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:CreateRouteController
 * @description
 * # CreateRouteController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('SetLimitsController', function ($filter, $location, $parse, $routeParams, $scope, AlertMessageService, DataService, LimitRangesService, Navigate, ProjectsService) {
    if ($routeParams.dcName && $routeParams.rcName) {
      Navigate.toErrorPage("Replication controller and deployment config can't both be provided.");
    }

    var type, displayName;
    if ($routeParams.dcName) {
      type = "deploymentconfigs";
      $scope.name = $routeParams.dcName;
      displayName = 'Deployment Configuration "' + $scope.name + '"';
      $scope.resourceURL = Navigate.resourceURL($scope.name, "DeploymentConfig", $routeParams.project);
    } else if ($routeParams.rcName) {
      type = "replicationcontrollers";
      $scope.name = $routeParams.rcName;
      displayName = 'Replication Controller "' + $scope.name + '"';
      $scope.resourceURL = Navigate.resourceURL($scope.name, "ReplicationController", $routeParams.project);
      $scope.showPodWarning = true;
    } else {
      Navigate.toErrorPage("A replication controller or deployment config must be provided.");
    }

    $scope.alerts = {};
    $scope.renderOptions = {
      hideFilterWidget: true
    };

    $scope.breadcrumbs = [{
      title: $routeParams.project,
      link: "project/" + $routeParams.project
    }, {
      title: "Deployments",
      link: "project/" + $routeParams.project + "/browse/deployments"
    }, {
      title: $scope.name,
      link: $scope.resourceURL
    }, {
      title: "Set Resource Limits"
    }];

    var getErrorDetails = $filter('getErrorDetails');

    var displayError = function(errorMessage, errorDetails) {
      $scope.alerts['set-compute-limits'] = {
        type: "error",
        message: errorMessage,
        details: errorDetails
      };
    };

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        // Update project breadcrumb with display name.
        $scope.breadcrumbs[0].title = $filter('displayName')(project);

        // Check if requests or limits are calculated. Memory limit is never calculated.
        $scope.cpuRequestCalculated = LimitRangesService.isRequestCalculated('cpu', project);
        $scope.cpuLimitCalculated = LimitRangesService.isLimitCalculated('cpu', project);
        $scope.memoryRequestCalculated = LimitRangesService.isRequestCalculated('memory', project);

        DataService.get(type, $scope.name, context).then(
          function(result) {
            var resource = angular.copy(result);
            $scope.containers = _.get(resource, 'spec.template.spec.containers');
            $scope.save = function() {
              $scope.disableInputs = true;
              DataService.update(type, $scope.name, resource, context).then(
                function() {
                  AlertMessageService.addAlert({
                    name: $scope.name,
                    data: {
                      type: "success",
                      message: displayName + " was updated."
                    }
                  });
                  $location.url($scope.resourceURL);
                },
                function(result) {
                  $scope.disableInputs = false;
                  displayError(displayName + ' could not be updated.', getErrorDetails(result));
                });
            };
          },
          function(result) {
            displayError(displayName + ' could not be loaded.', getErrorDetails(result));
          }
        );

        var validatePodLimits = function() {
          if (!$scope.hideCPU) {
            $scope.cpuProblems = LimitRangesService.validatePodLimits($scope.limitRanges, 'cpu', $scope.containers, project);
          }
          $scope.memoryProblems = LimitRangesService.validatePodLimits($scope.limitRanges, 'memory', $scope.containers, project);
        };

        DataService.list("limitranges", context, function(limitRanges) {
          $scope.limitRanges = limitRanges.by("metadata.name");
          if ($filter('hashSize')(limitRanges) !== 0) {
            $scope.$watch('containers', validatePodLimits, true);
          }
        });
    }));
  });
