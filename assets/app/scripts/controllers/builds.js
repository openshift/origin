'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildsController', function ($scope, DataService, $filter, LabelFilter, Logger, $location, $anchorScroll) {
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
        $scope.buildConfigBuildsInProgress = associateRunningBuildToBuildConfig($scope.buildsByBuildConfig);
      } else if (action === 'ADDED'){
        // When new build id instantiated/cloned associate him to his buildConfig and add him into buildConfigBuildsInProgress structure.
        $scope.buildConfigBuildsInProgress[buildConfigName] = $scope.buildConfigBuildsInProgress[buildConfigName] || {};
        $scope.buildConfigBuildsInProgress[buildConfigName][buildName] = build;
      } else if (action === 'MODIFIED'){
        // After the build ends remove him from the buildConfigBuildsInProgress structure.
        var buildStatus = build.status.phase;
        if (buildStatus === "Complete" || buildStatus === "Failed" || buildStatus === "Error" || buildStatus === "Cancelled"){
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
    }

    function associateRunningBuildToBuildConfig(buildsByBuildConfig) {
      var buildConfigBuildsInProgress = {};
      angular.forEach(buildsByBuildConfig, function(buildConfigBuilds, buildConfigName) {
        buildConfigBuildsInProgress[buildConfigName] = {};
        angular.forEach(buildConfigBuilds, function(build, buildName) {
          var buildStatus = build.status.phase;
          if (buildStatus === "New" || buildStatus === "Pending" || buildStatus === "Running") {
            buildConfigBuildsInProgress[buildConfigName][buildName] = build;
          }
        });
      });
      return buildConfigBuildsInProgress;
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

    // Function which will 'instantiate' new build from given buildConfigName
    $scope.startBuild = function(buildConfigName) {
      var req = {
        kind: "BuildRequest",
        apiVersion: "v1",
        metadata: {
          name: buildConfigName
        }
      };
      DataService.create("buildconfigs/instantiate", buildConfigName, req, $scope).then(
        function(build) { //success
            $scope.alerts = [
            {
              type: "success",
              message: "Build " + build.metadata.name + " has started.",
            }
          ];
        },
        function(result) { //failure
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred while starting the build.",
              details: $filter('getErrorDetails')(result)
            }
          ];
        }
      );
    };

    // Function which will 'clone' build from given buildName
    $scope.cloneBuild = function(buildName) {
      var req = {
        kind: "BuildRequest",
        apiVersion: "v1",
        metadata: {
          name: buildName
        }
      };
      DataService.create("builds/clone", buildName, req, $scope).then(
        function(build) { //success
            $scope.alerts = [
            {
              type: "success",
              message: "Build " + buildName + " is being rebuilt as " + build.metadata.name + ".",
            }
          ];
        },
        function(result) { //failure
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred while rerunning the build.",
              details: $filter('getErrorDetails')(result)
            }
          ];
        }
      );
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
