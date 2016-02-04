'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('AboutController', function ($scope, DataService, AuthService, Constants) {
    AuthService.withUser();
    
    $scope.version = {
      master: {
        openshift: Constants.VERSION.openshift,
        kubernetes: Constants.VERSION.kubernetes,
      },
    };
    $scope.cliDownloadURL = Constants.CLI;
    $scope.cliDownloadURLPresent = $scope.cliDownloadURL && !_.isEmpty($scope.cliDownloadURL);
    $scope.loginBaseURL = DataService.openshiftAPIBaseUrl();
    $scope.sessionToken = AuthService.UserStore().getToken();
  });
