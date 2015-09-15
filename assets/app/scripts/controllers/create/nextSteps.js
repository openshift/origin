"use strict";
/* jshint unused: false */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:NextStepsController
 * @description
 * # NextStepsController
 * Controller of the openshiftConsole
 */
angular.module("openshiftConsole")
  .controller("NextStepsController", function($scope, $http, $routeParams, DataService, $q, $location, TaskList, $parse, Navigate, $filter, imageObjectRefFilter, failureObjectNameFilter) {
    var displayNameFilter = $filter('displayName');
    var watches = [];

    $scope.emptyMessage = "Loading...";
    $scope.alerts = [];
    $scope.loginBaseUrl = DataService.openshiftAPIBaseUrl();
    $scope.projectName = $routeParams.project;
    $scope.buildConfigs = {};
    $scope.projectPromise = $.Deferred();
    DataService.get("projects", $scope.projectName, $scope).then(function(project) {
      $scope.project = project;
      $scope.projectPromise.resolve(project);
    });

    var name = $routeParams.name;
    if (!name || (!creatingFromTemplate($routeParams) && !(creatingFromImage($routeParams)))) {
      Navigate.toProjectOverview($scope.projectName);
      return;
    }

    $scope.name = name;

    watches.push(DataService.watch("buildconfigs", $scope, function(buildconfigs) {
      $scope.buildConfigs = buildconfigs.by("metadata.name");
      $scope.createdBuildConfig = $scope.buildConfigs[name];
      Logger.log("buildconfigs (subscribe)", $scope.buildConfigs);
    }));

    $scope.createdBuildConfigWithGitHubTrigger = function() {
      var created = false;
      if ($scope.createdBuildConfig) {
        angular.forEach($scope.createdBuildConfig.spec.triggers, function(trigger) {
          if (trigger.type == "GitHub") {
            created = true;
          }
        });
      }
      return created;
    };

    $scope.allTasksSuccessful = function(tasks) {
      return !pendingTasks(tasks).length && !erroredTasks(tasks).length;
    };

    $scope.projectDisplayName = function() {
      return displayNameFilter(this.project) || this.projectName;
    };

    function erroredTasks(tasks) {
      var erroredTasks = [];
      angular.forEach(tasks, function(task) {
        if (task.hasErrors) {
          erroredTasks.push(task);
        }
      });
      return erroredTasks;
    };
    $scope.erroredTasks = erroredTasks;

    function pendingTasks(tasks) {
      var pendingTasks = [];
      angular.forEach(tasks, function(task) {
        if (task.status !== "completed") {
          pendingTasks.push(task);
        }
      });
      return pendingTasks;
    };
    $scope.pendingTasks = pendingTasks;

    $scope.goBack = function() {
      if (creatingFromImage()) {
        $location.path("project/" + encodeURIComponent(this.projectName) + "/create/fromimage");
      } else {
        $location.path("project/" + encodeURIComponent(this.projectName) + "/create/fromtemplate");
      }
    };

    function creatingFromTemplate() {
      return $routeParams.name && $routeParams.namespace;
    }

    function creatingFromImage() {
      return $routeParams.imageName && $routeParams.imageTag && $routeParams.namespace && $routeParams.sourceURL;
    }

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
