'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectsController
 * @description
 * # ProjectsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectsController', function ($scope, DataService) {   
    $scope.projects = [];

    var callback = function(projects) {
      $scope.$apply(function(){
        $scope.projects = projects.items;
      });
    };

    DataService.getList("projects", callback, $scope);
  });
