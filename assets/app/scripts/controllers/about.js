'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('AboutController', function ($scope, AuthService, Constants) {
    AuthService.withUser();
    
    $scope.version = {
      master: {
        openshift: Constants.VERSION.openshift,
        kubernetes: Constants.VERSION.kubernetes,
      },
    };
  });
