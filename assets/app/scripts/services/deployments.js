'use strict';

angular.module("openshiftConsole")
  .factory("DeploymentsService", function(DataService, $filter, LabelFilter){
    function DeploymentsService() {}

    DeploymentsService.prototype.startLatestDeployment = function(deploymentConfig, context, $scope) {
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
      DataService.update("deploymentconfigs", deploymentConfig.metadata.name, req, context).then(
        function() {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["deploy"] =
            {
              type: "success",
              message: "Deployment #" + req.status.latestVersion + " of " + deploymentConfig.metadata.name + " has started.",
            };
        },
        function(result) {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["deploy"] =
            {
              type: "error",
              message: "An error occurred while starting the deployment.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    DeploymentsService.prototype.retryFailedDeployment = function(deployment, context, $scope) {
      var req = angular.copy(deployment);
      var deploymentName = deployment.metadata.name;
      var deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
      // TODO: we need a "retry" api endpoint so we don't have to do this manually

      // delete the deployer pod as well as the deployment hooks pods, if any
      DataService.list("pods", context, function(list) {
        var pods = list.by("metadata.name");
        var deleteDeployerPod = function(pod) {
          var deployerPodForAnnotation = $filter('annotationName')('deployerPodFor');
          if (pod.metadata.labels[deployerPodForAnnotation] === deploymentName) {
            DataService.delete("pods", pod.metadata.name, $scope).then(
              function() {
                Logger.info("Deployer pod " + pod.metadata.name + " deleted");
              },
              function(result) {
                $scope.alerts = $scope.alerts || {};
                $scope.alerts["retrydeployer"] =
                  {
                    type: "error",
                    message: "An error occurred while deleting the deployer pod.",
                    details: $filter('getErrorDetails')(result)
                  };
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
      DataService.update("replicationcontrollers", deploymentName, req, context).then(
        function() {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["retry"] =
            {
              type: "success",
              message: "Retrying deployment " + deploymentName + " of " + deploymentConfigName + ".",
            };
        },
        function(result) {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["retry"] =
            {
              type: "error",
              message: "An error occurred while retrying the deployment.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    DeploymentsService.prototype.rollbackToDeployment = function(deployment, changeScaleSettings, changeStrategy, changeTriggers, context, $scope) {
      var deploymentName = deployment.metadata.name;
      var deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
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
      DataService.create("deploymentconfigrollbacks", null, req, context).then(
        function(newDeploymentConfig) {
          // update the deployment config based on the one returned by the rollback
          DataService.update("deploymentconfigs", deploymentConfigName, newDeploymentConfig, context).then(
            function(rolledBackDeploymentConfig) {
              $scope.alerts = $scope.alerts || {};
              $scope.alerts["rollback"] =
                {
                  type: "success",
                  message: "Deployment #" + rolledBackDeploymentConfig.status.latestVersion + " is rolling back " + deploymentConfigName + " to " + deploymentName + ".",
                };
            },
            function(result) {
              $scope.alerts = $scope.alerts || {};
              $scope.alerts["rollback"] =
                {
                  type: "error",
                  message: "An error occurred while rolling back the deployment.",
                  details: $filter('getErrorDetails')(result)
                };
            }
          );
        },
        function(result) {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["rollback"] =
            {
              type: "error",
              message: "An error occurred while rolling back the deployment.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    DeploymentsService.prototype.cancelRunningDeployment = function(deployment, context, $scope) {
      var deploymentName = deployment.metadata.name;
      var deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
      var req = angular.copy(deployment);

      // TODO: we need a "cancel" api endpoint so we don't have to do this manually

      // set the cancellation annotations
      var deploymentCancelledAnnotation = $filter('annotationName')('deploymentCancelled');
      var deploymentStatusReasonAnnotation = $filter('annotationName')('deploymentStatusReason');
      req.metadata.annotations[deploymentCancelledAnnotation] = "true";
      req.metadata.annotations[deploymentStatusReasonAnnotation] = "The deployment was cancelled by the user";

      // update the deployment with cancellation annotations
      DataService.update("replicationcontrollers", deploymentName, req, context).then(
        function() {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["cancel"] =
            {
              type: "success",
              message: "Cancelling deployment " + deploymentName + " of " + deploymentConfigName + ".",
            };
        },
        function(result) {
          $scope.alerts = $scope.alerts || {};
          $scope.alerts["cancel"] =
            {
              type: "error",
              message: "An error occurred while cancelling the deployment.",
              details: $filter('getErrorDetails')(result)
            };
        }
      );
    };

    // deploymentConfigs is optional
    // filter will run the current label filter against any deployments whose DC is deleted, or any RCs
    DeploymentsService.prototype.associateDeploymentsToDeploymentConfig = function(deployments, deploymentConfigs, filter) {
      var deploymentsByDeploymentConfig = {};
      var labelSelector = LabelFilter.getLabelSelector();
      angular.forEach(deployments, function(deployment, deploymentName) {
        var deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
        if (!filter || deploymentConfigs && deploymentConfigs[deploymentConfigName] || labelSelector.matches(deployment)) {
          deploymentConfigName = deploymentConfigName || '';
          deploymentsByDeploymentConfig[deploymentConfigName] = deploymentsByDeploymentConfig[deploymentConfigName] || {};
          deploymentsByDeploymentConfig[deploymentConfigName][deploymentName] = deployment;
        }
      });
      // Make sure there is an empty map for every dc we know about even if there is no deployment currently
      angular.forEach(deploymentConfigs, function(deploymentConfig, deploymentConfigName) {
        deploymentsByDeploymentConfig[deploymentConfigName] = deploymentsByDeploymentConfig[deploymentConfigName] || {};
      });
      return deploymentsByDeploymentConfig;
    };

    DeploymentsService.prototype.deploymentBelongsToConfig = function(deployment, deploymentConfigName) {
      if (!deployment || !deploymentConfigName) {
        return false;
      }
      return deploymentConfigName === $filter('annotation')(deployment, 'deploymentConfig');
    };

    DeploymentsService.prototype.associateRunningDeploymentToDeploymentConfig = function(deploymentsByDeploymentConfig) {
      var deploymentConfigDeploymentsInProgress = {};
      angular.forEach(deploymentsByDeploymentConfig, function(deploymentConfigDeployments, deploymentConfigName) {
        deploymentConfigDeploymentsInProgress[deploymentConfigName] = {};
        angular.forEach(deploymentConfigDeployments, function(deployment, deploymentName) {
          var status = $filter('deploymentStatus')(deployment);
          if (status === "New" || status === "Pending" || status === "Running") {
            deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName] = deployment;
          }
        });
      });
      return deploymentConfigDeploymentsInProgress;
    };

    // Gets the latest in progress or complete deployment among deployments.
    // Deployments are assumed to be from the same deployment config.
    DeploymentsService.prototype.getActiveDeployment = function(deployments) {
      var orderByDate = $filter('orderObjectsByDate');
      var isInProgress = $filter('deploymentIsInProgress');
      var annotation = $filter('annotation');

      // Sort to look at most recent deployments first.
      var i, deployment, orderedDeployments = orderByDate(deployments, true);
      for (i = 0; i < orderedDeployments.length; i++) {
        deployment = orderedDeployments[i];
        if (isInProgress(deployment) || annotation(deployment, 'deploymentStatus') === 'Complete') {
          return deployment;
        }
      }

      return null;
    };

    DeploymentsService.prototype.scaleDC = function(dc, replicas) {
      // TODO: Use the scale subresource when the web console supports API groups.
      /*
      var scale = {
        kind: "Scale",
        metadata: {
          name: dc.metadata.name,
          namespace: dc.metadata.namespace,
          creationTimestamp: dc.metadata.creationTimestamp
        },
        spec: {
          replicas: replicas
        }
      };
      return DataService.update("deploymentconfigs/scale", dc.metadata.name, scale, {
        namespace: dc.metadata.namespace
      });
     */

      var req = angular.copy(dc);
      req.spec.replicas = replicas;
      return DataService.update("deploymentconfigs", dc.metadata.name, req, {
        namespace: dc.metadata.namespace
      });
    };

    DeploymentsService.prototype.scaleRC = function(rc, replicas) {
      var req = angular.copy(rc);
      req.spec.replicas = replicas;
      return DataService.update("replicationcontrollers", rc.metadata.name, req, {
        namespace: rc.metadata.namespace
      });
    };

    return new DeploymentsService();
  });
