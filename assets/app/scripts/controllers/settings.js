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
  .controller('SettingsController', function ($scope, DataService, AlertMessageService, $filter, $modal, $location, LabelFilter, $timeout, Logger) {
    $scope.quotas = {};
    $scope.limitRanges = {};
    $scope.limitsByType = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessageQuotas = "Loading...";
    $scope.emptyMessageLimitRanges = "Loading...";
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;

    var watches = [];

    $scope.openDeleteModal = function() {
      // opening the modal with settings scope as parent
      var modalInstance = $modal.open({
        animation: true,
        templateUrl: 'views/modals/delete-project.html',
        controller: 'DeleteModalController',
        scope: $scope
      });

      modalInstance.result.then(function(result) {
      /* upon clicking delete button, redirect to the home page,
         and keep alert in AlertMessageService to show alert
         that project has been marked for deletion */
        var projectName = $scope.project.metadata.name;
        DataService.delete('projects', projectName, $scope)
        .then(function() {
          var redirect = URI('/');
          AlertMessageService.addAlert({
            name: $scope.project.metadata.name,
            data: {
              type: "success",
              message: "Project " + $filter('displayName')($scope.project) + " was marked for deletion."
            }
          });
          $location.url(redirect);
        })
        .catch(function(err) {
          // called if failure to delete
          $scope.alerts[projectName] = {
            type: "error",
            message: "Project " + $filter('displayName')($scope.project) + " could not be deleted.",
            details: getErrorDetails(err)
          };
          Logger.error("Project " + $filter('displayName')($scope.project) + " could not be deleted.", err);
        });
      });
    };

    DataService.list("resourcequotas", $scope, function(quotas) {
      $scope.quotas = quotas.by("metadata.name");
      $scope.emptyMessageQuotas = "There are no resource quotas set on this project.";
      Logger.log("quotas", $scope.quotas);
    });

    DataService.list("limitranges", $scope, function(limitRanges) {
      $scope.limitRanges = limitRanges.by("metadata.name");
      $scope.emptyMessageLimitRanges = "There are no resource limits set on this project.";
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
  });
