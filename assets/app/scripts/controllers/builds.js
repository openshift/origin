'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildsController', function ($scope, DataService, $filter, LabelFilter) {
    $scope.builds = {};
    $scope.unfilteredBuilds = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    var buildsCallback = function(builds) {
      $scope.$apply(function() {
        $scope.unfilteredBuilds = builds.by("metadata.name");
        LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredBuilds, $scope.labelSuggestions);
        LabelFilter.setLabelSuggestions($scope.labelSuggestions);
        $scope.builds = LabelFilter.getLabelSelector().select($scope.unfilteredBuilds);
        $scope.emptyMessage = "No builds to show";
        updateFilterWarning();
      });

      console.log("builds (subscribe)", $scope.unfilteredBuilds);
    };
    watches.push(DataService.watch("builds", $scope, buildsCallback));    

    var updateFilterWarning = function() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.builds) && !$.isEmptyObject($scope.unfilteredBuilds)) {
        $scope.alerts["builds"] = {
          type: "warning",
          details: "The active filters are hiding all builds."
        };
      }
      else {
        delete $scope.alerts["builds"];
      }      
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.builds = labelSelector.select($scope.unfilteredBuilds);
        updateFilterWarning();
      });
    });   

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });