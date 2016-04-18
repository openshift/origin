'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:EditRouteController
 * @description
 * # EditRouteController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('EditRouteController', function ($filter,
                                               $location,
                                               $routeParams,
                                               $scope,
                                               AlertMessageService,
                                               DataService,
                                               Navigate,
                                               ProjectsService) {
    $scope.alerts = {};
    $scope.renderOptions = {
      hideFilterWidget: true
    };
    $scope.projectName = $routeParams.project;
    $scope.routeName = $routeParams.route;
    $scope.loading = true;

    $scope.routeURL = Navigate.resourceURL($scope.routeName, "Route", $scope.projectName);
    $scope.breadcrumbs = [{
      title: $scope.projectName,
      link: 'project/' + $scope.projectName
    }, {
      title: 'Routes',
      link: 'project/' + $scope.projectName + '/browse/routes'
    }, {
      title: $scope.routeName,
      link: $scope.routeURL
    }, {
      title: "Edit"
    }];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        // Update project breadcrumb with display name.
        $scope.breadcrumbs[0].title = $filter('displayName')(project);

        var orderByDisplayName = $filter('orderByDisplayName');

        var route;
        DataService.get("routes", $scope.routeName, context).then(
          function(original) {
            route = angular.copy(original);
            var serviceName = _.get(route, 'spec.to.name');
            $scope.routing = {
              service: _.get(route, 'spec.to.name'),
              host: _.get(route, 'spec.host'),
              path: _.get(route, 'spec.path'),
              targetPort: _.get(route, 'spec.port.targetPort'),
              tls: angular.copy(_.get(route, 'spec.tls'))
            };

            DataService.list("services", context, function(services) {
              var servicesByName = services.by("metadata.name");
              $scope.loading = false;
              $scope.services = orderByDisplayName(servicesByName);
              $scope.routing.service = servicesByName[serviceName];
            });
          },
          function() {
            Navigate.toErrorPage("Could not load route " + $scope.routeName + ".");
          });

        // Update the fields in the route from what was entered in the form.
        var updateRouteFields = function() {
          var serviceName = _.get($scope, 'routing.service.metadata.name');
          _.set(route, 'spec.to.name', serviceName);

          // Remove the host.generated annotation if the host was edited.
          if (_.get(route, ['metadata', 'annotations', 'openshift.io/host.generated']) === 'true' &&
              _.get(route, 'spec.host') !== $scope.routing.host) {
            delete route.metadata.annotations['openshift.io/host.generated'];
          }

          route.spec.host = $scope.routing.host;
          route.spec.path = $scope.routing.path;

          var targetPort = $scope.routing.targetPort;
          if (targetPort) {
            _.set(route, 'spec.port.targetPort', targetPort);
          } else {
            delete route.spec.port;
          }

          if (_.get($scope, 'routing.tls.termination')) {
            route.spec.tls = $scope.routing.tls;
          } else {
            delete route.spec.tls;
          }
        };

        $scope.updateRoute = function() {
          if ($scope.form.$valid) {
            $scope.disableInputs = true;
            updateRouteFields();
            DataService.update('routes', $scope.routeName, route, context)
              .then(function() { // Success
                AlertMessageService.addAlert({
                  name: $scope.routeName,
                  data: {
                    type: "success",
                    message: "Route " + $scope.routeName + " was successfully updated."
                  }
                });
                $location.path($scope.routeURL);
              }, function(response) { // Failure
                $scope.disableInputs = false;
                $scope.alerts['update-route'] = {
                  type: "error",
                  message: "An error occurred updating route " + $scope.routeName + ".",
                  details: $filter('getErrorDetails')(response)
                };
              });
          }
        };
    }));
  });
