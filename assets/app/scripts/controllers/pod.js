'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodController', function ($scope, $routeParams, $timeout, $uibModal, DataService, PodsService, ProjectsService, $filter, ImageStreamResolver, MetricsService) {
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

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        // FIXME: DataService.createStream() requires a scope with a
        // projectPromise rather than just a namespace, so we have to pass the
        // context into the log-viewer directive.
        $scope.logContext = context;
        DataService.get("pods", $routeParams.pod, context).then(
          // success
          function(pod) {
            $scope.loaded = true;
            $scope.pod = pod;
            $scope.logOptions.container = $routeParams.container || pod.spec.containers[0].name;
            $scope.logCanRun = !(_.includes(['New', 'Pending', 'Unknown'], pod.status.phase));
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
        $scope.debugTerminal = function(containerName) {
          var debugPod = PodsService.generateDebugPod($scope.pod, containerName);
          if (!debugPod) {
            $scope.alerts['debug-container-error'] = {
              type: "error",
              message: "Could not debug container " + containerName
            }
            return;
          }

          // Create the debug pod.
          DataService.create("pods", null, debugPod, context).then(
            // success
            function(pod) {
              // Watch the pod so we know when it's running to connect.
              // Keep the watch handle in a var outside the watches array so we
              // can unwatch immediately when the terminal is closed.
              debugPodWatch = DataService.watchObject("pods", debugPod.metadata.name, context, function(pod, action) {
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
                    return containerName
                  }
                },
                backdrop: 'static' // don't close modal when clicking backdrop
              });

              // On modal close, delete the pod.
              // TODO: Try to delete pod when user navigates away without closing modal.
              modalInstance.result.then(function() {
                DataService.unwatch(debugPodWatch);
                debugPodWatch = null;
                // Delete the pod when done.
                DataService.delete("pods", pod.metadata.name, context).then(
                  // success
                  function() {
                    $scope.debugPod = null;
                  },
                  // failure
                  function(result) {
                    $scope.alerts['debug-container-error'] = {
                      type: "error",
                      message: "Could not delete pod " + pod.metadata.name,
                      details: "Reason: " + $filter('getErrorDetails')(result)
                    };
                  });
              });
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
          if (debugPodWatch) {
            DataService.unwatch(debugPodWatch);
            debugPodWatch = null;
          }
        });
    }));
  });
