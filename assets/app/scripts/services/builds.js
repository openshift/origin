'use strict';

angular.module("openshiftConsole")
  .factory("BuildsService", function(DataService, $filter){
    function BuildsService() {}

    // Function which will 'instantiate' new build from given buildConfigName
    BuildsService.prototype.startBuild = function(buildConfigName, context, $scope) {
      var req = {
        kind: "BuildRequest",
        apiVersion: "v1",
        metadata: {
          name: buildConfigName
        }
      };
      DataService.create("buildconfigs/instantiate", buildConfigName, req, context).then(
        function(build) { //success
            $scope.alerts = $scope.alerts || {};
            $scope.alerts["create"] =
              {
                type: "success",
                message: "Build " + build.metadata.name + " has started."
              };
        },
        function(result) { //failure
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["create"] =
            {
              type: "error",
              message: "An error occurred while starting the build.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    BuildsService.prototype.cancelBuild = function(build, buildConfigName, context, $scope) {
      var canceledBuild = angular.copy(build);
      canceledBuild.status.cancelled = true;
      DataService.update("builds", canceledBuild.metadata.name, canceledBuild, context).then(
        function() {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["cancel"] =
            {
              type: "success",
              message: "Cancelling build " + build.metadata.name + " of " + buildConfigName + "."
            };
        },
        function(result) {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["cancel"] =
            {
              type: "error",
              message: "An error occurred cancelling the build.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    // Function which will 'clone' build from given buildName
    BuildsService.prototype.cloneBuild = function(buildName, context, $scope) {
      var req = {
        kind: "BuildRequest",
        apiVersion: "v1",
        metadata: {
          name: buildName
        }
      };
      DataService.create("builds/clone", buildName, req, context).then(
        function(build) { //success
            $scope.alerts = $scope.alerts || {};
            $scope.alerts["rebuild"] =
            {
              type: "success",
              message: "Build " + buildName + " is being rebuilt as " + build.metadata.name + ".",
              links: [{
                href: $filter('navigateResourceURL')(build) + "?tab=logs",
                label: "View Log"
              }]
            };
        },
        function(result) { //failure
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["rebuild"] =
            {
              type: "error",
              message: "An error occurred while rerunning the build.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    BuildsService.prototype.associateRunningBuildToBuildConfig = function(builds) {
      var buildConfigBuildsInProgress = {};
      angular.forEach(builds, function(build, buildName) {
        if ($filter('isIncompleteBuild')(build)) {
          var buildConfigName = build.metadata.labels.buildconfig;
          buildConfigBuildsInProgress[buildConfigName] = buildConfigBuildsInProgress[buildConfigName] || {};
          buildConfigBuildsInProgress[buildConfigName][buildName] = build;
        }
      });
      return buildConfigBuildsInProgress;
    };

    BuildsService.prototype.isPaused = function(buildConfig) {
      return $filter('annotation')(buildConfig, "openshift.io/build-config.paused") === 'true';
    };

    BuildsService.prototype.canBuild = function(buildConfig, buildConfigBuildsInProgressMap) {
      if (!buildConfig) {
        return false;
      }

      if (buildConfig.metadata.deletionTimestamp) {
        return false;
      }

      if (buildConfigBuildsInProgressMap &&
          $filter('hashSize')(buildConfigBuildsInProgressMap[buildConfig.metadata.name]) > 0) {
        return false;
      }

      if (this.isPaused(buildConfig)) {
        return false;
      }

      return true;
    };

    return new BuildsService();
  });
