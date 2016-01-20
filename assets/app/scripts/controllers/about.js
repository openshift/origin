'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:AboutController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('AboutController', function ($scope, DataService, AuthService, CLI_TOOLS, VERSION) {
  	$scope.version = {
  		api: {
  			openshift: DataService.oApiVersion,
  			kubernetes: DataService.k8sApiVersion,
  		},
  		master: {
  			openshift: {
	  			major: VERSION.openshift.major,
	  			minor: VERSION.openshift.minor,
	  			gitCommit: VERSION.openshift.gitCommit,
	  			gitVersion: VERSION.openshift.gitVersion
  			},
	  		kubernetes: {
	  			major: VERSION.kubernetes.major,
	  			minor: VERSION.kubernetes.minor,
	  			gitCommit: VERSION.kubernetes.gitCommit,
	  			gitVersion: VERSION.kubernetes.gitVersion
	  		}
  		},
  	};
  	$scope.cliDownloadURL = CLI_TOOLS.downloadURL;
  	$scope.cliDownloadURLPresent = CLI_TOOLS.downloadURL && !_.isEmpty(CLI_TOOLS.downloadURL);
    $scope.loginBaseURL = DataService.openshiftAPIBaseUrl();
    $scope.sessionToken = AuthService.UserStore().getToken();
  });
