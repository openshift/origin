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
  .controller('ConfirmScaleController', function ($scope, $modalInstance, resource, type) {
    $scope.resource = resource;
    $scope.type = type;

    $scope.confirmScale = function() {
      $modalInstance.close('confirmScale');
    };

    $scope.cancel = function() {
      $modalInstance.dismiss('cancel');
    };
  });
