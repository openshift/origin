'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DeploymentController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DeploymentController', function ($scope, $routeParams, DataService, ProjectsService, DeploymentsService, ImageStreamResolver, $filter) {
    $scope.projectName = $routeParams.project;
    $scope.deployment = null;
    $scope.deploymentConfig = null;
    $scope.deploymentConfigMissing = false;
    $scope.deployments = {};
    $scope.podTemplates = {};
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;
    $scope.breadcrumbs = [
      {
        title: "Deployments",
        link: "project/" + $routeParams.project + "/browse/deployments"
      }
    ];

    // if this is an RC it won't have deploymentconfig
    if ($routeParams.deploymentconfig){
      $scope.breadcrumbs.push({
        title: $routeParams.deploymentconfig,
        link: "project/" + $routeParams.project + "/browse/deployments/" + $routeParams.deploymentconfig
      });
    }
    $scope.breadcrumbs.push({
      title: $routeParams.deployment || $routeParams.replicationcontroller
    });

    // Check for a ?tab=<name> query param to allow linking directly to a tab.
    if ($routeParams.tab) {
      $scope.selectedTab = {};
      $scope.selectedTab[$routeParams.tab] = true;
    }

    $scope.logOptions = {};

    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        // FIXME: DataService.createStream() requires a scope with a
        // projectPromise rather than just a namespace, so we have to pass the
        // context into the log-viewer directive.
        $scope.logContext = context;

        var watchActiveDeployment = function() {
          // Watch all replication controllers so we know if this is the active deployment to enable scaling.
          watches.push(DataService.watch("replicationcontrollers", context, function(deployments) {
            var activeDeployment,
                deploymentsForConfig = [],
                getAnnotation = $filter("annotation");
            // Filter the list to just those deployments for this config.
            angular.forEach(deployments.by("metadata.name"), function(deployment) {
              var depConfigName = getAnnotation(deployment, 'deploymentConfig') || "";
              if (depConfigName === $scope.deploymentConfigName) {
                deploymentsForConfig.push(deployment);
              }
            });
            activeDeployment = DeploymentsService.getActiveDeployment(deploymentsForConfig);
            $scope.isActive = activeDeployment && activeDeployment.metadata.uid === $scope.deployment.metadata.uid;
          }));
        };

        DataService.get("replicationcontrollers", $routeParams.deployment || $routeParams.replicationcontroller, context).then(
          // success
          function(deployment) {
            $scope.loaded = true;
            $scope.deployment = deployment;
            var deploymentVersion = $filter("annotation")(deployment, "deploymentVersion");
            if (deploymentVersion) {
              $scope.breadcrumbs[2].title = "#" + deploymentVersion;
              $scope.logOptions.version = deploymentVersion;
            }
            $scope.deploymentConfigName = $filter("annotation") (deployment, "deploymentConfig");

            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("replicationcontrollers", $routeParams.deployment || $routeParams.replicationcontroller, context, function(deployment, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: $routeParams.deployment ? "This deployment has been deleted." : "This replication controller has been deleted."
                };
              }
              $scope.deployment = deployment;
            }));

            if ($scope.deploymentConfigName) {
              // Check if we're the active deployment to enable or disable scaling.
              watchActiveDeployment();
            }
          },
          // failure
          function(e) {
            $scope.loaded = true;
            $scope.alerts["load"] = {
              type: "error",
              message: $routeParams.deployment ? "The deployment details could not be loaded." : "The replication controller details could not be loaded.",
              details: "Reason: " + $filter('getErrorDetails')(e)
            };
          }
        );

        if ($routeParams.deploymentconfig) {
          DataService.get("deploymentconfigs", $routeParams.deploymentconfig, context, {
            errorNotification: false
          }).then(
            // success
            function(deploymentConfig) {
              $scope.deploymentConfig = deploymentConfig;
            },
            // failure
            function(e) {
              if (e.status === 404) {
                $scope.deploymentConfigMissing = true;
                return;
              }

              $scope.alerts["load"] = {
                type: "error",
                message: "The deployment configuration details could not be loaded.",
                details: "Reason: " + $filter('getErrorDetails')(e)
              };
            }
          );
        }

        function extractPodTemplates() {
          angular.forEach($scope.deployments, function(deployment, deploymentId){
            $scope.podTemplates[deploymentId] = deployment.spec.template;
          });
        }

        watches.push(DataService.watch("replicationcontrollers", context, function(deployments, action, deployment) {
          $scope.deployments = deployments.by("metadata.name");
          extractPodTemplates();
          ImageStreamResolver.fetchReferencedImageStreamImages($scope.podTemplates, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, context);
          $scope.emptyMessage = "No deployments to show";
          $scope.deploymentsByDeploymentConfig = DeploymentsService.associateDeploymentsToDeploymentConfig($scope.deployments);

          var deploymentConfigName;
          var deploymentName;
          if (deployment) {
            deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
            deploymentName = deployment.metadata.name;
          }
          if (!action) {
            // Loading of the page that will create deploymentConfigDeploymentsInProgress structure, which will associate running deployment to his deploymentConfig.
            $scope.deploymentConfigDeploymentsInProgress = DeploymentsService.associateRunningDeploymentToDeploymentConfig($scope.deploymentsByDeploymentConfig);
          } else if (action === 'ADDED' || (action === 'MODIFIED' && ['New', 'Pending', 'Running'].indexOf($filter('deploymentStatus')(deployment)) > -1)) {
            // When new deployment id instantiated/cloned, or in case of a retry, associate him to his deploymentConfig and add him into deploymentConfigDeploymentsInProgress structure.
            $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] = $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] || {};
            $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName] = deployment;
          } else if (action === 'MODIFIED') {
            // After the deployment ends remove him from the deploymentConfigDeploymentsInProgress structure.
            var status = $filter('deploymentStatus')(deployment);
            if (status === "Complete" || status === "Failed"){
              delete $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName];
            }
          }

          // Extract the causes from the encoded deployment config
          if (deployment) {
            if (action !== "DELETED") {
              deployment.causes = $filter('deploymentCauses')(deployment);
            }
          }
          else {
            angular.forEach($scope.deployments, function(deployment) {
              deployment.causes = $filter('deploymentCauses')(deployment);
            });
          }
        }));

        // Sets up subscription for imageStreams
        watches.push(DataService.watch("imagestreams", context, function(imageStreams) {
          $scope.imageStreams = imageStreams.by("metadata.name");
          ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
          ImageStreamResolver.fetchReferencedImageStreamImages($scope.podTemplates, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, context);
          Logger.log("imagestreams (subscribe)", $scope.imageStreams);
        }));

        watches.push(DataService.watch("builds", context, function(builds) {
          $scope.builds = builds.by("metadata.name");
          Logger.log("builds (subscribe)", $scope.builds);
        }));

        $scope.startLatestDeployment = function(deploymentConfig) {
          DeploymentsService.startLatestDeployment(deploymentConfig, context, $scope);
        };

        $scope.retryFailedDeployment = function(deployment) {
          DeploymentsService.retryFailedDeployment(deployment, context, $scope);
        };

        $scope.rollbackToDeployment = function(deployment, changeScaleSettings, changeStrategy, changeTriggers) {
          DeploymentsService.rollbackToDeployment(deployment, changeScaleSettings, changeStrategy, changeTriggers, context, $scope);
        };

        $scope.cancelRunningDeployment = function(deployment) {
          DeploymentsService.cancelRunningDeployment(deployment, context, $scope);
        };

        $scope.scale = function(replicas) {
          var showScalingError = function(result) {
            $scope.alerts = $scope.alerts || {};
            $scope.alerts["scale"] = {
              type: "error",
              message: "An error occurred scaling the deployment.",
              details: $filter('getErrorDetails')(result)
            };
          };

          if ($scope.deploymentConfig) {
            DeploymentsService.scaleDC($scope.deploymentConfig, replicas).then(_.noop, showScalingError);
          } else {
            DeploymentsService.scaleRC($scope.deployment, replicas).then(_.noop, showScalingError);
          }
        };

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });
    }));
  });
