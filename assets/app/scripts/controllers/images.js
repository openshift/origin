'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ImagesController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ImagesController', function ($scope, DataService, $filter, LabelFilter, Logger) {
    $scope.images = {};
    $scope.unfilteredImages = {};
    $scope.builds = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};    
    $scope.emptyMessage = "Loading...";
    var watches = [];

    watches.push(DataService.watch("images", $scope, function(images) {
      $scope.unfilteredImages = images.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredImages, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.images = LabelFilter.getLabelSelector().select($scope.unfilteredImages);
      $scope.emptyMessage = "No images to show";
      updateFilterWarning();
      Logger.log("images (subscribe)", $scope.images);
    }));    

    // Also load builds so we can link out to builds associated with images
    watches.push(DataService.watch("builds", $scope, function(builds) {
      $scope.builds = builds.by("metadata.name");
      Logger.log("builds (subscribe)", $scope.builds);
    }));     

    var updateFilterWarning = function() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.images) && !$.isEmptyObject($scope.unfilteredImages)) {
        $scope.alerts["images"] = {
          type: "warning",
          details: "The active filters are hiding all images."
        };
      }
      else {
        delete $scope.alerts["images"];
      }       
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.images = labelSelector.select($scope.unfilteredImages);
        updateFilterWarning();
      });
    });  

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });        
  });