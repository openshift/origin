'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectsController
 * @description
 * # ProjectsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectsController', function ($scope, $route, $timeout, $filter, DataService, AuthService, Logger, hashSizeFilter) {
    $scope.projects = {};
    $scope.alerts = $scope.alerts || {};
    $scope.showGetStarted = false;
    $scope.canCreate = undefined;
    $scope.confirmingMap = {};
    $scope.deletingMap = {};

    $('#openshift-logo').on('click.projectsPage', function() {
      // Force a reload. Angular doesn't reload the view when the URL doesn't change.
      $route.reload();
    });

    $scope.$on('$destroy', function(){
      // The click handler is only necessary on the projects page.
      $('#openshift-logo').off('click.projectsPage');
    });

    $scope.toggleConfirm = function(project) {
      // toggle prompt for user to decide if they are sure about deletion
      var projectName = project.metadata.name;

      // disable the trash button if current project is being deleted
      if (!$scope.deletingMap[projectName]) {
        $scope.confirmingMap[projectName] = !$scope.confirmingMap[projectName];
      }
    };

    $scope.deleteProject = function(project) {
      // actually deleting the project
      var projectName = project.metadata.name;
      delete $scope.alerts[projectName];
      $scope.confirmingMap[projectName] = false;
      $scope.deletingMap[projectName] = true;
      DataService.delete('projects', projectName, $scope)
      .then(function() {
        // called if successful deletion
        $scope.alerts[projectName] = {
          type: "success",
          message: "Project " + $filter('displayName')(project) + " was successfully deleted."
        };
        delete $scope.projects[projectName];
      })
      .catch(function(err) {
        // called if failure to delete
        $scope.alerts[projectName] = {
          type: "error",
          message: "Project " + $filter('displayName')(project) + " could not be deleted.",
          details: err
        };
        Logger.error("Project " + $filter('displayName')(project) + " could not be deleted.", err);
      })
      .finally(function() {
        // common stuff
        $scope.deletingMap[projectName] = false;
        $timeout(function() {
          delete $scope.alerts[projectName];
        }, 10000);
      });
    };

    AuthService.withUser().then(function() {
      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        $scope.showGetStarted = hashSizeFilter($scope.projects) === 0;
      });
    });

    // Test if the user can submit project requests. Handle error notifications
    // ourselves because 403 responses are expected.
    DataService.get("projectrequests", null, $scope, { errorNotification: false})
    .then(function() {
      $scope.canCreate = true;
    }, function(result) {
      $scope.canCreate = false;

      var data = result.data || {};

      // 403 Forbidden indicates the user doesn't have authority.
      // Any other failure status is an unexpected error.
      if (result.status !== 403) {
        var msg = 'Failed to determine create project permission';
        if (result.status !== 0) {
          msg += " (" + result.status + ")";
        }
        Logger.warn(msg);
        return;
      }

      // Check if there are detailed messages. If there are, show them instead of our default message.
      if (data.details) {
        var messages = [];
        _.forEach(data.details.causes || [], function(cause) {
          if (cause.message) { messages.push(cause.message); }
        });
        if (messages.length > 0) {
          $scope.newProjectMessage = messages.join("\n");
        }
      }
    });
  });