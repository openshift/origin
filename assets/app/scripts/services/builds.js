'use strict';

angular.module("openshiftConsole")
  .factory("BuildsService", function(DataService, $filter){
    return {
      // Function which will 'instantiate' new build from given buildConfigName
      startBuild: function(buildConfigName, context) {
        var req = {
          kind: "BuildRequest",
          apiVersion: "v1",
          metadata: {
            name: buildConfigName
          }
        };
        return DataService.create("buildconfigs/instantiate", buildConfigName, req, context);
      },
      cancelBuild: function(build, buildConfigName, context) {
        var canceledBuild = angular.copy(build);
        canceledBuild.status.cancelled = true;
        return DataService.update("builds", canceledBuild.metadata.name, canceledBuild, context);
      },
      // Function which will 'clone' build from given buildName
      cloneBuild: function(buildName, context) {
        var req = {
          kind: "BuildRequest",
          apiVersion: "v1",
          metadata: {
            name: buildName
          }
        };
        return DataService.create("builds/clone", buildName, req, context);
      },
      associateRunningBuildToBuildConfig: function(builds) {
        var buildConfigBuildsInProgress = {};
        angular.forEach(builds, function(build, buildName) {
          if ($filter('isIncompleteBuild')(build)) {
            var buildConfigName = build.metadata.labels.buildconfig;
            buildConfigBuildsInProgress[buildConfigName] = buildConfigBuildsInProgress[buildConfigName] || {};
            buildConfigBuildsInProgress[buildConfigName][buildName] = build;
          }
        });
        return buildConfigBuildsInProgress;
      }
    };
  });
