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
    $scope.pods = {};
    $scope.podsPromise = $.Deferred();
    $scope.services = {};
    $scope.servicesPromise = $.Deferred();    
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

    var projectCallback = function(project) {
      $scope.$apply(function(){
        $scope.project = project;
        $scope.projectPromise.resolve(project);
      });
    };

    DataService.getObject("projects", $scope.projectName, projectCallback, $scope);

    var podsCallback = function(pods) {
      $scope.$apply(function() {
        // Have to wipe the lists out because we get a brand new list every time when polling
        $scope.pods = {};
        $scope.podsByLabel = {};
        DataService.objectsByAttribute(pods.items, "id", $scope.pods);
        DataService.objectsByAttribute(pods.items, "labels", $scope.podsByLabel, null, "id");
      });

      console.log("podsByLabel (list)", $scope.podsByLabel);      

      $scope.servicesPromise.done($.proxy(function() {
        $scope.$apply(function() {
          podsByServiceByLabel();
        });
      }));
    };

    DataService.getList("pods", $scope.podsPromise, $scope);

    var servicesCallback = function(services) {
      $scope.$apply(function() {
        // TODO this is being fixed upstream so that items is never null
        if (services.items) {
          DataService.objectsByAttribute(services.items, "id", $scope.services);
        }
        if ($scope.servicesPromise.state() !== "resolved") {
          $scope.servicesPromise.resolve();
        }        
      });

      console.log("services (list)", $scope.services);
    };

    DataService.getList("services", $scope.servicesPromise, $scope);    


    var servicesSubscribeCallback = function(action, service) {
      $scope.$apply(function() {
        DataService.objectByAttribute(service, "id", $scope.services, action);
        podsByServiceByLabel();
      });

      console.log("services (subscribe)", $scope.services);
    };

    var podsByServiceByLabel = function() {
      for (var serviceId in $scope.services) {
        var service = $scope.services[serviceId];
        var servicePods = [];
        for (var selectorKey in service.selector) {
          var selectorValue = service.selector[selectorKey];
          var pods = $scope.podsByLabel[selectorKey][selectorValue];
          for (var podId in pods) {
            var pod = pods[podId];
            servicePods.push(pod);
          }
        }
        $scope.podsByServiceByLabel[serviceId]  =  {};
        DataService.objectsByAttribute(servicePods, "labels", $scope.podsByServiceByLabel[serviceId], null, "id");
      }

      console.log("podsByServiceByLabel", $scope.podsByServiceByLabel);      
    };

    $scope.podsPromise.done($.proxy(function(pods){
      $scope.servicesPromise.done($.proxy(function(services) {
        //we've loaded the initial set of pods and services at this point
        // TODO we'll restructure how we handle this data to prevent things from jumping around
        // in crazy structural cases by service
        servicesCallback(services);
        podsCallback(pods);

        DataService.subscribe("services", servicesSubscribeCallback, $scope);
        DataService.subscribePolling("pods", podsCallback, $scope);
      }, this));
    }, this));

    // Sets up subscription for deployments and deploymentsByConfig
    var deploymentsCallback = function(action, deployment) {
      $scope.$apply(function() {
        DataService.objectByAttribute(deployment, "metadata.name", $scope.deployments, action);
        DataService.objectByAttribute(deployment, "metadata.annotations.deploymentConfig", $scope.deploymentsByConfig, action, "id");
      });

      console.log("deployments (subscribe)", $scope.deployments);
      console.log("deploymentsByConfig (subscribe)", $scope.deploymentsByConfig);
    };
    DataService.subscribe("deployments", deploymentsCallback, $scope);

    // Sets up subscription for images and imagesByDockerReference
    var imagesCallback = function(action, image) {
      $scope.$apply(function() {
        DataService.objectByAttribute(image, "metadata.name", $scope.images, action);
        DataService.objectByAttribute(image, "dockerImageReference", $scope.imagesByDockerReference, action);
      });

      console.log("imagesByDockerReference (subscribe)", $scope.imagesByDockerReference);
    };
    DataService.subscribe("images", imagesCallback, $scope);


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
            trigger.builds[build.id] = build;
          }          
        }
      }
    };

    // Sets up subscription for deploymentConfigs, associates builds to triggers on deploymentConfigs
    var deploymentConfigsCallback = function(action, deploymentConfig) {
      $scope.$apply(function() {
        DataService.objectByAttribute(deploymentConfig, "metadata.name", $scope.deploymentConfigs, action);
        if (action === 'ADDED' || action === 'MODIFIED') {
          for (var buildId in $scope.builds) {
            associateDeploymentConfigTriggersToBuild(deploymentConfig, $scope.builds[buildId]);
          }
        }
        else if (action === 'DELETED') {
          // TODO
        }
      });

      console.log("deploymentConfigs (subscribe)", $scope.deploymentConfigs);
    };
    DataService.subscribe("deploymentConfigs", deploymentConfigsCallback, $scope);  

    // Sets up subscription for builds, associates builds to triggers on deploymentConfigs
    var buildsCallback = function(action, build) {
      $scope.$apply(function() {
        DataService.objectByAttribute(build, "metadata.name", $scope.builds, action);
        if (action === 'ADDED' || action === 'MODIFIED') {
          for (var depConfigId in $scope.deploymentConfigs) {
            associateDeploymentConfigTriggersToBuild($scope.deploymentConfigs[depConfigId], build);
          }
        }
        else if (action === 'DELETED'){
          // TODO
        }
      });

      console.log("builds (subscribe)", $scope.builds);
    };
    DataService.subscribe("builds", buildsCallback, $scope);
  });
