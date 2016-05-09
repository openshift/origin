'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ConfirmReplaceModalController
 * @description
 * # ConfirmReplaceModalController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ConfirmReplaceModalController', function ($scope, $uibModalInstance) {
    $scope.replace = function() {
      $uibModalInstance.close('replace');
    };

    $scope.cancel = function() {
      $uibModalInstance.dismiss('cancel');
    };
  });
