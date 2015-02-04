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

    watches.push(DataService.watch("replicationControllers", $scope, function(deployments) {
      $scope.unfilteredDeployments = deployments.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredDeployments, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.deployments = LabelFilter.getLabelSelector().select($scope.unfilteredDeployments);
      $scope.emptyMessage = "No deployments to show";
      updateFilterWarning();
      console.log("deployments (subscribe)", $scope.deployments);
    }));    

    // Also load images and builds to fill out details in the pod template
    watches.push(DataService.watch("images", $scope, function(images) {
      $scope.images = images.by("metadata.name");
      $scope.imagesByDockerReference = images.by("dockerImageReference");
      console.log("images (subscribe)", $scope.images);
      console.log("imagesByDockerReference (subscribe)", $scope.imagesByDockerReference);
    }));    

    watches.push(DataService.watch("builds", $scope, function(builds) {
      $scope.builds = builds.by("metadata.name");
      console.log("builds (subscribe)", $scope.builds);
    }));  

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