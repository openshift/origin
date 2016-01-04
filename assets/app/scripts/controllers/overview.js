'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:OverviewController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('OverviewController',
              function ($routeParams,
                        $scope,
                        DataService,
                        DeploymentsService,
                        ProjectsService,
                        annotationFilter,
                        hashSizeFilter,
                        imageObjectRefFilter,
                        deploymentCausesFilter,
                        labelFilter, // for getting k8s resource labels
                        LabelFilter, // for the label-selector widget in the navbar
                        Logger,
                        ImageStreamResolver,
                        ObjectDescriber,
                        $parse,
                        $filter,
                        $interval) {
    $scope.projectName = $routeParams.project;
    $scope.pods = {};
    $scope.services = {};
    $scope.routes = {};
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

    $scope.recentBuildsByOutputImage = {};

    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.showSidebarRight = false;
    $scope.overviewMode = 'tiles';

    // Make sure only one deployment per deployment config is scalable on the overview page.
    // This is the most recent deployment in progress or complete.
    var scalableDeploymentByConfig = {};

    /*
     * HACK: The use of <base href="/"> that is encouraged by angular is
     * a cop-out. It breaks a number of real world use cases, including
     * local xlink:href. Use location.href to get around it, even though
     * these SVG <defs> are local in the template.
     */
    $scope.topologyKinds = {
      DeploymentConfig: location.href + "#vertex-DeploymentConfig",
      Pod: location.href + "#vertex-Pod",
      ReplicationController: location.href + "#vertex-ReplicationController",
      Route: location.href + "#vertex-Route",
      Service: location.href + "#vertex-Service"
    };

    $scope.topologySelection = null;

    /* Filled in by updateTopology */
    $scope.topologyItems = { };
    $scope.topologyRelations = [ ];

    var intervals = [];
    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;

        watches.push(DataService.watch("pods", context, function(pods) {
          $scope.pods = pods.by("metadata.name");
          podRelationships();
          // Must be called after podRelationships()
          updateShowGetStarted();
          ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, context);
          updateTopologyLater();
          Logger.log("pods", $scope.pods);
        }));

        watches.push(DataService.watch("services", context, function(services) {
          $scope.unfilteredServices = services.by("metadata.name");

          LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredServices, $scope.labelSuggestions);
          LabelFilter.setLabelSuggestions($scope.labelSuggestions);
          $scope.services = LabelFilter.getLabelSelector().select($scope.unfilteredServices);

          // Order is important here since podRelationships expects deploymentsByServiceByDeploymentConfig to be up to date
          deploymentRelationships();
          deploymentConfigsByService();
          podRelationships();

          // Must be called after deploymentConfigsByService() and podRelationships()
          updateShowGetStarted();

          $scope.emptyMessage = "No services to show";
          updateFilterWarning();
          updateTopologyLater();
          Logger.log("services (list)", $scope.services);
        }));

        watches.push(DataService.watch("routes", context, function(routes) {
          $scope.routes = routes.by("metadata.name");
          var routeMap = $scope.routesByService = {};
          var displayRouteMap = $scope.displayRouteByService = {};
          angular.forEach($scope.routes, function(route, routeName){
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

          updateTopologyLater();
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

          updateTopologyLater();
        }

        $scope.isScalable = function(deployment, deploymentConfigId) {
          // Allow scaling of RCs with no deployment config.
          if (!deploymentConfigId) {
            return true;
          }

          // Wait for deployment configs to load before allowing scaling of
          // a deployment with a deployment config.
          if (!$scope.deploymentConfigs) {
            return false;
          }

          // Allow scaling of deployments whose deployment config has been deleted.
          if (!$scope.deploymentConfigs[deploymentConfigId]) {
            return true;
          }

          // Otherwise, check the map to find the most recent deployment that's scalable.
          return scalableDeploymentByConfig[deploymentConfigId] === deployment;
        };

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
          if (labelFilter(pod, "openshift.io/deployer-pod-for.name")) {
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
            var getLabels = $parse('spec.template.metadata.labels');
            var depConfigSelector = new LabelSelector(getLabels(deploymentConfig) || {});
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

        // Only the most recent in progress or complete deployment for a given
        // deployment config is scalable in the overview.
        function updateScalableDeployments(deploymentsByDC) {
          scalableDeploymentByConfig = {};
          angular.forEach(deploymentsByDC, function(deployments, dcName) {
            scalableDeploymentByConfig[dcName] = DeploymentsService.getActiveDeployment(deployments);
          });
        }

        function deploymentRelationships() {
          var bySvc = $scope.deploymentsByService = {"": {}};
          var bySvcByDepCfg = $scope.deploymentsByServiceByDeploymentConfig = {"": {}};

          // Also keep a map of deployments by deployment config to determine which are scalable.
          var byDepConfig = {};

          angular.forEach($scope.deployments, function(deployment, depName){
            var foundMatch = false;
            var getLabels = $parse('spec.template.metadata.labels');
            var deploymentSelector = new LabelSelector(getLabels(deployment) || {});
            var depConfigName = annotationFilter(deployment, 'deploymentConfig') || "";

            if (depConfigName) {
              byDepConfig[depConfigName] = byDepConfig[depConfigName] || [];
              byDepConfig[depConfigName].push(deployment);
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

            updateScalableDeployments(byDepConfig);
          });
        }

        // Sets up subscription for deployments
        watches.push(DataService.watch("replicationcontrollers", context, function(deployments, action, deployment) {
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
          deploymentRelationships();
          podRelationships();

          // Must be called after podRelationships()
          updateShowGetStarted();
          updateTopologyLater();

          Logger.log("deployments (subscribe)", $scope.deployments);
        }));

        // Sets up subscription for imageStreams
        watches.push(DataService.watch("imagestreams", context, function(imageStreams) {
          $scope.imageStreams = imageStreams.by("metadata.name");
          ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
          ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, context);
          updateTopologyLater();
          Logger.log("imagestreams (subscribe)", $scope.imageStreams);
        }));

        // Sets up subscription for deploymentConfigs, associates builds to triggers on deploymentConfigs
        watches.push(DataService.watch("deploymentconfigs", context, function(deploymentConfigs) {
          $scope.deploymentConfigs = deploymentConfigs.by("metadata.name");

          deploymentConfigsByService();
          // Must be called after deploymentConfigsByService()
          updateShowGetStarted();
          updateTopologyLater();

          Logger.log("deploymentconfigs (subscribe)", $scope.deploymentConfigs);
        }));

        function updateRecentBuildsByOutputImage() {
          $scope.recentBuildsByOutputImage = {};
          angular.forEach($scope.builds, function(build) {
            // pre-filter the list to save us some time on each digest loop later
            if ($filter('isRecentBuild')(build) || $filter('isOscActiveObject')(build)) {
              var buildOutputImage = imageObjectRefFilter(build.spec.output.to, build.metadata.namespace);
              $scope.recentBuildsByOutputImage[buildOutputImage] = $scope.recentBuildsByOutputImage[buildOutputImage] || [];
              $scope.recentBuildsByOutputImage[buildOutputImage].push(build);
            }
          });
        }

        // Sets up subscription for builds, associates builds to triggers on deploymentConfigs
        watches.push(DataService.watch("builds", context, function(builds) {
          $scope.builds = builds.by("metadata.name");
          updateRecentBuildsByOutputImage();

          intervals.push($interval(updateRecentBuildsByOutputImage, 5 * 60 * 1000)); // prune the list every 5 minutes

          updateTopologyLater();
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
            updateTopology();
          });
        });

        var updateTimeout = null;

        function updateTopology() {
          updateTimeout = null;

          var topologyRelations = [];
          var topologyItems = { };

          // Because metadata.uid is not unique among resources
          function makeId(resource) {
            return resource.kind + resource.metadata.uid;
          }

          // Add the services
          angular.forEach($scope.services, function(service) {
            topologyItems[makeId(service)] = service;
          });

          var isRecentDeployment = $filter('isRecentDeployment');
          $scope.isVisibleDeployment = function(deployment) {
            // If this is a replication controller and not a deployment, then it's visible.
            var dcName = annotationFilter(deployment, 'deploymentConfig');
            if (!dcName) {
              return true;
            }

            // If the deployment is active, it's visible.
            if (hashSizeFilter($scope.podsByDeployment[deployment.metadata.name]) > 0) {
              return true;
            }

            // Wait for deployment configs to load.
            if (!$scope.deploymentConfigs) {
              return false;
            }

            // If the deployment config has been deleted and the deployment has no replicas, hide it.
            // Otherwise all old deployments for a deleted deployment config will be visible.
            var dc = $scope.deploymentConfigs[dcName];
            if (!dc) {
              return false;
            }

            // Show the deployment if it's recent (latest or in progress) or if it's scalable.
            return isRecentDeployment(deployment, dc) || $scope.isScalable(deployment, dcName);
          };

          // Add everything related to services, each of these tables are in
          // standard form with string keys, pointing to a map of further
          // name -> resource mappings.
          [
            $scope.podsByService,
            $scope.monopodsByService,
            $scope.deploymentsByService,
            $scope.deploymentConfigsByService,
            $scope.routesByService
          ].forEach(function(map) {
            angular.forEach(map, function(resources, serviceName) {
              var service = $scope.services[serviceName];
              if (!serviceName || service) {
                angular.forEach(resources, function(resource) {
                  // Filter some items to be consistent with the tiles view.
                  if (map === $scope.monopodsByService && !showMonopod(resource)) {
                    return;
                  }

                  if (map === $scope.deploymentsByService && !$scope.isVisibleDeployment(resource)) {
                    return;
                  }

                  topologyItems[makeId(resource)] = resource;
                });
              }
            });
          });

          // Things to link to services. Note that we can push as relations
          // no non-existing items into the topology without ill effect
          [
            $scope.podsByService,
            $scope.monopodsByService,
            $scope.routesByService
          ].forEach(function(map) {
            angular.forEach(map, function(resources, serviceName) {
              var service = $scope.services[serviceName];
              if (service) {
                angular.forEach(resources, function(resource) {
                  topologyRelations.push({ source: makeId(service), target: makeId(resource) });
                });
              }
            });
          });

          // A special case, not related to services
          angular.forEach($scope.podsByDeployment, function(pods, deploymentName) {
            var deployment = $scope.deployments[deploymentName];
            if (makeId(deployment) in topologyItems) {
              angular.forEach(pods, function(pod) {
          topologyItems[makeId(pod)] = pod;
                topologyRelations.push({ source: makeId(deployment), target: makeId(pod) });
              });
            }
          });

          // Link deployment configs to their deployment
          angular.forEach($scope.deployments, function(deployment, deploymentName) {
      var deploymentConfig, annotations = deployment.metadata.annotations || {};
      var deploymentConfigName = annotations["openshift.io/deployment-config.name"] || deploymentName;
      if (deploymentConfigName && $scope.deploymentConfigs) {
              deploymentConfig = $scope.deploymentConfigs[deploymentConfigName];
              if (deploymentConfig) {
                topologyRelations.push({ source: makeId(deploymentConfig), target: makeId(deployment) });
              }
            }
          });

          $scope.$evalAsync(function() {
            $scope.topologyItems = topologyItems;
            $scope.topologyRelations = topologyRelations;
          });
        }

        function updateTopologyLater() {
          if (!updateTimeout) {
            updateTimeout = window.setTimeout(updateTopology, 100);
          }
        }

        $scope.$on("select", function(ev, resource) {
          $scope.$apply(function() {
            $scope.topologySelection = resource;
            if (resource) {
              ObjectDescriber.setObject(resource, resource.kind);
            } else {
              ObjectDescriber.clearObject();
            }
          });
        }, true);

        function selectionChanged(resource) {
          $scope.topologySelection = resource;
        }

        ObjectDescriber.onResourceChanged(selectionChanged);

        // When view changes to topology, clear source of ObjectDescriber object
        // So that selection can remain for topology view
        $scope.$watch("overviewMode", function(value) {
          if (value === "topology") {
            ObjectDescriber.source = null;
          }
        });

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
          window.clearTimeout(updateTimeout);
          ObjectDescriber.removeResourceChangedCallback(selectionChanged);
          angular.forEach(intervals, function (interval){
            $interval.cancel(interval);
          });
        });

      }));
  });
