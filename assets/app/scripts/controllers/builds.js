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
    $scope.unfilteredBuildConfigs = {};
    $scope.buildConfigs = undefined;
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
      $scope.builds = builds.by("metadata.name");
      $scope.emptyMessage = "No builds to show";
      associateBuildsToBuildConfig();

      var buildConfigName;
      var buildName;
      if (build) {
        buildConfigName = build.metadata.labels.buildconfig;
        buildName = build.metadata.name;
      }
      if (!action) {
        // Loading of the page that will create buildConfigBuildsInProgress structure, which will associate running build to his buildConfig.
        $scope.buildConfigBuildsInProgress = BuildsService.associateRunningBuildToBuildConfig($scope.builds);
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

      Logger.log("builds (subscribe)", $scope.builds);
    }));

    watches.push(DataService.watch("buildconfigs", $scope, function(buildConfigs) {
      $scope.unfilteredBuildConfigs = buildConfigs.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredBuildConfigs, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.buildConfigs = LabelFilter.getLabelSelector().select($scope.unfilteredBuildConfigs);      
      associateBuildsToBuildConfig();
      updateFilterWarning();
      Logger.log("buildconfigs (subscribe)", $scope.buildConfigs);
    }));

    // Used to determine whether the build should be added to the buildsByBuildConfig map
    // based on current filtering state
    function showBuild(build) {
      // If we aren't filtering by labels, show the build
      var labelSelector = LabelFilter.getLabelSelector();
      if (labelSelector.isEmpty()) {
        return true;
      }

      // If we are filtering, and the build is owned by a build config
      // then the build config will control whether the row is shown
      var buildConfigName = "";
      if (build.metadata.labels) {
        buildConfigName = build.metadata.labels.buildconfig || "";
      }
      if (buildConfigName) {
        return !!$scope.buildConfigs[buildConfigName];
      }

      // Otherwise this build has no build config and so will have its own
      // row, so see if the current filter matches it
      return labelSelector.matches(build);
    }

    function associateBuildsToBuildConfig() {
      $scope.buildsByBuildConfig = {};
      angular.forEach($scope.builds, function(build, buildName) {
        var buildConfigName = "";
        if (build.metadata.labels) {
          buildConfigName = build.metadata.labels.buildconfig || "";
        }
        if (showBuild(build)) {
          $scope.buildsByBuildConfig[buildConfigName] = $scope.buildsByBuildConfig[buildConfigName] || {};
          $scope.buildsByBuildConfig[buildConfigName][buildName] = build;
        }
      });
      // Make sure there is an empty hash for every build config we know about
      angular.forEach($scope.buildConfigs, function(buildConfig, buildConfigName){
        $scope.buildsByBuildConfig[buildConfigName] = $scope.buildsByBuildConfig[buildConfigName] || {};
      });
    }

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.buildsByBuildConfig)) {
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
        $scope.buildConfigs = labelSelector.select($scope.unfilteredBuildConfigs);
        associateBuildsToBuildConfig();
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
