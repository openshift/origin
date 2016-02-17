'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildController', function ($scope, $routeParams, DataService, ProjectsService, BuildsService, $filter) {
    $scope.projectName = $routeParams.project;
    $scope.build = null;
    $scope.buildConfigName = $routeParams.buildconfig;
    $scope.builds = {};
    $scope.alerts = {};
    $scope.renderOptions = {
      hideFilterWidget: true
    };
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

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        // FIXME: DataService.createStream() requires a scope with a
        // projectPromise rather than just a namespace, so we have to pass the
        // context into the log-viewer directive.
        $scope.projectContext = context;
        $scope.logOptions = {};
        DataService.get("builds", $routeParams.build, context).then(
          // success
          function(build) {

            $scope.loaded = true;
            $scope.build = build;
            $scope.logOptions.container = $filter("annotation")(build, "buildPod");
            var buildNumber = $filter("annotation")(build, "buildNumber");
            if (buildNumber) {
              $scope.breadcrumbs[2].title = "#" + buildNumber;
            }

            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("builds", $routeParams.build, context, function(build, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: "This build has been deleted."
                };
              }
              $scope.build = build;
              // TODO: if build.status.phase === 'Error' then we should not
              // fetch the log, BUT ALSO indicate this somehow in UI
              $scope.logCanRun = !(_.includes(['New', 'Pending', 'Error'], build.status.phase));
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

        var hashSize = $filter('hashSize');
        watches.push(DataService.watch("builds", context, function(builds, action, build) {
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

          $scope.canBuild = !hashSize($scope.buildConfigBuildsInProgress[buildConfigName]);
        }));

        $scope.cancelBuild = function() {
          BuildsService.cancelBuild($scope.build, $scope.buildConfigName, context, $scope);
        };

        $scope.cloneBuild = function() {
          var name = _.get($scope, 'build.metadata.name');
          if (name && $scope.canBuild) {
            BuildsService.cloneBuild(name, context, $scope);
          }
        };

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });
      }));
  });
