'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:OverviewController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('OverviewController', function ($scope, DataService, $filter, LabelFilter, Logger, ImageStreamResolver, ObjectDescriber) {
    $scope.pods = {};
    $scope.services = {};
    $scope.unfilteredServices = {};
    $scope.deployments = {};
    $scope.deploymentConfigs = {};
    $scope.builds = {};
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream    

    // All pods under a service (no "" service key)
    $scope.podsByService = {};
    // All pods under a deployment (no "" deployment key)
    $scope.podsByDeployment = {};
    // Pods not in a deployment
    // "" service key for pods not under any service
    $scope.monopodsByService = {};
    // All deployments
    // "" service key for deployments not under any service
    // "" deployment config for deployments not created from a deployment config
    $scope.deploymentsByServiceByDeploymentConfig = {};
    // All deployments
    // "" service key for deployments not under any service
    // Only being built to improve efficiency in the podRelationships method, not used by the view
    $scope.deploymentsByService = {};
    // All deployment configs
    // "" service key for deployment configs not under any service
    $scope.deploymentConfigsByService = {};

    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.showSidebarRight = true;
    var watches = [];

    watches.push(DataService.watch("pods", $scope, function(pods) {
      $scope.pods = pods.by("metadata.name");
      podRelationships();
      ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      Logger.log("pods", $scope.pods);
    }));

    watches.push(DataService.watch("services", $scope, function(services) {
      $scope.unfilteredServices = services.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredServices, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.services = LabelFilter.getLabelSelector().select($scope.unfilteredServices);

      // Order is important here since podRelationships expects deploymentsByServiceByDeploymentConfig to be up to date
      deploymentsByService();
      deploymentConfigsByService();
      podRelationships();

      $scope.emptyMessage = "No services to show";
      updateFilterWarning();
      Logger.log("services (list)", $scope.services);
    }));

    // Expects deploymentsByServiceByDeploymentConfig to be up to date
    var podRelationships = function() {
      $scope.monopodsByService = {"": {}};
      $scope.podsByService = {};
      $scope.podsByDeployment = {};

      // Initialize all the label selectors upfront
      var depSelectors = {};
      angular.forEach($scope.deployments, function(deployment, depName){
        depSelectors[depName] = new LabelSelector(deployment.spec.selector);
        $scope.podsByDeployment[depName] = {};
      });
      var svcSelectors = {};
      angular.forEach($scope.unfilteredServices, function(service, svcName){
        svcSelectors[svcName] = new LabelSelector(service.spec.selector);
        $scope.podsByService[svcName] = {};
      });


      angular.forEach($scope.pods, function(pod, name){
        var deployments = [];
        var services = [];
        angular.forEach($scope.deployments, function(deployment, depName){
          var ls = depSelectors[depName];
          if (ls.matches(pod)) {
            deployments.push(depName);
            $scope.podsByDeployment[depName][name] = pod;
          }
        });
        angular.forEach($scope.unfilteredServices, function(service, svcName){
          var ls = svcSelectors[svcName];
          if (ls.matches(pod)) {
            services.push(svcName);
            $scope.podsByService[svcName][name] = pod;

            var isInDepInSvc = false;
            angular.forEach(deployments, function(depName) {
              isInDepInSvc = isInDepInSvc || ($scope.deploymentsByService[svcName] && $scope.deploymentsByService[svcName][depName]);
            });

            if (!isInDepInSvc) {
              $scope.monopodsByService[svcName] = $scope.monopodsByService[svcName] || {};
              $scope.monopodsByService[svcName][name] = pod;
            }
          }
        });
        if (deployments.length == 0 && services.length == 0 && showMonopod(pod)) {
          $scope.monopodsByService[""][name] = pod;
        }
      });

      Logger.log("podsByDeployment", $scope.podsByDeployment);
      Logger.log("podsByService", $scope.podsByService);
      Logger.log("monopodsByService", $scope.monopodsByService);
    };

    // Filter out monopods we know we don't want to see
    var showMonopod = function(pod) {
      // Hide pods in the Succeeded or Terminated phase since these are run once pods
      // that are done
      if (pod.status.phase == 'Succeeded' || pod.status.phase == 'Terminated') {
        // TODO we may want to show pods for X amount of time after they have completed
        return false;
      }
      // Hide our deployer pods since it is obvious the deployment is happening when the new deployment
      // appears.
      if (pod.metadata.annotations && pod.metadata.annotations.deployment) {
        return false;
      }
      // Hide our build pods since we are already showing details for currently running or recently
      // run builds under the appropriate areas
      for (var id in $scope.builds) {
        if ($scope.builds[id].metadata.name == pod.metadata.name) {
          return false;
        }
      }

      return true;
    };

    var deploymentConfigsByService = function() {
      $scope.deploymentConfigsByService = {"": {}};
      angular.forEach($scope.deploymentConfigs, function(deploymentConfig, depName){
        var foundMatch = false;
        // TODO this is using the k8s v1beta1 ReplicationControllerState schema, replicaSelector will change to selector eventually
        var depConfigSelector = new LabelSelector(deploymentConfig.template.controllerTemplate.replicaSelector);
        angular.forEach($scope.unfilteredServices, function(service, name){
          $scope.deploymentConfigsByService[name] = $scope.deploymentConfigsByService[name] || {};

          var serviceSelector = new LabelSelector(service.spec.selector);
          if (serviceSelector.covers(depConfigSelector)) {
            $scope.deploymentConfigsByService[name][depName] = deploymentConfig;
            foundMatch = true;
          }
        });
        if (!foundMatch) {
          $scope.deploymentConfigsByService[""][depName] = deploymentConfig;
        }
      });
    };

    var deploymentsByService = function() {
      var bySvc = $scope.deploymentsByService = {"": {}};
      var bySvcByDepCfg = $scope.deploymentsByServiceByDeploymentConfig = {"": {}};

      angular.forEach($scope.deployments, function(deployment, depName){
        var foundMatch = false;
        var deploymentSelector = new LabelSelector(deployment.spec.selector);
        var depConfigName = "";
        if (deployment.metadata.annotations) {
          depConfigName = deployment.metadata.annotations.deploymentConfig || "";
        }

        angular.forEach($scope.unfilteredServices, function(service, name){
          bySvc[name] = bySvc[name] || {};
          bySvcByDepCfg[name] = bySvcByDepCfg[name] || {};

          var serviceSelector = new LabelSelector(service.spec.selector);
          if (serviceSelector.covers(deploymentSelector)) {
            bySvc[name][depName] = deployment;

            bySvcByDepCfg[name][depConfigName] = bySvcByDepCfg[name][depConfigName] || {};
            bySvcByDepCfg[name][depConfigName][depName] = deployment;
            foundMatch = true;
          }
        });
        if (!foundMatch) {
          bySvc[""][depName] = deployment;

          bySvcByDepCfg[""][depConfigName] = bySvcByDepCfg[""][depConfigName] || {};
          bySvcByDepCfg[""][depConfigName][depName] = deployment;
        }
      });
    };

    function parseEncodedDeploymentConfig(deployment) {
      if (deployment.metadata.annotations && deployment.metadata.annotations.encodedDeploymentConfig) {
        try {
          var depConfig = $.parseJSON(deployment.metadata.annotations.encodedDeploymentConfig);
          deployment.details = depConfig.details;
        }
        catch (e) {
          Logger.error("Failed to parse encoded deployment config", e);
        }
      }
    }

    // Sets up subscription for deployments
    watches.push(DataService.watch("replicationcontrollers", $scope, function(deployments, action, deployment) {
      $scope.deployments = deployments.by("metadata.name");
      if (deployment) {
        if (action !== "DELETED") {
          parseEncodedDeploymentConfig(deployment);
        }
      }
      else {
        angular.forEach($scope.deployments, function(dep) {
          parseEncodedDeploymentConfig(dep);
        });
      }

      // Order is important here since podRelationships expects deploymentsByServiceByDeploymentConfig to be up to date
      deploymentsByService();
      podRelationships();
      Logger.log("deployments (subscribe)", $scope.deployments);
    }));

    // Sets up subscription for imageStreams
    watches.push(DataService.watch("imageStreams", $scope, function(imageStreams) {
      $scope.imageStreams = imageStreams.by("metadata.name");
      ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
      ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      Logger.log("imageStreams (subscribe)", $scope.imageStreams);
    }));

    var associateDeploymentConfigTriggersToBuild = function(deploymentConfig, build) {
      // Make sure we have both a deploymentConfig and a build
      if (!deploymentConfig || !build) {
        return;
      }
      // Make sure we have a build output
      if (!build.parameters.output.to) {
        return;
      }
      for (var i = 0; i < deploymentConfig.triggers.length; i++) {
        var trigger = deploymentConfig.triggers[i];
        if (trigger.type === "ImageChange") {
          var triggerImage = trigger.imageChangeParams.from.name;
          var buildImage = build.parameters.output.to.name;
          if (triggerImage !== buildImage) {
          	continue;
          }

          var triggerNamespace = trigger.imageChangeParams.from.namespace || deploymentConfig.metadata.namespace;
          var buildNamespace = build.parameters.output.to.namespace || build.metadata.namespace;
          if (triggerNamespace !== buildNamespace) {
          	continue;
          }

          var triggerTag = trigger.imageChangeParams.tag;
          var buildTag = build.parameters.output.tag || "latest";
          if (triggerTag !== buildTag) {
          	continue;
          }

          trigger.builds = trigger.builds || {};
          trigger.builds[build.metadata.name] = build;
        }
      }
    };

    // Sets up subscription for deploymentConfigs, associates builds to triggers on deploymentConfigs
    watches.push(DataService.watch("deploymentConfigs", $scope, function(deploymentConfigs, action, deploymentConfig) {
      $scope.deploymentConfigs = deploymentConfigs.by("metadata.name");
      if (!action) {
        angular.forEach($scope.deploymentConfigs, function(depConfig) {
          angular.forEach($scope.builds, function(build) {
            associateDeploymentConfigTriggersToBuild(depConfig, build);
          });
        });
      }
      else if (action !== 'DELETED') {
        angular.forEach($scope.builds, function(build) {
          associateDeploymentConfigTriggersToBuild(deploymentConfig, build);
        });
      }

      deploymentConfigsByService();

      Logger.log("deploymentConfigs (subscribe)", $scope.deploymentConfigs);
    }));

    // Sets up subscription for builds, associates builds to triggers on deploymentConfigs
    watches.push(DataService.watch("builds", $scope, function(builds, action, build) {
      $scope.builds = builds.by("metadata.name");
      if (!action) {
        angular.forEach($scope.builds, function(bld) {
          angular.forEach($scope.deploymentConfigs, function(depConfig) {
            associateDeploymentConfigTriggersToBuild(depConfig, bld);
          });
        });
      }
      else if (action === 'ADDED' || action === 'MODIFIED') {
        angular.forEach($scope.deploymentConfigs, function(depConfig) {
          associateDeploymentConfigTriggersToBuild(depConfig, build);
        });
      }
      else if (action === 'DELETED'){
        // TODO
      }
      Logger.log("builds (subscribe)", $scope.builds);
    }));

    var updateFilterWarning = function() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.services) && !$.isEmptyObject($scope.unfilteredServices)) {
        $scope.alerts["services"] = {
          type: "warning",
          details: "The active filters are hiding all services."
        };
      }
      else {
        delete $scope.alerts["services"];
      }
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.services = labelSelector.select($scope.unfilteredServices);
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
      ObjectDescriber.clearObject();
    });
  });
