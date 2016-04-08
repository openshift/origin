'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildConfigController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildConfigController', function ($scope, $routeParams, DataService, ProjectsService, BuildsService, $filter, LabelFilter, AlertMessageService) {
    $scope.projectName = $routeParams.project;
    $scope.buildConfigName = $routeParams.buildconfig;
    $scope.buildConfig = null;
    $scope.labelSuggestions = {};
    $scope.alerts = {};
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

    AlertMessageService.getAlerts().forEach(function(alert) {
      $scope.alerts[alert.name] = alert.data;
    });
    AlertMessageService.clearAlerts();

    $scope.aceLoaded = function(editor) {
      var session = editor.getSession();
      session.setOption('tabSize', 2);
      session.setOption('useSoftTabs', true);
      editor.$blockScrolling = Infinity;
    };

    var orderByDate = $filter('orderObjectsByDate');

    var updateCanBuild = function() {
      if (!$scope.buildConfig || !$scope.buildConfigBuildsInProgress) {
        $scope.canBuild = false;
      } else {
        $scope.canBuild = BuildsService.canBuild($scope.buildConfig, $scope.buildConfigBuildsInProgress);
      }
    };

    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;

        DataService.get("buildconfigs", $routeParams.buildconfig, context).then(
          // success
          function(buildConfig) {
            $scope.loaded = true;
            $scope.buildConfig = buildConfig;
            $scope.paused = BuildsService.isPaused($scope.buildConfig);
            updateCanBuild();

            if ($scope.buildConfig.spec.source.images) {
              $scope.imageSources = $scope.buildConfig.spec.source.images;
              $scope.imageSourcesPaths = [];
              $scope.imageSources.forEach(function(imageSource) {
                $scope.imageSourcesPaths.push($filter('destinationSourcePair')(imageSource.paths));
              });
            }

            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("buildconfigs", $routeParams.buildconfig, context, function(buildConfig, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: "This build configuration has been deleted."
                };
              }
              $scope.buildConfig = buildConfig;
              $scope.paused = BuildsService.isPaused($scope.buildConfig);
              updateCanBuild();
            }));


            watches.push(DataService.watch("builds", context, function(builds, action, build) {
              $scope.emptyMessage = "No builds to show";
              // TODO we should send the ?labelSelector=buildconfig=<name> on the API request
              // to only load the buildconfig's builds, but this requires some DataService changes
              if (!action) {
                $scope.unfilteredBuilds = builds.by("metadata.name");

                // Loading of the page that will create buildConfigBuildsInProgress structure, which will associate running build to his buildConfig.
                $scope.buildConfigBuildsInProgress = BuildsService.associateRunningBuildToBuildConfig($scope.unfilteredBuilds);
              } else if (build.metadata.labels && build.metadata.labels.buildconfig === $routeParams.buildconfig) {
                var buildName = build.metadata.name;
                var buildConfigName = $routeParams.buildconfig;
                switch (action) {
                  case 'ADDED':
                  case 'MODIFIED':
                    $scope.unfilteredBuilds[buildName] = build;
                    // After the build ends remove him from the buildConfigBuildsInProgress structure.
                    if ($filter('isIncompleteBuild')(build)){
                      $scope.buildConfigBuildsInProgress[buildConfigName] = $scope.buildConfigBuildsInProgress[buildConfigName] || {};
                      $scope.buildConfigBuildsInProgress[buildConfigName][buildName] = build;
                    } else if ($scope.buildConfigBuildsInProgress[buildConfigName]) {
                      delete $scope.buildConfigBuildsInProgress[buildConfigName][buildName];
                    }
                    break;
                  case 'DELETED':
                    delete $scope.unfilteredBuilds[buildName];
                    if ($scope.buildConfigBuildsInProgress[buildConfigName]){
                      delete $scope.buildConfigBuildsInProgress[buildConfigName][buildName];
                    }
                    break;
                }
              }

              $scope.builds = LabelFilter.getLabelSelector().select($scope.unfilteredBuilds);
              updateCanBuild();
              updateFilterWarning();
              LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredBuilds, $scope.labelSuggestions);
              LabelFilter.setLabelSuggestions($scope.labelSuggestions);

              // Sort now to avoid sorting on every digest loop.
              $scope.orderedBuilds = orderByDate($scope.builds, true);
              $scope.latestBuild = $scope.orderedBuilds.length ? $scope.orderedBuilds[0] : null;
            },
            // params object for filtering
            {
              // http is passed to underlying $http calls
              http: {
                params: {
                  labelSelector: 'buildconfig='+$scope.buildConfigName
                }
              }
            }));


          },
          // failure
          function(e) {
            $scope.loaded = true;
            $scope.alerts["load"] = {
              type: "error",
              message: e.status === 404 ? "This build configuration can not be found, it may have been deleted." : "The build configuration details could not be loaded.",
              details: e.status === 404 ? "Any remaining build history for this build will be shown." : "Reason: " + $filter('getErrorDetails')(e)
            };
          }
        );




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

        LabelFilter.onActiveFiltersChanged(function(labelSelector) {
          // trigger a digest loop
          $scope.$apply(function() {
            $scope.builds = labelSelector.select($scope.unfilteredBuilds);
            $scope.orderedBuilds = orderByDate($scope.builds, true);
            $scope.latestBuild = $scope.orderedBuilds.length ? $scope.orderedBuilds[0] : null;
            updateFilterWarning();
          });
        });

        $scope.startBuild = function() {
          if ($scope.canBuild) {
            BuildsService.startBuild($scope.buildConfig.metadata.name, context, $scope);
          }
        };

        $scope.cancelBuild = function(build, buildConfigName) {
          BuildsService.cancelBuild(build, buildConfigName, context, $scope);
        };

        $scope.cloneBuild = function(buildName) {
          BuildsService.cloneBuild(buildName, context, $scope);
        };

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });

    }));
  });
