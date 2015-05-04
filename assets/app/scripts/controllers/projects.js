'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectsController
 * @description
 * # ProjectsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectsController', function ($scope, $location, DataService, AuthService) {
    $scope.projects = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";

    AuthService.withUser().then(function() {
      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        $scope.emptyMessage = "No projects to show.";
      });
    });
  });
