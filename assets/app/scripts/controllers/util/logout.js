'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:LogoutController
 * @description
 * # LogoutController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('LogoutController', function ($rootScope, $scope, AuthService) {
  	// If clearing the user results in a change from authenticated to unauthenticated, force the page in response
  	AuthService.onUserChanged(function(){
  		console.log("LogoutController - user changed, reloading the page");
  		window.location.reload(false);
  	});

  	// TODO: actually run the logout flow, delete the token, etc
  	AuthService.setUser(null, null);
  });
