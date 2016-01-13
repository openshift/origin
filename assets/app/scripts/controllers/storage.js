'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:StorageController
 * @description
 * # StorageController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('StorageController', function ($routeParams, $scope, AlertMessageService, DataService, ProjectsService, $filter, LabelFilter, Logger) {
    $scope.projectName = $routeParams.project;
    $scope.pvcs = {};
    $scope.unfilteredPVCs = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";

    // get and clear any alerts
    AlertMessageService.getAlerts().forEach(function(alert) {
      $scope.alerts[alert.name] = alert.data;
    });
    AlertMessageService.clearAlerts();

    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
         watches.push(DataService.watch("persistentvolumeclaims", context, function(pvcs) {
          $scope.unfilteredPVCs = pvcs.by("metadata.name");
          LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredPVCs, $scope.labelSuggestions);
          LabelFilter.setLabelSuggestions($scope.labelSuggestions);
          $scope.pvcs = LabelFilter.getLabelSelector().select($scope.unfilteredPVCs);
          $scope.emptyMessage = "No persistent volume claims to show";
          updateFilterWarning();
          Logger.log("pvcs (subscribe)", $scope.unfilteredPVCs);
        }));

        function updateFilterWarning() {
          if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.pvcs)  && !$.isEmptyObject($scope.unfilteredPVCs)) {
            $scope.alerts["storage"] = {
              type: "warning",
              details: "The active filters are hiding all persistent volume claims."
            };
          }
          else {
            delete $scope.alerts["storage"];
          }
        }

        LabelFilter.onActiveFiltersChanged(function(labelSelector) {
          // trigger a digest loop
          $scope.$apply(function() {
            $scope.pvcs = labelSelector.select($scope.unfilteredPVCs);
            updateFilterWarning();
          });
        });

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });

      }));
  });
