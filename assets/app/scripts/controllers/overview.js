'use strict';
/* jshint sub: true */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:OverviewController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('OverviewController',
              function ($scope,
                        DataService,
                        annotationFilter,
                        hashSizeFilter,
                        imageObjectRefFilter,
                        deploymentCausesFilter,
                        LabelFilter,
                        Logger,
                        ImageStreamResolver) {
    $scope.pods = {};
    $scope.services = {};
    $scope.routesByService = {};
    // The "best" route to display on the overview page for each service
    // (one with a custom host if present)
    $scope.displayRouteByService = {};
    $scope.unfilteredServices = {};
    $scope.deployments = {};
    // Initialize to undefined so we know when deployment configs are actually loaded.
    $scope.deploymentConfigs = undefined;
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
    $scope.renderOptions.showSidebarRight = false;

    var watches = [];

    watches.push(DataService.watch("pods", $scope, function(pods) {
      $scope.pods = pods.by("metadata.name");
      podRelationships();
      // Must be called after podRelationships()
      updateShowGetStarted();
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

      // Must be called after deploymentConfigsByService() and podRelationships()
      updateShowGetStarted();

      $scope.emptyMessage = "No services to show";
      updateFilterWarning();
      Logger.log("services (list)", $scope.services);
    }));

    watches.push(DataService.watch("routes", $scope, function(routes) {
      var routeMap = $scope.routesByService = {};
      var displayRouteMap = $scope.displayRouteByService = {};
      angular.forEach(routes.by("metadata.name"), function(route, routeName){
        if (route.spec.to.kind !== "Service") {
          return;
        }

        var serviceName = route.spec.to.name;
        routeMap[serviceName] = routeMap[serviceName] || {};
        routeMap[serviceName][routeName] = route;

        // Find the best route to display for a service. Prefer the first custom host we find.
        if (!displayRouteMap[serviceName] ||
            (!isGeneratedHost(route) && isGeneratedHost(displayRouteMap[serviceName]))) {
          displayRouteMap[serviceName] = route;
        }
      });

      Logger.log("routes (subscribe)", $scope.routesByService);
    }));

    // Expects deploymentsByServiceByDeploymentConfig to be up to date
    function podRelationships() {
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
        if (deployments.length === 0 && services.length === 0 && showMonopod(pod)) {
          $scope.monopodsByService[""][name] = pod;
        }
      });

      Logger.log("podsByDeployment", $scope.podsByDeployment);
      Logger.log("podsByService", $scope.podsByService);
      Logger.log("monopodsByService", $scope.monopodsByService);
    }

    // Filter out monopods we know we don't want to see
    function showMonopod(pod) {
      // Hide pods in the Succeeded, Terminated, and Failed phases since these
      // are run once pods that are done.
      if (pod.status.phase === 'Succeeded' ||
          pod.status.phase === 'Terminated' ||
          pod.status.phase === 'Failed') {
        // TODO we may want to show pods for X amount of time after they have completed
        return false;
      }

      // Hide our deployer pods since it is obvious the deployment is
      // happening when the new deployment appears.
      if (annotationFilter(pod, "openshift.io/deployment.name")) {
        return false;
      }

      // Hide our build pods since we are already showing details for
      // currently running or recently run builds under the appropriate
      // areas.
      if (annotationFilter(pod, "openshift.io/build.name")) {
        return false;
      }

      return true;
    }

    function deploymentConfigsByService() {
      $scope.deploymentConfigsByService = {"": {}};
      angular.forEach($scope.deploymentConfigs, function(deploymentConfig, depName){
        var foundMatch = false;
        var depConfigSelector = new LabelSelector(deploymentConfig.spec.selector);
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
    }

    function deploymentsByService() {
      var bySvc = $scope.deploymentsByService = {"": {}};
      var bySvcByDepCfg = $scope.deploymentsByServiceByDeploymentConfig = {"": {}};

      angular.forEach($scope.deployments, function(deployment, depName){
        var foundMatch = false;
        var deploymentSelector = new LabelSelector(deployment.spec.selector);
        var depConfigName = annotationFilter(deployment, 'deploymentConfig') || "";

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
    }

    // Sets up subscription for deployments
    watches.push(DataService.watch("replicationcontrollers", $scope, function(deployments, action, deployment) {
      $scope.deployments = deployments.by("metadata.name");
      if (deployment) {
        if (action !== "DELETED") {
          deployment.causes = deploymentCausesFilter(deployment);
        }
      }
      else {
        angular.forEach($scope.deployments, function(deployment) {
          deployment.causes = deploymentCausesFilter(deployment);
        });
      }

      // Order is important here since podRelationships expects deploymentsByServiceByDeploymentConfig to be up to date
      deploymentsByService();
      podRelationships();

      // Must be called after podRelationships()
      updateShowGetStarted();

      Logger.log("deployments (subscribe)", $scope.deployments);
    }));

    // Sets up subscription for imageStreams
    watches.push(DataService.watch("imagestreams", $scope, function(imageStreams) {
      $scope.imageStreams = imageStreams.by("metadata.name");
      ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
      ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      Logger.log("imagestreams (subscribe)", $scope.imageStreams);
    }));

    function associateDeploymentConfigTriggersToBuild(deploymentConfig, build) {
      // Make sure we have both a deploymentConfig and a build
      if (!deploymentConfig || !build) {
        return;
      }
      // Make sure the deployment config has triggers.
      if (!deploymentConfig.spec.triggers) {
        return;
      }
      // Make sure we have a build output
      if (!build.spec.output.to) {
        return;
      }
      for (var i = 0; i < deploymentConfig.spec.triggers.length; i++) {
        var trigger = deploymentConfig.spec.triggers[i];
        if (trigger.type === "ImageChange") {
          var buildOutputImage = imageObjectRefFilter(build.spec.output.to, build.metadata.namespace);
          var deploymentTriggerImage = imageObjectRefFilter(trigger.imageChangeParams.from, deploymentConfig.metadata.namespace);
          if (buildOutputImage !== deploymentTriggerImage) {
              continue;
          }

          trigger.builds = trigger.builds || {};
          trigger.builds[build.metadata.name] = build;
        }
      }
    }

    // Sets up subscription for deploymentConfigs, associates builds to triggers on deploymentConfigs
    watches.push(DataService.watch("deploymentconfigs", $scope, function(deploymentConfigs, action, deploymentConfig) {
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
      // Must be called after deploymentConfigsByService()
      updateShowGetStarted();

      Logger.log("deploymentconfigs (subscribe)", $scope.deploymentConfigs);
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

    // Show the "Get Started" message if the project is empty.
    function updateShowGetStarted() {
      var projectEmpty =
        hashSizeFilter($scope.unfilteredServices) === 0 &&
        hashSizeFilter($scope.pods) === 0 &&
        hashSizeFilter($scope.deployments) === 0 &&
        hashSizeFilter($scope.deploymentConfigs) === 0;

      $scope.renderOptions.showSidebarRight = !projectEmpty;
      $scope.renderOptions.showGetStarted = projectEmpty;
    }

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.services) && !$.isEmptyObject($scope.unfilteredServices)) {
        $scope.alerts["services"] = {
          type: "warning",
          details: "The active filters are hiding all services."
        };
      }
      else {
        delete $scope.alerts["services"];
      }
    }

    function isGeneratedHost(route) {
      return annotationFilter(route, "openshift.io/host.generated") === "true";
    }

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.services = labelSelector.select($scope.unfilteredServices);
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
