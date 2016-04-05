'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DebugTerminalModalController
 * @description
 * # DebugTerminalModalController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DebugTerminalModalController', function ($scope, $filter, $uibModalInstance, container, image) {
    $scope.container = container;
    $scope.image = image;
    $scope.$watch('debugPod.status.containerStatuses', function() {
      $scope.containerState = _.get($scope, 'debugPod.status.containerStatuses[0].state');
    });
    $scope.close = function() {
      $uibModalInstance.close('close');
    };
  });
