'use strict';
/* jshint unused: false */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServicesController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('SettingsController', function ($routeParams, $scope, DataService, ProjectsService, AlertMessageService, $filter, $location, LabelFilter, $timeout, Logger, annotationFilter, annotationNameFilter) {
    $scope.projectName = $routeParams.project;
    $scope.quotas = {};
    $scope.limitRanges = {};
    $scope.limitsByType = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessageQuotas = "Loading...";
    $scope.quotaHelp = "Limits resource usage within the project.";
    $scope.emptyMessageLimitRanges = "Loading...";
    $scope.limitRangeHelp = "Defines minimum and maximum constraints for runtime resources such as memory and CPU.";
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;

    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {

        var editableFields = function(resource) {
          return {
            description: annotationFilter(resource, 'description'),
            displayName: annotationFilter(resource, 'displayName')
          };
        };

        var mergeEditable = function(resource, editable) {
          var toSubmit = angular.copy(resource);
          toSubmit.metadata.annotations[annotationNameFilter('description')] = editable.description;
          toSubmit.metadata.annotations[annotationNameFilter('displayName')] = editable.displayName;
          return toSubmit;
        };

        angular.extend($scope, {
          project: project,
          editableFields: editableFields(project),
          show: {
            editing: false
          },
          actions: {
            canSubmit: false
          },
          canSubmit: function(bool) {
            $scope.actions.canSubmit = bool;
          },
          setEditing: function(bool) {
            $scope.show.editing = bool;
          },
          cancel: function() {
            $scope.setEditing(false);
            $scope.editableFields = editableFields(project);
          },
          update: function() {
            $scope.setEditing(false);
            ProjectsService
              .update($routeParams.project, mergeEditable(project, $scope.editableFields))
              .then(function(updated) {
                project = $scope.project = updated;
                $scope.editableFields = editableFields(updated);
                $scope.$emit('project.settings.update', updated);
              }, function(result) {
                $scope.editableFields = editableFields(project);
                $scope.alerts["update"] = {
                  type: "error",
                  message: "An error occurred while updating the project",
                  details: $filter('getErrorDetails')(result)
                };
              });
          }
        });

        DataService.list("resourcequotas", context, function(quotas) {
          $scope.quotas = quotas.by("metadata.name");
          $scope.emptyMessageQuotas = "There are no resource quotas set on this project.";
          Logger.log("quotas", $scope.quotas);
        });

        DataService.list("limitranges", context, function(limitRanges) {
          $scope.limitRanges = limitRanges.by("metadata.name");
          $scope.emptyMessageLimitRanges = "There are no limit ranges set on this project.";
          // Convert to a sane format for a view to a build a table with rows per resource type
          angular.forEach($scope.limitRanges, function(limitRange, name){
            $scope.limitsByType[name] = {};

            angular.forEach(limitRange.spec.limits, function(limit) {
              // We have nested types, top level type is something like "Container"
              var typeLimits = $scope.limitsByType[name][limit.type] = {};
              angular.forEach(limit.max, function(value, type) {
                typeLimits[type] = typeLimits[type] || {};
                typeLimits[type].max = value;
              });
              angular.forEach(limit.min, function(value, type) {
                typeLimits[type] = typeLimits[type] || {};
                typeLimits[type].min = value;
              });
              angular.forEach(limit["default"], function(value, type) {
                typeLimits[type] = typeLimits[type] || {};
                typeLimits[type]["default"] = value;
              });
              angular.forEach(limit.defaultRequest, function(value, type) {
                typeLimits[type] = typeLimits[type] || {};
                typeLimits[type].defaultRequest = value;
              });
              angular.forEach(limit.maxLimitRequestRatio, function(value, type) {
                typeLimits[type] = typeLimits[type] || {};
                typeLimits[type].maxLimitRequestRatio = value;
              });
            });
          });
          Logger.log("limitRanges", $scope.limitRanges);
        });

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });

      }));
  });
