'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectController', function ($scope, $routeParams, DataService, AuthService, $filter, LabelFilter) {

    $scope.projectName = $routeParams.project;
    $scope.project = {};
    $scope.projectPromise = $.Deferred();
    $scope.projects = {};
    $scope.renderOptions = {
      hideFilterWidget: false
    };

    AuthService.withUser().then(function() {
      DataService.get("projects", $scope.projectName, $scope).then(function(project) {
        $scope.project = project;
        $scope.projectPromise.resolve(project);
      });

      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        console.log("projects", $scope.projects);
      });

    });
  });
