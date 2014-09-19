'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutCtrl
 * @description
 * # AboutCtrl
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodsController', function ($scope, $http) {
    $scope.pods = [];
    $http.defaults.useXDomain = true;
    $http.get('http://localhost:8080/api/v1beta1/pods').success(function(data) {
      $scope.pods = data.items;
    });
  });
