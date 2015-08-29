'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectsController
 * @description
 * # ProjectsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectsController', function ($scope, $route, $filter, $location, $modal, DataService, AuthService, AlertMessageService, Logger, hashSizeFilter) {
    $scope.projects = {};
    $scope.alerts = $scope.alerts || {};
    $scope.showGetStarted = false;
    $scope.canCreate = undefined;

    AlertMessageService.getAlerts().forEach(function(alert) {
      $scope.alerts[alert.name] = alert.data;
    });
    AlertMessageService.clearAlerts();

    $('#openshift-logo').on('click.projectsPage', function() {
      // Force a reload. Angular doesn't reload the view when the URL doesn't change.
      $route.reload();
    });

    $scope.$on('$destroy', function(){
      // The click handler is only necessary on the projects page.
      $('#openshift-logo').off('click.projectsPage');
    });

    $scope.openDeleteModal = function(project) {
      // opening the modal with settings scope as parent
      $scope.project = project;
      var modalInstance = $modal.open({
        animation: true,
        templateUrl: 'views/modals/delete-project.html',
        controller: 'DeleteModalController',
        scope: $scope
      });

      modalInstance.result.then(function() {
        // actually deleting the project
        var projectName = project.metadata.name;
        delete $scope.alerts[projectName];
        DataService.delete('projects', projectName, $scope)
        .then(function() {
          // called if successful deletion
          $scope.alerts[projectName] = {
            type: "success",
            message: "Project " + $filter('displayName')(project) + " was marked for deletion."
          };

          loadProjects();
        })
        .catch(function(err) {
          // called if failure to delete
          $scope.alerts[projectName] = {
            type: "error",
            message: "Project " + $filter('displayName')(project) + " could not be deleted.",
            details: getErrorDetails(err)
          };
          Logger.error("Project " + $filter('displayName')(project) + " could not be deleted.", getErrorDetails(err));
        })
        .finally(function() {
          $scope.project = {};
        });
      });
    };

    AuthService.withUser().then(function() {
      loadProjects();
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

    var getErrorDetails = function(result) {
      var error = result.data || {};
      if (error.message) {
        return error.message;
      }

      var status = result.status || error.status;
      if (status) {
        return "Status: " + status;
      }

      return "";
    };

    var loadProjects = function() {
      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        $scope.showGetStarted = hashSizeFilter($scope.projects) === 0;
      });
    };
  });