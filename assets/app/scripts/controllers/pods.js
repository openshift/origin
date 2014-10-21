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

    var callback = function(action, pod) {
      $scope.$apply(function() {
        var label, labelValue;
        if (action === 'ADDED' || action === 'MODIFIED') {
          $scope.pods[pod.id] = pod;

          for (label in pod.labels) {
            labelValue = pod.labels[label];
            if (!$scope.podsByLabel[label]) {
              $scope.podsByLabel[label] = {};
            }
            if (!$scope.podsByLabel[label][labelValue]) {
              $scope.podsByLabel[label][labelValue] = {};
            }
            $scope.podsByLabel[label][labelValue][pod.id] = pod;
          }

        }
        else if (action === 'DELETED') {
          delete $scope.pods[pod.id];

          for (label in pod.labels) {
            labelValue = pod.labels[label];
            if ($scope.podsByLabel[label] && $scope.podsByLabel[label][labelValue]) {
              delete $scope.podsByLabel[label][labelValue][pod.id];
            }
          }        
        }
      });
    };

    // TODO we should separately call DataService.getList first to make a static request
    // But this won't help us until we implement tracking of resourceVersion
    DataService.subscribe("pods", callback, $scope);
  });
