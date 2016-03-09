'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:CreateRouteController
 * @description
 * # CreateRouteController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CreateRouteController', function ($filter, $routeParams, $scope, $window, ApplicationGenerator, DataService, Navigate, ProjectsService) {
    $scope.alerts = {};
    $scope.renderOptions = {
      hideFilterWidget: true
    };
    $scope.projectName = $routeParams.project;
    $scope.serviceName = $routeParams.service;

    // Prefill route name with the service name.
    $scope.routing = {
      name: $scope.serviceName || ""
    };

    $scope.breadcrumbs = [
      {
        title: $scope.projectName,
        link: "project/" + $scope.projectName
      },
      {
        title: "Create Route"
      }
    ];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        // Update project breadcrumb with display name.
        $scope.breadcrumbs[0].title = $filter('displayName')(project);
        var updatePortOptions = function(service) {
          if (!service) {
            return;
          }

          var unnamedServicePort;
          if (service.spec.ports.length === 1 && !service.spec.ports[0].name) {
            unnamedServicePort = true;
            $scope.alerts['unnamed-service-port'] = {
              type: "warning",
              message: "Service " + service.metadata.name + " has a single, unnamed port.",
              details: "A route cannot specifically target an unnamed service port. " +
                "If more service ports are added later, the route will also direct traffic to them."
            };
          }

          // Only show port options when there is more than one port or when a
          // single service port has a name. We want to use the service port
          // name when creating a route. (Port name is required for services
          // with more than one port.)
          if (service.spec.ports.length && !unnamedServicePort) {
            $scope.routing.portOptions = _.map(service.spec.ports, function(portMapping) {
              return {
                port: portMapping.name,
                // \u2192 is a Unicode right arrow.
                label: portMapping.port + " \u2192 " +
                       portMapping.targetPort + " (" + portMapping.protocol + ")"
              };
            });
            $scope.routing.targetPort = $scope.routing.portOptions[0];
          } else {
            $scope.routing.portOptions = [];
          }
        };

        var labels = {},
            orderByDisplayName = $filter('orderByDisplayName');
        if ($scope.serviceName) {
          DataService.get("services", $scope.serviceName, context).then(function(service) {
            updatePortOptions(service);
            labels = angular.copy(service.metadata.labels);
          });
        } else {
          // Prompt the user for the service.
          DataService.list("services", context, function(services) {
            $scope.services = orderByDisplayName(services.by("metadata.name"));
            if (!$scope.routing.service && $scope.services.length) {
              $scope.routing.service = $scope.services[0];
            }
            $scope.$watch('routing.service', function() {
              updatePortOptions($scope.routing.service);
              labels = angular.copy($scope.routing.service.metadata.labels);
            });
          });
        }

        $scope.createRoute = function() {
          $scope.disableInputs = true;
          if ($scope.createRouteForm.$valid) {
            var serviceName = $scope.serviceName || $scope.routing.service.metadata.name;
            var route = ApplicationGenerator.createRoute($scope.routing, serviceName, labels);
            DataService.create('routes', null, route, context)
              .then(function() { // Success
                // Return to the previous page
                $window.history.back();
              }, function(result) { // Failure
                $scope.disableInputs = false;
                $scope.alerts['create-route'] = {
                  type: "error",
                  message: "An error occurred creating the route.",
                  details: $filter('getErrorDetails')(result)
                };
              });
          }
        };
    }));
  });
