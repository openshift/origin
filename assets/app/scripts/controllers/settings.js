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
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessageQuotas = "Loading...";
    $scope.emptyMessageLimitRanges = "Loading...";
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;

    var watches = [];

    DataService.list("resourcequotas", $scope, function(quotas) {
      $scope.quotas = quotas.by("metadata.name");
      $scope.emptyMessageQuotas = "There are no resource quotas set on this project.";
      Logger.log("quotas", $scope.quotas);
    });

    DataService.list("limitranges", $scope, function(limitRanges) {
      $scope.limitRanges = limitRanges.by("metadata.name");
      $scope.emptyMessageLimitRanges = "There are no resource limits set on this project.";
      // Make sure max and min have the same sets of keys so we can actually create a table
      // cleanly from a view.
      angular.forEach($scope.limitRanges, function(limitRange, name){
        angular.forEach(limitRange.spec.limits, function(limit) {
          limit.min = limit.min || {};
          limit.max = limit.max || {};
          limit["default"] = limit["default"] || {};
          angular.forEach(limit.max, function(value, type) {
            limit.min[type] = limit.min[type] || "";
            limit["default"][type] = limit["default"][type] || "";
          });
          angular.forEach(limit.min, function(value, type) {
            limit.max[type] = limit.max[type] || "";
            limit["default"][type] = limit["default"][type] || "";
          });
          angular.forEach(limit["default"], function(value, type) {
            limit.max[type] = limit.max[type] || "";
            limit.min[type] = limit.min[type] || "";
          });
        });
      });
      Logger.log("limitRanges", $scope.limitRanges);
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
