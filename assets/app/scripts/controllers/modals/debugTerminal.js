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
    $scope.close = function() {
      $uibModalInstance.close('close');
    };
  });
