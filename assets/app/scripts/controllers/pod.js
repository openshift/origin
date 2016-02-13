'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodController', function ($scope,
                                         $filter,
                                         $routeParams,
                                         $timeout,
                                         $uibModal,
                                         DataService,
                                         ImageStreamResolver,
                                         MetricsService,
                                         PodsService,
                                         ProjectsService) {
    $scope.projectName = $routeParams.project;
    $scope.pod = null;
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;
    $scope.logOptions = {};
    $scope.terminalTabWasSelected = false;
    $scope.breadcrumbs = [
      {
        title: "Pods",
        link: "project/" + $routeParams.project + "/browse/pods"
      },
      {
        title: $routeParams.pod
      }
    ];

    // Check for a ?tab=<name> query param to allow linking directly to a tab.
    if ($routeParams.tab) {
      $scope.selectedTab = {};
      $scope.selectedTab[$routeParams.tab] = true;
    }

    var watches = [];

    // Check if the metrics service is available so we know when to show the tab.
    MetricsService.isAvailable().then(function(available) {
      $scope.metricsAvailable = available;
    });

    var setLogVars = function(pod) {
      $scope.logOptions.container = $routeParams.container || pod.spec.containers[0].name;
      $scope.logCanRun = !(_.includes(['New', 'Pending', 'Unknown'], pod.status.phase));
    };

    var setContainerVars = function() {
      if(!$scope.pod) {
        return;
      }
      var containerStatus = _.find($scope.pod.status.containerStatuses, { name: $scope.logOptions.container });
      var state = _.get(containerStatus, 'state');
      var statusKey = _.head(_.keys(state));
      var knownKey = _.includes(['running', 'waiting', 'terminated'], statusKey) ? statusKey : '';
      var lastState = _.get(containerStatus, 'lastState');
      var lastStatusKey = _.head(_.keys(lastState));
      var isWaiting =  _.get(containerStatus, 'state.waiting');

      angular.extend($scope, {
        containerStatusKey: knownKey,
        containerStateReason: _.get(state, [statusKey, 'reason'])
      });

      if(isWaiting) {
        angular.extend($scope, {
          lasStatusKey: lastStatusKey,
          containerStartTime:  _.get(lastState, [lastStatusKey, 'startedAt']),
          containerEndTime:  _.get(lastState, [lastStatusKey, 'finishedAt'])
        });
      } else {
        angular.extend($scope, {
          containerStartTime: _.get(state, [statusKey, 'startedAt']),
          containerEndTime: _.get(state, [statusKey, 'finishedAt'])
        });
      }
    };

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        // FIXME: DataService.createStream() requires a scope with a
        // projectPromise rather than just a namespace, so we have to pass the
        // context into the log-viewer directive.
        $scope.projectContext = context;
        DataService.get("pods", $routeParams.pod, context).then(
          // success
          function(pod) {
            $scope.loaded = true;
            $scope.pod = pod;
            setLogVars(pod);
            setContainerVars();
            var pods = {};
            pods[pod.metadata.name] = pod;
            ImageStreamResolver.fetchReferencedImageStreamImages(pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, context);

            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("pods", $routeParams.pod, context, function(pod, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: "This pod has been deleted."
                };
              }
              $scope.pod = pod;
              setLogVars(pod);
              setContainerVars();
            }));
          },
          // failure
          function(e) {
            $scope.loaded = true;
            $scope.alerts["load"] = {
              type: "error",
              message: "The pod details could not be loaded.",
              details: "Reason: " + $filter('getErrorDetails')(e)
            };
          }
        );

        // covers container picker if multiple containers
        // outside of the above watch to avoid repeatedly generating new watches.
        $scope.$watch('logOptions.container', setContainerVars);

        // Sets up subscription for imageStreams
        watches.push(DataService.watch("imagestreams", context, function(imageStreams) {
          $scope.imageStreams = imageStreams.by("metadata.name");
          ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
          ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, context);
          Logger.log("imagestreams (subscribe)", $scope.imageStreams);
        }));

        watches.push(DataService.watch("builds", context, function(builds) {
          $scope.builds = builds.by("metadata.name");
          Logger.log("builds (subscribe)", $scope.builds);
        }));

        var debugPodWatch;

        // Delete the debug pod and any watches if they exist.
        var cleanUpDebugPod = function() {
          var debugPod = $scope.debugPod;

          if (debugPodWatch) {
            DataService.unwatch(debugPodWatch);
            debugPodWatch = null;
          }

          if (debugPod) {
            DataService.delete("pods", debugPod.metadata.name, context, {
              gracePeriodSeconds: 0
            }).then(
              // success
              _.noop,
              // failure
              function(result) {
                $scope.alerts['debug-container-error'] = {
                  type: "error",
                  message: "Could not delete pod " + debugPod.metadata.name,
                  details: "Reason: " + $filter('getErrorDetails')(result)
                };
              });
            $scope.debugPod = null;
          }
        };

        $scope.debugTerminal = function(containerName) {
          var debugPod = PodsService.generateDebugPod($scope.pod, containerName);
          if (!debugPod) {
            $scope.alerts['debug-container-error'] = {
              type: "error",
              message: "Could not debug container " + containerName
            };
            return;
          }

          // Create the debug pod.
          DataService.create("pods", null, debugPod, context).then(
            function(pod) {
              $scope.debugPod = pod;

              // Watch the pod so we know when it's running to connect.
              // Keep the watch handle in a var outside the watches array so we
              // can unwatch immediately when the terminal is closed.
              debugPodWatch = DataService.watchObject("pods",
                                                      debugPod.metadata.name,
                                                      context,
                                                      function(pod) {
                $scope.debugPod = pod;
              });

              // Show the terminal in a modal window.
              var modalInstance = $uibModal.open({
                animation: true,
                templateUrl: 'views/modals/debug-terminal.html',
                controller: 'DebugTerminalModalController',
                scope: $scope,
                resolve: {
                  containerName: function() {
                    return containerName;
                  }
                },
                backdrop: 'static' // don't close modal when clicking backdrop
              });

              // On modal close, delete the pod.
              modalInstance.result.then(cleanUpDebugPod);
            },
            //failure
            function(result) {
              $scope.alerts['debug-container-error'] = {
                type: "error",
                message: "Could not debug container " + containerName,
                details: "Reason: " + $filter('getErrorDetails')(result)
              };
            });
        };

        $scope.containersRunning = function(containerStatuses) {
          var running = 0;
          if (containerStatuses) {
            containerStatuses.forEach(function(v) {
              if (v.state && v.state.running) {
                running++;
              }
            });
          }
          return running;
        };

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
          cleanUpDebugPod();
        });
    }));
  });
