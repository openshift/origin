'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DeploymentsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DeploymentsController', function ($scope, DataService, $filter, LabelFilter, Logger, ImageStreamResolver, DeploymentsService) {
    $scope.deployments = {};
    $scope.unfilteredDeploymentConfigs = {};
    // leave undefined so we know when data is loaded
    $scope.deploymentConfigs = undefined;
    $scope.deploymentsByDeploymentConfig = {};
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    $scope.expandedDeploymentConfigRow = {};
    $scope.unfilteredReplicationControllers = {};

    var watches = [];

    watches.push(DataService.watch("replicationcontrollers", $scope, function(deployments, action, deployment) {
      $scope.deployments = deployments.by("metadata.name");

      var deploymentConfigName;
      var deploymentName;
      if (deployment) {
        deploymentConfigName = $filter('annotation')(deployment, 'deploymentConfig');
        deploymentName = deployment.metadata.name;
      }

      $scope.deploymentsByDeploymentConfig = DeploymentsService.associateDeploymentsToDeploymentConfig($scope.deployments, $scope.deploymentConfigs, true); 
      if ($scope.deploymentsByDeploymentConfig['']) {
        $scope.unfilteredReplicationControllers = $scope.deploymentsByDeploymentConfig[''];
        $scope.deploymentsByDeploymentConfig[''] = LabelFilter.getLabelSelector().select($scope.deploymentsByDeploymentConfig['']);
      }
      updateFilterWarning();

      if (!action) {
        // Loading of the page that will create deploymentConfigDeploymentsInProgress structure, which will associate running deployment to his deploymentConfig.
        $scope.deploymentConfigDeploymentsInProgress = DeploymentsService.associateRunningDeploymentToDeploymentConfig($scope.deploymentsByDeploymentConfig);
      } else if (action === 'ADDED' || (action === 'MODIFIED' && ['New', 'Pending', 'Running'].indexOf($filter('deploymentStatus')(deployment)) > -1)) {
        // When new deployment id instantiated/cloned, or in case of a retry, associate him to his deploymentConfig and add him into deploymentConfigDeploymentsInProgress structure.
        $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] = $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] || {};
        $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName] = deployment;
      } else if (action === 'MODIFIED') {
        // After the deployment ends remove him from the deploymentConfigDeploymentsInProgress structure.
        var status = $filter('deploymentStatus')(deployment);
        if (status === "Complete" || status === "Failed"){
          delete $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName];
        }
      }

      // Extract the causes from the encoded deployment config
      if (deployment) {
        if (action !== "DELETED") {
          deployment.causes = $filter('deploymentCauses')(deployment);
        }
      }
      else {
        angular.forEach($scope.deployments, function(deployment) {
          deployment.causes = $filter('deploymentCauses')(deployment);
        });
      }

      Logger.log("deployments (subscribe)", $scope.deployments);
    }));

    watches.push(DataService.watch("deploymentconfigs", $scope, function(deploymentConfigs) {
      $scope.unfilteredDeploymentConfigs = deploymentConfigs.by("metadata.name");
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredDeploymentConfigs, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.deploymentConfigs = LabelFilter.getLabelSelector().select($scope.unfilteredDeploymentConfigs);
      $scope.emptyMessage = "No deployments to show";
      $scope.deploymentsByDeploymentConfig = DeploymentsService.associateDeploymentsToDeploymentConfig($scope.deployments, $scope.deploymentConfigs, true);
      if ($scope.deploymentsByDeploymentConfig['']) {
        $scope.unfilteredReplicationControllers = $scope.deploymentsByDeploymentConfig[''];
        $scope.deploymentsByDeploymentConfig[''] = LabelFilter.getLabelSelector().select($scope.deploymentsByDeploymentConfig['']);
      }
      updateFilterWarning();
      Logger.log("deploymentconfigs (subscribe)", $scope.deploymentConfigs);
    }));

    function updateFilterWarning() {
      var isFiltering = !LabelFilter.getLabelSelector().isEmpty();
      var isFilteringAllDCs = $.isEmptyObject($scope.deploymentConfigs) && !$.isEmptyObject($scope.unfilteredDeploymentConfigs);
      var thereAreDCs = !$.isEmptyObject($scope.unfilteredDeploymentConfigs);
      var isFilteringAllRCs = $.isEmptyObject($scope.deploymentsByDeploymentConfig['']) && !$.isEmptyObject($scope.unfilteredReplicationControllers);
      var thereAreRCs = !$.isEmptyObject($scope.unfilteredReplicationControllers);

      if (isFiltering && (isFilteringAllDCs || !thereAreDCs) && (isFilteringAllRCs || !thereAreRCs) && (thereAreDCs || thereAreRCs)) {
        $scope.alerts["deployments"] = {
          type: "warning",
          details: "The active filters are hiding all deployments."
        };
      }
      else {
        delete $scope.alerts["deployments"];
      }
    }

    $scope.showEmptyMessage = function() {
      if ($filter('hashSize')($scope.deploymentsByDeploymentConfig) === 0) {
        return true;
      }

      if ($filter('hashSize')($scope.deploymentsByDeploymentConfig) === 1 && $scope.deploymentsByDeploymentConfig['']) {
        return true;
      }

      return false;
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.deploymentConfigs = labelSelector.select($scope.unfilteredDeploymentConfigs);
        $scope.deploymentsByDeploymentConfig = DeploymentsService.associateDeploymentsToDeploymentConfig($scope.deployments, $scope.deploymentConfigs, true); 
        if ($scope.deploymentsByDeploymentConfig['']) {
          $scope.unfilteredReplicationControllers = $scope.deploymentsByDeploymentConfig[''];
          $scope.deploymentsByDeploymentConfig[''] = LabelFilter.getLabelSelector().select($scope.deploymentsByDeploymentConfig['']);
        }
        updateFilterWarning();
      });
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
