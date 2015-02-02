'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectController', function ($scope, $routeParams, DataService, $filter, LabelFilter) {
    $scope.projectName = $routeParams.project;
    $scope.project = {};
    $scope.projectPromise = $.Deferred();
    $scope.projects = {};

    var projectCallback = function(project) {
      $scope.$apply(function(){
        $scope.project = project;
        $scope.projectPromise.resolve(project);
      });
    };

    DataService.get("projects", $scope.projectName, $scope, projectCallback);

    var projectsCallback = function(projects) {
      $scope.$apply(function(){
        $scope.projects = projects.by("metadata.name");
      });

      console.log("projects", $scope.projects);
    };
    
    DataService.list("projects", $scope, projectsCallback);
  });
