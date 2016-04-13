'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProcessTemplateModalController
 * @description
 * # ProcessTemplateModalController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProcessTemplateModalController', function ($scope, $uibModalInstance) {
    $scope.continue = function() {
      $uibModalInstance.close('create');
    };

    $scope.cancel = function() {
      $uibModalInstance.dismiss('cancel');
    };
  });
