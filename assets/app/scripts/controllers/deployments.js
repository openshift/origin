'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DeploymentsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DeploymentsController', function ($scope, DataService, $filter, LabelFilter, Logger, ImageStreamResolver) {
    $scope.deployments = {};
    $scope.unfilteredDeployments = {};
    // leave undefined so we know when data is loaded
    $scope.deploymentConfigs = undefined;
    $scope.deploymentsByDeploymentConfig = {};
    $scope.podTemplates = {};
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    function extractPodTemplates() {
      angular.forEach($scope.deployments, function(deployment, deploymentId){
        $scope.podTemplates[deploymentId] = deployment.spec.template;
      });
    }

    watches.push(DataService.watch("replicationcontrollers", $scope, function(deployments, action, deployment) {
      $scope.unfilteredDeployments = deployments.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredDeployments, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.deployments = LabelFilter.getLabelSelector().select($scope.unfilteredDeployments);
      extractPodTemplates();
      ImageStreamResolver.fetchReferencedImageStreamImages($scope.podTemplates, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      $scope.emptyMessage = "No deployments to show";
      associateDeploymentsToDeploymentConfig();
      updateFilterWarning();

      var deploymentConfigName;
      var deploymentName;
      if (deployment) {
        deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
        deploymentName = deployment.metadata.name;
      }
      if (!action) {
        // Loading of the page that will create deploymentConfigDeploymentsInProgress structure, which will associate running deployment to his deploymentConfig.
        $scope.deploymentConfigDeploymentsInProgress = associateRunningDeploymentToDeploymentConfig($scope.deploymentsByDeploymentConfig);
      } else if (action === 'ADDED' || (action === 'MODIFIED' && ['New', 'Pending', 'Running'].indexOf($scope.deploymentStatus(deployment)) > -1)) {
        // When new deployment id instantiated/cloned, or in case of a retry, associate him to his deploymentConfig and add him into deploymentConfigDeploymentsInProgress structure.
        $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] = $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] || {};
        $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName] = deployment;
      } else if (action === 'MODIFIED') {
        // After the deployment ends remove him from the deploymentConfigDeploymentsInProgress structure.
        var deploymentStatus = $scope.deploymentStatus(deployment);
        if (deploymentStatus === "Complete" || deploymentStatus === "Failed"){
          delete $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName];
        }
      }

      Logger.log("deployments (subscribe)", $scope.deployments);
    }));

    watches.push(DataService.watch("deploymentconfigs", $scope, function(deploymentConfigs) {
      $scope.deploymentConfigs = deploymentConfigs.by("metadata.name");
      Logger.log("deploymentconfigs (subscribe)", $scope.deploymentConfigs);
    }));

    // Sets up subscription for imageStreams
    watches.push(DataService.watch("imagestreams", $scope, function(imageStreams) {
      $scope.imageStreams = imageStreams.by("metadata.name");
      ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
      ImageStreamResolver.fetchReferencedImageStreamImages($scope.podTemplates, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      Logger.log("imagestreams (subscribe)", $scope.imageStreams);
    }));

    watches.push(DataService.watch("builds", $scope, function(builds) {
      $scope.builds = builds.by("metadata.name");
      Logger.log("builds (subscribe)", $scope.builds);
    }));

    function associateDeploymentsToDeploymentConfig() {
      $scope.deploymentsByDeploymentConfig = {};
      angular.forEach($scope.deployments, function(deployment, deploymentName) {
        var deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
        if (deploymentConfigName) {
          $scope.deploymentsByDeploymentConfig[deploymentConfigName] = $scope.deploymentsByDeploymentConfig[deploymentConfigName] || {};
          $scope.deploymentsByDeploymentConfig[deploymentConfigName][deploymentName] = deployment;
        }
      });
    }

    function associateRunningDeploymentToDeploymentConfig(deploymentsByDeploymentConfig) {
      var deploymentConfigDeploymentsInProgress = {};
      angular.forEach(deploymentsByDeploymentConfig, function(deploymentConfigDeployments, deploymentConfigName) {
        deploymentConfigDeploymentsInProgress[deploymentConfigName] = {};
        angular.forEach(deploymentConfigDeployments, function(deployment, deploymentName) {
          var deploymentStatus = $scope.deploymentStatus(deployment);
          if (deploymentStatus === "New" || deploymentStatus === "Pending" || deploymentStatus === "Running") {
            deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName] = deployment;
          }
        });
      });
      return deploymentConfigDeploymentsInProgress;
    }

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.deployments) && !$.isEmptyObject($scope.unfilteredDeployments)) {
        $scope.alerts["deployments"] = {
          type: "warning",
          details: "The active filters are hiding all deployments."
        };
      }
      else {
        delete $scope.alerts["deployments"];
      }
    }

    $scope.startLatestDeployment = function(deploymentConfigName) {
      var deploymentConfig = $scope.deploymentConfigs[deploymentConfigName];

      // increase latest version by one so starts new deployment based on latest
      var req = {
        kind: "DeploymentConfig",
        apiVersion: "v1",
        metadata: deploymentConfig.metadata,
        spec: deploymentConfig.spec,
        status: deploymentConfig.status
      };
      if (!req.status.latestVersion) {
        req.status.latestVersion = 0;
      }
      req.status.latestVersion++;

      // update the deployment config
      DataService.update("deploymentconfigs", deploymentConfigName, req, $scope).then(
        function() {
            $scope.alerts = [
            {
              type: "success",
              message: "Deployment #" + req.status.latestVersion + " of " + deploymentConfigName + " has started.",
            }
          ];
        },
        function(result) {
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred while starting the deployment.",
              details: $filter('getErrorDetails')(result)
            }
          ];
        }
      );
    };

    $scope.retryFailedDeployment = function(deploymentConfigName, deploymentName) {
      var deployment = $scope.deploymentsByDeploymentConfig[deploymentConfigName][deploymentName];
      var req = deployment;

      // TODO: we need a "retry" api endpoint so we don't have to do this manually

      // delete the deployer pod as well as the deployment hooks pods, if any
      DataService.list("pods", $scope, function(list) {
        var pods = list.by("metadata.name");
        var deleteDeployerPod = function(pod) {
          var deployerPodForAnnotation = $filter('annotationName')('deployerPodFor');
          if (pod.metadata.labels[deployerPodForAnnotation] === deploymentName) {
            DataService.delete("pods", pod.metadata.name, $scope).then(
              function() {
                Logger.info("Deployer pod " + pod.metadata.name + " deleted");
              },
              function(result) {
                $scope.alerts = [
                  {
                    type: "error",
                    message: "An error occurred while deleting the deployer pod.",
                    details: $filter('getErrorDetails')(result)
                  }
                ];
              }
            );
          }
        };
        angular.forEach(pods, deleteDeployerPod);
      });

      // set deployment to "New" and remove statuses so we can retry
      var deploymentStatusAnnotation = $filter('annotationName')('deploymentStatus');
      var deploymentStatusReasonAnnotation = $filter('annotationName')('deploymentStatusReason');
      var deploymentCancelledAnnotation = $filter('annotationName')('deploymentCancelled');
      req.metadata.annotations[deploymentStatusAnnotation] = "New";
      delete req.metadata.annotations[deploymentStatusReasonAnnotation];
      delete req.metadata.annotations[deploymentCancelledAnnotation];

      // update the deployment
      DataService.update("replicationcontrollers", deploymentName, req, $scope).then(
        function() {
            $scope.alerts = [
            {
              type: "success",
              message: "Retrying deployment " + deploymentName + " of " + deploymentConfigName + ".",
            }
          ];
        },
        function(result) {
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred while retrying the deployment.",
              details: $filter('getErrorDetails')(result)
            }
          ];
        }
      );
    };

    $scope.rollbackToDeployment = function(deploymentConfigName, deploymentName, changeScaleSettings, changeStrategy, changeTriggers) {
      // put together a new rollback request
      var req = {
        kind: "DeploymentConfigRollback",
        apiVersion: "v1",
        spec: {
          from: {
            name: deploymentName
          },
          includeTemplate: true,
          includeReplicationMeta: changeScaleSettings,
          includeStrategy: changeStrategy,
          includeTriggers: changeTriggers
        }
      };

      // TODO: we need a "rollback" api endpoint so we don't have to do this manually

      // create the deployment config rollback 
      DataService.create("deploymentconfigrollbacks", null, req, $scope).then(
        function(newDeploymentConfig) {
          // update the deployment config based on the one returned by the rollback
          DataService.update("deploymentconfigs", deploymentConfigName, newDeploymentConfig, $scope).then(
            function(rolledBackDeploymentConfig) {
                $scope.alerts = [
                {
                  type: "success",
                  message: "Deployment #" + rolledBackDeploymentConfig.status.latestVersion + " is rolling back " + deploymentConfigName + " to " + deploymentName + ".",
                }
              ];
            },
            function(result) {
              $scope.alerts = [
                {
                  type: "error",
                  message: "An error occurred while rolling back the deployment.",
                  details: $filter('getErrorDetails')(result)
                }
              ];
            }
          );
        },
        function(result) {
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred while rolling back the deployment.",
              details: $filter('getErrorDetails')(result)
            }
          ];
        }
      );
    };

    $scope.cancelRunningDeployment = function(deploymentConfigName, deploymentName) {
      var deployment = $scope.deploymentsByDeploymentConfig[deploymentConfigName][deploymentName];
      var req = deployment;

      // TODO: we need a "cancel" api endpoint so we don't have to do this manually

      // set the cancellation annotations
      var deploymentCancelledAnnotation = $filter('annotationName')('deploymentCancelled');
      var deploymentStatusReasonAnnotation = $filter('annotationName')('deploymentStatusReason');
      req.metadata.annotations[deploymentCancelledAnnotation] = "true";
      req.metadata.annotations[deploymentStatusReasonAnnotation] = "The deployment was cancelled by the user";

      // update the deployment with cancellation annotations
      DataService.update("replicationcontrollers", deploymentName, req, $scope).then(
        function() {
            $scope.alerts = [
            {
              type: "success",
              message: "Cancelling deployment " + deploymentName + " of " + deploymentConfigName + ".",
            }
          ];
        },
        function(result) {
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred while cancelling the deployment.",
              details: $filter('getErrorDetails')(result)
            }
          ];
        }
      );
    };

    $scope.deploymentIsLatest = function(deploymentConfig, deployment) {
      var deploymentVersion = parseInt($filter('annotation')(deployment, 'deploymentVersion'));
      var deploymentConfigVersion = deploymentConfig.status.latestVersion;
      return deploymentVersion === deploymentConfigVersion;
    };

    $scope.deploymentStatus = function(deployment) {
      return $filter('annotation')(deployment, 'deploymentStatus');
    };

    $scope.deploymentIsInProgress = function(deployment) {
      return ['New', 'Pending', 'Running'].indexOf($scope.deploymentStatus(deployment)) > -1;
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.deployments = labelSelector.select($scope.unfilteredDeployments);
        associateDeploymentsToDeploymentConfig();
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
