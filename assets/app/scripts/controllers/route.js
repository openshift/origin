'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServiceController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('RouteController', function ($scope, $routeParams, DataService, project, $filter) {
    $scope.route = null;
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};    
    $scope.renderOptions.hideFilterWidget = true;    
    $scope.breadcrumbs = [
      {
        title: "Routes",
        link: "project/" + $routeParams.project + "/browse/routes"
      },
      {
        title: $routeParams.route
      }
    ];

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("routes", $routeParams.route, $scope).then(
        // success
        function(route) {
          $scope.route = route;

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("routes", $routeParams.route, $scope, function(route, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This route has been deleted."
              }; 
            }
            $scope.route = route;
          }));          
        },
        // failure
        function(e) {
          $scope.alerts["load"] = {
            type: "error",
            message: "The route details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
