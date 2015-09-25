'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServicesController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ServicesController', function ($scope, DataService, $filter, LabelFilter, Logger, $location, $anchorScroll) {
    $scope.services = {};
    $scope.unfilteredServices = {};
    $scope.routesByService = {};
    $scope.routes = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    $scope.emptyMessageRoutes = "Loading...";
    var watches = [];

    watches.push(DataService.watch("services", $scope, function(services, action) {
      $scope.unfilteredServices = services.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredServices, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.services = LabelFilter.getLabelSelector().select($scope.unfilteredServices);
      $scope.emptyMessage = "No services to show";
      updateFilterWarning();

      // Scroll to anchor on first load if location has a hash.
      if (!action && $location.hash()) {
        // Wait until the digest loop completes.
        setTimeout($anchorScroll, 10);
      }

      Logger.log("services (subscribe)", $scope.unfilteredServices);
    }));

    watches.push(DataService.watch("routes", $scope, function(routes){
        $scope.routes = routes.by("metadata.name");
        $scope.emptyMessageRoutes = "No routes to show";
        $scope.routesByService = routesByService($scope.routes);
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
