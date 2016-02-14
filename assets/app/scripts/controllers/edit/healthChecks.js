'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:EditHealthChecksController
 * @description
 * # EditHealthChecksController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('EditHealthChecksController',
              function ($filter,
                        $location,
                        $routeParams,
                        $scope,
                        AlertMessageService,
                        APIService,
                        DataService,
                        Navigate,
                        ProjectsService) {
    if (!$routeParams.kind || !$routeParams.name) {
      Navigate.toErrorPage("Kind or name parameter missing.");
      return;
    }

    if ($routeParams.kind !== 'DeploymentConfig' && $routeParams.kind !== 'ReplicationController') {
      Navigate.toErrorPage("Health checks are not supported for kind " + $routeParams.kind + ".");
      return;
    }

    $scope.name = $routeParams.name;
    $scope.resourceURL = Navigate.resourceURL($scope.name, $routeParams.kind, $routeParams.project);
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

    // Map of removed probes so that removing and adding back a probe remembers what was previously set.
    $scope.previousProbes = {};

    var getErrorDetails = $filter('getErrorDetails');

    var displayError = function(errorMessage, errorDetails) {
      $scope.alerts['add-health-check'] = {
        type: "error",
        message: errorMessage,
        details: errorDetails
      };
    };

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        var displayName = $filter('humanizeKind')($routeParams.kind) + ' "' + $scope.name + '"';
        DataService.get(APIService.kindToResource($routeParams.kind), $scope.name, context).then(
          function(result) {
            // Modify a copy of the resource.
            var resource = angular.copy(result);
            $scope.containers = _.get(resource, 'spec.template.spec.containers');

            $scope.addProbe = function(container, probe) {
              // Restore the previous values if set.
              container[probe] = _.get($scope.previousProbes, [container.name, probe], {});
              $scope.form.$setDirty();
            };

            $scope.removeProbe = function(container, probe) {
              // Remember previous values if the probe is added back.
              _.set($scope.previousProbes, [container.name, probe], container[probe]);
              delete container[probe];
              $scope.form.$setDirty();
            };

            $scope.save = function() {
              $scope.disableInputs = true;
              DataService.update(APIService.kindToResource($routeParams.kind),
                                 $scope.name,
                                 resource,
                                 context).then(
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

