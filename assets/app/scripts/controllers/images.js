'use strict';
/* jshint sub: true */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ImagesController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ImagesController', function ($scope, DataService, $filter, LabelFilter, Logger) {
    $scope.imageStreams = {};
    $scope.unfilteredImageStreams = {};
    $scope.builds = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    watches.push(DataService.watch("imagestreams", $scope, function(imageStreams) {
      $scope.unfilteredImageStreams = imageStreams.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredImageStreams, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.imageStreams = LabelFilter.getLabelSelector().select($scope.unfilteredImageStreams);
      $scope.emptyMessage = "No image streams to show";
      updateFilterWarning();
      Logger.log("image streams (subscribe)", $scope.imageStreams);
    }));

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.imageStreams) && !$.isEmptyObject($scope.unfilteredImageStreams)) {
        $scope.alerts["imageStreams"] = {
          type: "warning",
          details: "The active filters are hiding all image streams."
        };
      }
      else {
        delete $scope.alerts["imageStreams"];
      }
    }

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.imageStreams = labelSelector.select($scope.unfilteredImageStreams);
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
