'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutCtrl
 * @description
 * # AboutCtrl
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodsController', function ($scope, $http, DataService) {
    $scope.pods = {};
    $scope.podsByLabel = {};
    $scope.watches = [];
    var callback = function(pods, action, pod) {
      $scope.$apply(function() {
        $scope.pods = pods.by("metadata.name");
        $scope.podsByLabel = pods.by("labels", "metadata.name");
      });
    };

    $scope.watches.push(DataService.watch("pods", $scope, callback));

    $scope.$on('$destroy', function(){
      for (var i = 0; i < $scope.watches.length; i++) {
        DataService.unwatch($scope.watches[i]);
      }
    });    
  });
