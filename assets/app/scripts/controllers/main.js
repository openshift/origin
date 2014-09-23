'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:MainCtrl
 * @description
 * # MainCtrl
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('MainCtrl', function ($scope) {
    $scope.awesomeThings = [
      'HTML5 Boilerplate',
      'AngularJS',
      'Karma'
    ];
  });
