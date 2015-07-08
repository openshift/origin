'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServicesController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ServicesController', function ($scope, DataService, $filter, LabelFilter, Logger) {
    $scope.services = {};
    $scope.unfilteredServices = {};
    $scope.routesByService = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    watches.push(DataService.watch("services", $scope, function(services) {
      $scope.unfilteredServices = services.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredServices, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.services = LabelFilter.getLabelSelector().select($scope.unfilteredServices);
      $scope.emptyMessage = "No services to show";
      updateFilterWarning();
      Logger.log("services (subscribe)", $scope.unfilteredServices);
    }));

    watches.push(DataService.watch("routes", $scope, function(routes){
        $scope.routesByService = routesByService(routes.by("metadata.name"));
        Logger.log("routes (subscribe)", $scope.routesByService);
    }));

    function routesByService(routes) {
        var routeMap = {};
        angular.forEach(routes, function(route, routeName){
          var to = route.spec.to;
          if (to.kind === "Service") {
            routeMap[to.name] = routeMap[to.name] || {};
            routeMap[to.name][routeName] = route;
          }
        });
        return routeMap;
    }

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.services)  && !$.isEmptyObject($scope.unfilteredServices)) {
        $scope.alerts["services"] = {
          type: "warning",
          details: "The active filters are hiding all services."
        };
      }
      else {
        delete $scope.alerts["services"];
      }
    }

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.services = labelSelector.select($scope.unfilteredServices);
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
