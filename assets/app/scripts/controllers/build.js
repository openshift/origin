'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildController', function ($scope, $routeParams, DataService, project, BuildsService, $filter) {
    $scope.build = null;
    $scope.buildConfigName = $routeParams.buildconfig;
    $scope.builds = {};
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;
    $scope.breadcrumbs = [
      {
        title: "Builds",
        link: "project/" + $routeParams.project + "/browse/builds"
      }
    ];

    if ($routeParams.buildconfig) {
      $scope.breadcrumbs.push({
        title: $routeParams.buildconfig,
        link: "project/" + $routeParams.project + "/browse/builds/" + $routeParams.buildconfig
      });
    }

    $scope.breadcrumbs.push({
      title: $routeParams.build
    });

    // Check for a ?tab=<name> query param to allow linking directly to a tab.
    if ($routeParams.tab) {
      $scope.selectedTab = {};
      $scope.selectedTab[$routeParams.tab] = true;
    }

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("builds", $routeParams.build, $scope).then(
        // success
        function(build) {
          $scope.loaded = true;
          $scope.build = build;
          var buildNumber = $filter("annotation")(build, "buildNumber");
          if (buildNumber) {
            $scope.breadcrumbs[2].title = "#" + buildNumber;
          }

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("builds", $routeParams.build, $scope, function(build, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This build has been deleted."
              };
            }
            $scope.build = build;
          }));
        },
        // failure
        function(e) {
          $scope.loaded = true;
          $scope.alerts["load"] = {
            type: "error",
            message: "The build details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );

      watches.push(DataService.watch("builds", $scope, function(builds, action, build) {
        $scope.builds = {};
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
          if (!$filter('isIncompleteBuild')(build)){
            delete $scope.buildConfigBuildsInProgress[buildConfigName][buildName];
          }
        }
      }));


      var runLogs = function() {
        angular.extend($scope, {
          logs: [],
          logsLoading: true,
          canShowDownload: false,
          canInitAgain: false
        });

        var streamer = DataService.createStream('builds/log',$routeParams.build, $scope);
        streamer.onMessage(function(msg) {
          $scope.$apply(function() {
            $scope.logs.push({text: msg});
            $scope.canShowDownload = true;
          });
        });
        streamer.onClose(function() {
          $scope.$apply(function() {
            $scope.logsLoading = false;
          });
        });
        streamer.onError(function() {
          $scope.$apply(function() {
            angular.extend($scope, {
              logsLoading: false,
              logError: true
            });
          });
        });

        streamer.start();
        $scope.$on('$destroy', function() {
          streamer.stop();
        });
      };

      angular.extend($scope, {
        initLogs: _.once(runLogs),
        runLogs: runLogs
      });


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
