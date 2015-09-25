'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildsController', function ($scope, DataService, $filter, LabelFilter, Logger, $location, $anchorScroll, BuildsService) {
    $scope.builds = {};
    $scope.unfilteredBuilds = {};
    $scope.buildConfigs = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    // Expand builds on load if there's a hash that might link to a hidden build.
    $scope.expanded = !!$location.hash();
    // Show only 3 builds for each build config by default.
    $scope.defaultBuildLimit = 3;

    $scope.buildsByBuildConfig = {};
    $scope.expandedBuildConfigRow = {};

    var watches = [];

    watches.push(DataService.watch("builds", $scope, function(builds, action, build) {
      $scope.unfilteredBuilds = builds.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredBuilds, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.builds = LabelFilter.getLabelSelector().select($scope.unfilteredBuilds);
      $scope.emptyMessage = "No builds to show";
      associateBuildsToBuildConfig();
      updateFilterWarning();

      var buildConfigName;
      var buildName;
      if (build) {
        buildConfigName = build.metadata.labels.buildconfig;
        buildName = build.metadata.name;
      }
      if (!action) {
        // Loading of the page that will create buildConfigBuildsInProgress structure, which will associate running build to his buildConfig.
        $scope.buildConfigBuildsInProgress = BuildsService.associateRunningBuildToBuildConfig($scope.unfilteredBuilds);
      } else if (action === 'ADDED'){
        // When new build id instantiated/cloned associate him to his buildConfig and add him into buildConfigBuildsInProgress structure.
        $scope.buildConfigBuildsInProgress[buildConfigName] = $scope.buildConfigBuildsInProgress[buildConfigName] || {};
        $scope.buildConfigBuildsInProgress[buildConfigName][buildName] = build;
      } else if (action === 'MODIFIED'){
        // After the build ends remove him from the buildConfigBuildsInProgress structure.
        if (!$filter('isIncompleteBuild')(build) && $scope.buildConfigBuildsInProgress[buildConfigName]){
          delete $scope.buildConfigBuildsInProgress[buildConfigName][buildName];
        }
      }

      // Scroll to anchor on first load if location has a hash.
      if (!action && $location.hash()) {
        // Wait until the digest loop completes.
        setTimeout($anchorScroll, 10);
      }

      Logger.log("builds (subscribe)", $scope.unfilteredBuilds);
    }));

    watches.push(DataService.watch("buildconfigs", $scope, function(buildConfigs) {
      $scope.buildConfigs = buildConfigs.by("metadata.name");
      associateBuildsToBuildConfig();
      Logger.log("buildconfigs (subscribe)", $scope.buildConfigs);
    }));

    function associateBuildsToBuildConfig() {
      $scope.buildsByBuildConfig = {};
      angular.forEach($scope.builds, function(build, buildName) {
        var buildConfigName = "";
        if (build.metadata.labels) {
          buildConfigName = build.metadata.labels.buildconfig || "";
        }
        $scope.buildsByBuildConfig[buildConfigName] = $scope.buildsByBuildConfig[buildConfigName] || {};
        $scope.buildsByBuildConfig[buildConfigName][buildName] = build;
      });
      // Make sure there is an empty hash for every build config we know about
      angular.forEach($scope.buildConfigs, function(buildConfig, buildConfigName){
        $scope.buildsByBuildConfig[buildConfigName] = $scope.buildsByBuildConfig[buildConfigName] || {};
      });
    }

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.builds) && !$.isEmptyObject($scope.unfilteredBuilds)) {
        $scope.alerts["builds"] = {
          type: "warning",
          details: "The active filters are hiding all builds."
        };
      }
      else {
        delete $scope.alerts["builds"];
      }
    }

    $scope.startBuild = function(buildConfigName) {
      BuildsService.startBuild(buildConfigName, $scope);
    };

    $scope.cancelBuild = function(build, buildConfigName) {
      BuildsService.cancelBuild(build, buildConfigName, $scope);
    };

    $scope.cloneBuild = function(buildName) {
      BuildsService.cloneBuild(buildName, $scope);
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.builds = labelSelector.select($scope.unfilteredBuilds);
        associateBuildsToBuildConfig();
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
