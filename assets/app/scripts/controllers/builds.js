'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildsController', function ($scope, DataService, $filter, LabelFilter, Logger) {
    $scope.builds = {};
    $scope.unfilteredBuilds = {};
    $scope.buildConfigs = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";

    $scope.buildsByBuildConfig = {};

    var watches = [];

    watches.push(DataService.watch("builds", $scope, function(builds) {
      $scope.unfilteredBuilds = builds.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredBuilds, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.builds = LabelFilter.getLabelSelector().select($scope.unfilteredBuilds);
      $scope.emptyMessage = "No builds to show";
      updateFilterWarning();

      $scope.buildsByBuildConfig = {};
      angular.forEach($scope.builds, function(build, buildName) {
        var buildConfigName = "";
        if (build.metadata.labels) {
          buildConfigName = build.metadata.labels.buildconfig || "";
        }
        $scope.buildsByBuildConfig[buildConfigName] = $scope.buildsByBuildConfig[buildConfigName] || {};
        $scope.buildsByBuildConfig[buildConfigName][buildName] = build;
      });

      Logger.log("builds (subscribe)", $scope.unfilteredBuilds);
    }));

    watches.push(DataService.watch("buildConfigs", $scope, function(buildConfigs) {
      $scope.buildConfigs = buildConfigs.by("metadata.name");
      Logger.log("buildConfigs (subscribe)", $scope.buildConfigs);
    }));    

    var updateFilterWarning = function() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.builds) && !$.isEmptyObject($scope.unfilteredBuilds)) {
        $scope.alerts["builds"] = {
          type: "warning",
          details: "The active filters are hiding all builds."
        };
      }
      else {
        delete $scope.alerts["builds"];
      }      
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.builds = labelSelector.select($scope.unfilteredBuilds);
        updateFilterWarning();
      });
    });   

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });