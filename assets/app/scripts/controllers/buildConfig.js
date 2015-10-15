'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildConfigController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildConfigController', function ($scope, $routeParams, DataService, project, BuildsService, $filter) {
    $scope.buildConfig = null;
    $scope.builds = {};
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};    
    $scope.renderOptions.hideFilterWidget = true;    
    $scope.breadcrumbs = [
      {
        title: "Builds",
        link: "project/" + $routeParams.project + "/browse/builds"
      },
      {
        title: $routeParams.buildconfig
      }
    ];
    $scope.emptyMessage = "Loading...";

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("buildconfigs", $routeParams.buildconfig, $scope).then(
        // success
        function(buildConfig) {
          $scope.loaded = true;
          $scope.buildConfig = buildConfig;

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("buildconfigs", $routeParams.buildconfig, $scope, function(buildConfig, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This build configuration has been deleted."
              }; 
            }
            $scope.buildConfig = buildConfig;
          }));           
        },
        // failure
        function(e) {
          $scope.loaded = true;
          $scope.alerts["load"] = {
            type: "error",
            message: "The build configuration details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );

      watches.push(DataService.watch("builds", $scope, function(builds, action, build) {
        $scope.builds = {};
        $scope.emptyMessage = "No builds to show";
        // TODO we should send the ?labelSelector=buildconfig=<name> on the API request
        // to only load the buildconfig's builds, but this requires some DataService changes
        var allBuilds = builds.by("metadata.name");
        angular.forEach(allBuilds, function(build, name) {
          if (build.metadata.labels && build.metadata.labels.buildconfig === $routeParams.buildconfig) {
            $scope.builds[name] = build;
          }
        });

        var buildConfigName;
        var buildName;
        if (build) {
          if (build.metadata.labels) {
            buildConfigName = build.metadata.labels.buildconfig;
          }
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
      }));
    });

    $scope.startBuild = function(buildConfigName) {
      BuildsService.startBuild(buildConfigName, $scope);
    };

    $scope.cancelBuild = function(build, buildConfigName) {
      BuildsService.cancelBuild(build, buildConfigName, $scope);
    };

    $scope.cloneBuild = function(buildName) {
      BuildsService.cloneBuild(buildName, $scope);
    };

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
