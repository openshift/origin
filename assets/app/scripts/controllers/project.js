'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectController', function ($scope, $routeParams, DataService) {
    // TODO get the project details, this data does not need to be live updating
    // so a single request is sufficient
    $scope.projectName = $routeParams.project;
    $scope.project = $.Deferred();

    // TODO handle errors retrieving project
    var callback = function(project) {
      $scope.project.resolve(project);
    };

    DataService.getObject("projects", $scope.projectName, callback, $scope);
  });
