'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:CreateProjectController
 * @description
 * # CreateProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CreateProjectController', function ($scope, DataService, Notification, Navigate) {
    $scope.createProject = function() {
      if ($scope.createProjectForm.$valid) {
        DataService.create('projectrequests', null, {
          apiVersion: "v1",
          kind: "ProjectRequest",
          metadata: {
            name: $scope.name,
          },
          displayName: $scope.displayName,
          description: $scope.description
        }, $scope).then(function(data) { // Success
          Navigate.toProjectOverview(data.metadata.name);
        }, function(result) { // Failure
          var data = result.data || {};
          if (data.reason === 'AlreadyExists') {
            $scope.nameTaken = true;
          } else {
            var msg = data.message || 'An error occurred creating the project.';
            Notification.error(msg, { mustDismiss: true });
          }
        });
      }
    };
  });
