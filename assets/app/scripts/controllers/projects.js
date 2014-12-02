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

    $scope.tileClickHandler = function(evt) {
      var t = $(evt.target);
      if (t && t.is('a')){
        return;
      }
      var a = $('a.tile-target', t)[0];
      if (a) {
        if (evt.which === 2 || evt.ctrlKey || evt.shiftKey) {
          window.open(a.href);
        }
        else {
          window.location = a.href;
        }
      }
    };
  });
