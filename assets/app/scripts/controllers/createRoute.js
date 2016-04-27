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

        var labels = {},
            orderByDisplayName = $filter('orderByDisplayName');

        DataService.list("services", context, function(services) {
          $scope.services = orderByDisplayName(services.by("metadata.name"));
          $scope.routing.service = _.find($scope.services, function(service) {
            return !$scope.serviceName || service.metadata.name === $scope.serviceName;
          });
          $scope.$watch('routing.service', function() {
            labels = angular.copy($scope.routing.service.metadata.labels);
          });
        });

        $scope.createRoute = function() {
          if ($scope.createRouteForm.$valid) {
            $scope.disableInputs = true;
            var serviceName = $scope.routing.service.metadata.name;
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
