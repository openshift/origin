'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:LogoutController
 * @description
 * # LogoutController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('LogoutController', function ($scope, $log, AuthService) {
    $log.debug("LogoutController");

    if (AuthService.isLoggedIn()) {
      $log.debug("LogoutController, logged in, initiating logout");
      $scope.logoutMessage = "Logging out...";

      AuthService.startLogout().finally(function(){
        // Make sure the logout completed
        if (AuthService.isLoggedIn()) {
          $log.debug("LogoutController, logout failed, still logged in");
          $scope.logoutMessage = 'You could not be logged out. Return to the <a href="/">console</a>.';
        } else {
          // TODO: redirect to configurable logout destination
          $log.debug("LogoutController, logout completed, reloading the page");
          window.location.reload(false);
        }
      });
    } else {
      // TODO: redirect to configurable logout destination
      $log.debug("LogoutController, not logged in, logout complete");
      $scope.logoutMessage = 'You are logged out. Return to the <a href="/">console</a>.';
    }
  });
