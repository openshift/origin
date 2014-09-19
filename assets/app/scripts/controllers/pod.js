'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutCtrl
 * @description
 * # AboutCtrl
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodController', function ($scope, $http, $routeParams) {
    $scope.pod = {desiredState: {containers: []}};
    $http.defaults.useXDomain = true;
    $http.get('http://localhost:8080/api/v1beta1/pods/' + $routeParams.pod).success(function(data) {
      $scope.pod = data;
    });
  });
