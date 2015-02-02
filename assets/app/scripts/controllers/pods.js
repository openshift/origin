'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodsController', function ($scope, DataService, $filter, LabelFilter) {
    $scope.pods = {};
    $scope.unfilteredPods = {};
    $scope.images = {};
    $scope.imagesByDockerReference = {};
    $scope.builds = {};    
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    watches.push(DataService.watch("pods", $scope, function(pods) {
      $scope.unfilteredPods = pods.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredPods, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.pods = LabelFilter.getLabelSelector().select($scope.unfilteredPods);
      $scope.emptyMessage = "No pods to show";
      updateFilterWarning();
      console.log("pods (subscribe)", $scope.unfilteredPods);
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
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.pods) && !$.isEmptyObject($scope.unfilteredPods)) {
        $scope.alerts["pods"] = {
          type: "warning",
          details: "The active filters are hiding all pods."
        };
      }
      else {
        delete $scope.alerts["pods"];
      }       
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.pods = labelSelector.select($scope.unfilteredPods);
        updateFilterWarning();
      });
    });   

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });     
  });