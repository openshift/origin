'use strict';
/* jshint unused: false */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ConfirmScaleController
 * @description
 * # ConfirmScaleController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ConfirmScaleController', function ($scope, $uibModalInstance, resource, type) {
    $scope.resource = resource;
    $scope.type = type;

    $scope.confirmScale = function() {
      $uibModalInstance.close('confirmScale');
    };

    $scope.cancel = function() {
      $uibModalInstance.dismiss('cancel');
    };
  });
