'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:EditHealthChecksController
 * @description
 * # EditHealthChecksController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('EditHealthChecksController', function ($filter, $location, $routeParams, $scope, AlertMessageService, DataService, Navigate, ProjectsService) {
    if ($routeParams.dcName && $routeParams.rcName) {
      Navigate.toErrorPage("Replication controller and deployment config can't both be provided.");
      return;
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
    } else {
      Navigate.toErrorPage("A replication controller or deployment config must be provided.");
      return;
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
      title: "Edit Health Checks"
    }];

    $scope.type = "http";

    // Map of removed probes so that removing and adding back a probe remembers
    // what was previously set.
    $scope.previousProbes = {};

    var getErrorDetails = $filter('getErrorDetails');

    var displayError = function(errorMessage, errorDetails) {
      $scope.alerts['add-health-check'] = {
        type: "error",
        message: errorMessage,
        details: errorDetails
      };
    };

    // Tracks whether probes have been added or removed.
    var pristine = true;

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        DataService.get(type, $scope.name, context).then(
          function(result) {
            // Modify a copy of the resource.
            var resource = angular.copy(result);
            $scope.containers = _.get(resource, 'spec.template.spec.containers');

            $scope.addProbe = function(container, probe) {
              // Restore the previous values if set.
              container[probe] = _.get($scope.previousProbes, [container.name, probe], {});
              pristine = false;
            };

            $scope.removeProbe = function(container, probe) {
              // Remember previous values if the probe is added back.
              _.set($scope.previousProbes, [container.name, probe], container[probe]);
              delete container[probe];
              pristine = false;
            };

            $scope.isPristine = function() {
              // Return false if a probe was added or removed, even if the form itself is pristine.
              return pristine && $scope.form.$pristine;
            };

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
    }));
  });

