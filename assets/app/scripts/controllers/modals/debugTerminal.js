'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DebugTerminalModalController
 * @description
 * # DebugTerminalModalController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DebugTerminalModalController', function ($scope, $filter, $uibModalInstance, containerName) {
    $scope.containerName = containerName;
    $scope.$watch('debugPod.status.containerStatuses', function() {
      $scope.containerState = _.get($scope, 'debugPod.status.containerStatuses[0].state');
    });

    var imageID = _.get($scope, 'debugPod.spec.containers[0].image');
    if (imageID) {
      $scope.image = _.get($scope, ['imagesByDockerReference', imageID]);
    }
    $scope.close = function() {
      $uibModalInstance.close('close');
    };
  });
