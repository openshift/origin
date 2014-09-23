'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutCtrl
 * @description
 * # AboutCtrl
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('MinionsController', function ($scope, $http) {
    $scope.minions = [];
    $http.defaults.useXDomain = true;
    $http.get('http://localhost:8080/api/v1beta1/minions').success(function(data) {
      $scope.minions = data.minions;
    });
  });
