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
    $scope.imageStreams = {};
    $scope.unfilteredImageStreams = {};
    $scope.missingStatusTagsByImageStream = {};
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
      updateMissingStatusTags();
      updateFilterWarning();
      Logger.log("image streams (subscribe)", $scope.imageStreams);
    }));

    // Check each image stream to see if the spec tags resolve to status tags.
    // We call out missing status tags with a warning.
    function updateMissingStatusTags() {
      angular.forEach($scope.unfilteredImageStreams, function(is, name) {
        var missingStatusTags = $scope.missingStatusTagsByImageStream[name] = {};
        if (!is.spec || !is.spec.tags) {
          return;
        }

        // Index the status tags for this image stream to avoid iterating the list for every spec tag.
        var statusTagMap = {};
        if (is.status && is.status.tags) {
          angular.forEach(is.status.tags, function(tag) {
            statusTagMap[tag.tag] = true;
          });
        }

        // Make sure each spec tag has a corresponding status tag.
        angular.forEach(is.spec.tags, function(specTag) {
          if (!statusTagMap[specTag.name]) {
            missingStatusTags[specTag.name] = specTag;
          }
        });
      });
    }

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
