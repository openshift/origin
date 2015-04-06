'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ErrorController
 * @description
 * # ErrorController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ErrorController', function ($scope) {
    var params = URI(window.location.href).query(true);
    var error = params.error;
    var error_description = params.error_description;
    var error_uri = params.error_uri;

    switch(error) {
      case 'access_denied':
        $scope.errorMessage = "Access denied";
        break;
      case 'not_found':
        $scope.errorMessage = "Not found";
        break;
      case 'invalid_request':
        $scope.errorMessage = "Invalid request";
        break;
      default:
        $scope.errorMessage = "An error has occurred";
    }

    if (params.error_description) {
      $scope.errorDetails = params.error_description;
    }
  });
