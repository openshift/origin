'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DeploymentsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DeploymentsController', function ($scope, DataService, $filter, LabelFilter) {
    $scope.deployments = {};
    $scope.unfilteredDeployments = {};
    $scope.images = {};
    $scope.imagesByDockerReference = {};
    $scope.builds = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};    
    $scope.emptyMessage = "Loading...";
    var watches = [];

    var deploymentsCallback = function(deployments) {
      $scope.$apply(function() {
        $scope.unfilteredDeployments = deployments.by("metadata.name");
        LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredDeployments, $scope.labelSuggestions);
        LabelFilter.setLabelSuggestions($scope.labelSuggestions);
        $scope.deployments = LabelFilter.getLabelSelector().select($scope.unfilteredDeployments);
        $scope.emptyMessage = "No deployments to show";
        updateFilterWarning();
      });

      console.log("deployments (subscribe)", $scope.deployments);
    };
    watches.push(DataService.watch("replicationControllers", $scope, deploymentsCallback));    


    // Also load images and builds to fill out details in the pod template
    var imagesCallback = function(images) {
      $scope.$apply(function() {
        $scope.images = images.by("metadata.name");
        $scope.imagesByDockerReference = images.by("dockerImageReference");
      });
      
      console.log("images (subscribe)", $scope.images);
      console.log("imagesByDockerReference (subscribe)", $scope.imagesByDockerReference);
    };
    watches.push(DataService.watch("images", $scope, imagesCallback));    

    var buildsCallback = function(builds) {
      $scope.$apply(function() {
        $scope.builds = builds.by("metadata.name");
      });

      console.log("builds (subscribe)", $scope.builds);
    };
    watches.push(DataService.watch("builds", $scope, buildsCallback));  

    var updateFilterWarning = function() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.deployments) && !$.isEmptyObject($scope.unfilteredDeployments)) {
        $scope.alerts["deployments"] = {
          type: "warning",
          details: "The active filters are hiding all deployments."
        };
      }
      else {
        delete $scope.alerts["deployments"];
      }      
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.deployments = labelSelector.select($scope.unfilteredDeployments);
        updateFilterWarning();
      });
    });   

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });