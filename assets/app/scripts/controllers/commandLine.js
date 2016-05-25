'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:CommandLineController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CommandLineController', function ($scope, DataService, AuthService, Constants) {
    AuthService.withUser();

    $scope.cliDownloadURL = Constants.CLI;
    $scope.cliDownloadURLPresent = $scope.cliDownloadURL && !_.isEmpty($scope.cliDownloadURL);
    $scope.loginBaseURL = DataService.openshiftAPIBaseUrl();
    $scope.sessionToken = AuthService.UserStore().getToken();
    $scope.showSessionToken = false;

    $scope.toggleShowSessionToken = function() {
      $scope.showSessionToken = !$scope.showSessionToken;
    };
  });
