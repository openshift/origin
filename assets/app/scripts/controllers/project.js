'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectController', function ($scope, $routeParams, DataService, $filter) {
    $scope.projectName = $routeParams.project;
    $scope.project = {};
    $scope.projectPromise = $.Deferred();
    $scope.projects = {};
    $scope.pods = {};
    $scope.services = {};
    $scope.podsByLabel = {};
    $scope.deployments = {};
    $scope.deploymentsByConfig = {};
    $scope.deploymentConfigs = {"": null}; // when we have deployments that were not created from a deploymentConfig
                                           // the explicit assignment of the "" key is needed so that the null depConfig is
                                           // iterated over during the ng-repeat in the template
    $scope.builds = {};
    $scope.images = {};
    $scope.imagesByDockerReference = {};
    $scope.podsByServiceByLabel = {};

    $scope.watches = [];

    var projectCallback = function(project) {
      $scope.$apply(function(){
        $scope.project = project;
        $scope.projectPromise.resolve(project);
      });
    };

    DataService.get("projects", $scope.projectName, $scope, projectCallback);

    var projectsCallback = function(projects) {
      $scope.$apply(function(){
        $scope.projects = projects.by("metadata.name");
      });

      console.log("projects", $scope.projects);
    };
    
    DataService.list("projects", $scope, projectsCallback);

    var podsCallback = function(pods) {
      $scope.$apply(function() {
        $scope.pods = pods.by("metadata.name");
        $scope.podsByLabel = pods.by("labels", "metadata.name");
        podsByServiceByLabel();
      });

      console.log("podsByLabel (list)", $scope.podsByLabel);      
    };

    $scope.watches.push(DataService.watch("pods", $scope, podsCallback, {poll: true}));

    var servicesCallback = function(services) {
      $scope.$apply(function() {
        $scope.services = services.by("metadata.name");
        podsByServiceByLabel();  
      });

      console.log("services (list)", $scope.services);
    };

    $scope.watches.push(DataService.watch("services", $scope, servicesCallback));

    var podsByServiceByLabel = function() {
      $scope.podsByServiceByLabel = {};
      $each($scope.services, function(name, service) {
        var servicePods = [];
        $each(service.selector, function(selectorKey, selectorValue) {
          if ($scope.podsByLabel[selectorKey]) {
            var pods = $scope.podsByLabel[selectorKey][selectorValue] || {};
            $each(pods, function(name, pod) {
              servicePods.push(pod);
            });
          }
        });
        $scope.podsByServiceByLabel[name]  =  {};
        // TODO last remaining reference to this... 
        DataService.objectsByAttribute(servicePods, "labels", $scope.podsByServiceByLabel[name], null, "metadata.name");
      });

      console.log("podsByServiceByLabel", $scope.podsByServiceByLabel);      
    };

    function parseEncodedDeploymentConfig(deployment) {
      if (deployment.annotations && deployment.annotations.encodedDeploymentConfig) {
        try {
          var depConfig = $.parseJSON(deployment.annotations.encodedDeploymentConfig);
          deployment.details = depConfig.details;
        }
        catch (e) {
          console.log("Failed to parse encoded deployment config", e);
        }
      }
    }

    // Sets up subscription for deployments and deploymentsByConfig
    var deploymentsCallback = function(deployments, action, deployment) {
      $scope.$apply(function() {
        $scope.deployments = deployments.by("metadata.name");
        $scope.deploymentsByConfig = deployments.by("annotations.deploymentConfig", "metadata.name");
        if (deployment) {
          if (action !== "DELETED") {
            parseEncodedDeploymentConfig(deployment);
          }
        }
        else {
          $each($scope.deployments, function(name, dep) {
            parseEncodedDeploymentConfig(dep);
          });
        }
      });

      console.log("deployments (subscribe)", $scope.deployments);
      console.log("deploymentsByConfig (subscribe)", $scope.deploymentsByConfig);
    };
    $scope.watches.push(DataService.watch("replicationControllers", $scope, deploymentsCallback));

    // Sets up subscription for images and imagesByDockerReference
    var imagesCallback = function(images) {
      $scope.$apply(function() {
        $scope.images = images.by("metadata.name");
        $scope.imagesByDockerReference = images.by("dockerImageReference");
      });
      
      console.log("images (subscribe)", $scope.images);
      console.log("imagesByDockerReference (subscribe)", $scope.imagesByDockerReference);
    };
    $scope.watches.push(DataService.watch("images", $scope, imagesCallback));


    var associateDeploymentConfigTriggersToBuild = function(deploymentConfig, build) {
      if (!deploymentConfig || !build) {
        return;
      }
      for (var i = 0; i < deploymentConfig.triggers.length; i++) {
        var trigger = deploymentConfig.triggers[i];
        if (trigger.type === "ImageChange") {
          var image = trigger.imageChangeParams.repositoryName + ":" + trigger.imageChangeParams.tag;
          var buildImage = build.parameters.output.registry + "/" + build.parameters.output.imageTag;
          if (image === buildImage) {
            if (!trigger.builds) {
              trigger.builds = {};
            }
            trigger.builds[build.metadata.name] = build;
          }          
        }
      }
    };

    // Sets up subscription for deploymentConfigs, associates builds to triggers on deploymentConfigs
    var deploymentConfigsCallback = function(deploymentConfigs, action, deploymentConfig) {
      $scope.$apply(function() {
        $scope.deploymentConfigs = deploymentConfigs.by("metadata.name");
        if (!action) {
          $each($scope.deploymentConfigs, function(name, depConfig) {
            $each($scope.builds, function(name, build) {
              associateDeploymentConfigTriggersToBuild(depConfig, build);
            });   
          });
        }
        else if (action !== 'DELETED') {
          $each($scope.builds, function(name, build) {
            associateDeploymentConfigTriggersToBuild(deploymentConfig, build);
          });
        }
      });

      console.log("deploymentConfigs (subscribe)", $scope.deploymentConfigs);
    };
    $scope.watches.push(DataService.watch("deploymentConfigs", $scope, deploymentConfigsCallback));

    // Sets up subscription for builds, associates builds to triggers on deploymentConfigs
    var buildsCallback = function(builds, action, build) {
      $scope.$apply(function() {
        $scope.builds = builds.by("metadata.name");
        if (!action) {
          $each($scope.builds, function(name, bld) {
            $each($scope.deploymentConfigs, function(name, depConfig) {
              associateDeploymentConfigTriggersToBuild(depConfig, bld);
            });
          });
        }        
        else if (action === 'ADDED' || action === 'MODIFIED') {
          $each($scope.deploymentConfigs, function(name, depConfig) {
            associateDeploymentConfigTriggersToBuild(depConfig, build);
          });
        }
        else if (action === 'DELETED'){
          // TODO
        }
      });

      console.log("builds (subscribe)", $scope.builds);
    };
    $scope.watches.push(DataService.watch("builds", $scope, buildsCallback));

    $scope.$on('$destroy', function(){
      for (var i = 0; i < $scope.watches.length; i++) {
        DataService.unwatch($scope.watches[i]);
      }
    });    
  });
