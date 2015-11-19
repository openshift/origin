'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServicesController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('RoutesController', function ($scope, DataService, $filter, LabelFilter) {
    $scope.unfilteredRoutes = {};
    $scope.routes = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    watches.push(DataService.watch("routes", $scope, function(routes) {
      $scope.unfilteredRoutes = routes.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredRoutes, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.routes = LabelFilter.getLabelSelector().select($scope.unfilteredRoutes);
      $scope.emptyMessage = "No routes to show";
      updateFilterWarning();
    }));

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.routes)  && !$.isEmptyObject($scope.unfilteredRoutes)) {
        $scope.alerts["routes"] = {
          type: "warning",
          details: "The active filters are hiding all routes."
        };
      }
      else {
        delete $scope.alerts["routes"];
      }
    }

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.routes = labelSelector.select($scope.unfilteredRoutes);
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
