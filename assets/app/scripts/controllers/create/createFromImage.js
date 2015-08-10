"use strict";

angular.module("openshiftConsole")
  .controller("CreateFromImageController", function ($scope,
      Logger,
      $q,
      $routeParams, 
      DataService, 
      Navigate, 
      NameGenerator, 
      ApplicationGenerator,
      TaskList,
      failureObjectNameFilter
    ){
    function initAndValidate(scope){

      if(!$routeParams.imageName){
        Navigate.toErrorPage("Cannot create from source: a base image was not specified");
      }
      if(!$routeParams.imageTag){
        Navigate.toErrorPage("Cannot create from source: a base image tag was not specified");
      }
      if(!$routeParams.sourceURL){
        Navigate.toErrorPage("Cannot create from source: source url was not specified");
      }

      scope.emptyMessage = "Loading...";
      scope.imageName = $routeParams.imageName;
      scope.imageTag = $routeParams.imageTag;
      scope.namespace = $routeParams.namespace;
      scope.buildConfig = {
        sourceUrl: $routeParams.sourceURL,
        buildOnSourceChange: true,
        buildOnImageChange: true
      };
      scope.deploymentConfig = {
        deployOnNewImage: true,
        deployOnConfigChange: true,
        envVars : {
        }
      };
      scope.routing = {
        include: true
      };
      scope.labels = {};
      scope.annotations = {};
      scope.scaling = {
        replicas: 1
      };

      DataService.get("imagestreams", scope.imageName, scope, {namespace: scope.namespace}).then(function(imageRepo){
          scope.imageRepo = imageRepo;
          var imageName = scope.imageTag;
          DataService.get("imagestreamtags", imageRepo.metadata.name + ":" + imageName, {namespace: scope.namespace}).then(function(imageStreamTag){
              scope.image = imageStreamTag.image;
              angular.forEach(imageStreamTag.image.dockerImageMetadata.ContainerConfig.Env, function(entry){
                var pair = entry.split("=");
                scope.deploymentConfig.envVars[pair[0]] = pair[1];
              });
            }, function(){
                Navigate.toErrorPage("Cannot create from source: the specified image could not be retrieved.");
              }
            );
        },
        function(){
          Navigate.toErrorPage("Cannot create from source: the specified image could not be retrieved.");
        });
        scope.name = NameGenerator.suggestFromSourceUrl(scope.buildConfig.sourceUrl);
    }

    initAndValidate($scope);

    var ifResourcesDontExist = function(resources, namespace, scope){
      var result = $q.defer();
      var successResults = [];
      var failureResults = [];
      var remaining = resources.length;

      function _checkDone() {
        if (remaining === 0) {
          if(successResults.length > 0){
            //means some resources exist with the given nanme
            result.reject(successResults); 
          }
          else
            //means no resources exist with the given nanme
            result.resolve(resources);
        }
      }

      resources.forEach(function(resource) {
        DataService.get(resource.kind, resource.metadata.name, scope, {namespace: namespace, errorNotification: false}).then(
          function (data) {
            successResults.push(data);
            remaining--;
            _checkDone();
          },
          function (data) {
            failureResults.push(data);
            remaining--;
            _checkDone();
          }
        );
      });
      return result.promise;
    }

    var createResources = function(resources){
      var titles = {
        started: "Creating application " + $scope.name + " in project " + $scope.projectName,
        success: "Created application " + $scope.name + " in project " + $scope.projectName,
        failure: "Failed to create " + $scope.name + " in project " + $scope.projectName
      };
      var helpLinks = {};

      TaskList.add(titles, helpLinks, function(){
        var d = $q.defer();
        DataService.createList(resources, $scope)
          //refactor these helpers to be common for 'newfromtemplate'
          .then(function(result) {
                var alerts = [];
                var hasErrors = false;
                if (result.failure.length > 0) {
                  result.failure.forEach(
                    function(failure) {
                      var objectName = failureObjectNameFilter(failure) || "object";
                      alerts.push({
                        type: "error",
                        message: "Cannot create " + objectName + ". ",
                        details: failure.data.message
                      });
                      hasErrors = true;
                    }
                  );
                } else {
                  alerts.push({ type: "success", message: "All resources for application " + $scope.name +
                    " were created successfully."});
                }
                d.resolve({alerts: alerts, hasErrors: hasErrors});
              }
            );
            return d.promise;
          },
          function(result) { // failure
            $scope.alerts = [
              {
                type: "error",
                message: "An error occurred creating the application.",
                details: "Status: " + result.status + ". " + result.data
              }
            ];
          }
        );
      Navigate.toProjectOverview($scope.projectName);
    };

    var elseShowWarning = function(){
      $scope.nameTaken = true;
    };

    $scope.createApp = function(){
      var resourceMap = ApplicationGenerator.generate($scope);
      //init tasks
      var resources = [];
      angular.forEach(resourceMap, function(value, key){
        if(value !== null){
          Logger.debug("Generated resource definition:", value);
          resources.push(value);
        }
      });

      ifResourcesDontExist(resources, $scope.projectName, $scope).then(
          createResources,
          elseShowWarning
        );

    };
  }
);

