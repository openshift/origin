'use strict';
/* jshint eqeqeq: false */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectController', function ($scope, $routeParams, DataService, AuthService, $filter, LabelFilter, $location) {

    $scope.projectName = $routeParams.project;
    $scope.project = {};
    $scope.projectPromise = $.Deferred();
    $scope.projects = {};
    $scope.alerts = {};
    $scope.renderOptions = {
      hideFilterWidget: false,
      showSidebarRight: false
    };

    /* The view mode of the overview project page */
    $scope.overviewMode = 'tiles';

    AuthService.withUser().then(function() {
      DataService.get("projects", $scope.projectName, $scope, {errorNotification: false}).then(
        // success
        function(project) {
          $scope.project = project;
          $scope.projectPromise.resolve(project);
        },
        // failure
        function(e) {
          $scope.projectPromise.reject(e);
          if (e.status == 403 || e.status == 404) {
            var message = e.status == 403 ?
              ("The project " + $scope.projectName + " does not exist or you are not authorized to view it.") :
              ("The project " + $scope.projectName + " does not exist.");
            var redirect = URI('error').query({
              "error_description": message,
              "error" : e.status == 403 ? 'access_denied' : 'not_found'
            }).toString();
            $location.url(redirect);
          }
          else {
            // Something spurious has happened, stay on the page and show an alert
            $scope.alerts["load"] = {
              type: "error",
              message: "The project could not be loaded.",
              details: e.data
            };
          }
        }
      );
    });
  });
