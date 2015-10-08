'use strict';

angular.module('openshiftConsole')
  .directive('overviewDeployment', function($location, $timeout, LabelFilter) {
    return {
      restrict: 'E',
      scope: {
      	// Replication controller / deployment fields
        rc: '=',
        deploymentConfigId: '=',
        deploymentConfigMissing: '=',
        deploymentConfigDifferentService: '=',

        // Nested podTemplate fields
        imagesByDockerReference: '=',
        builds: '=',

        // Pods
        pods: '='
      },
      templateUrl: 'views/_overview-deployment.html',
      controller: function($scope) {
        $scope.viewPodsForDeployment = function(deployment) {
          $location.url("/project/" + deployment.metadata.namespace + "/browse/pods");
          $timeout(function() {
            LabelFilter.setLabelSelector(new LabelSelector(deployment.spec.selector, true));
          }, 1);
        };
      }
    };
  })
  .directive('overviewMonopod', function() {
    return {
      restrict: 'E',
      scope: {
        pod: '='
      },
      templateUrl: 'views/_overview-monopod.html'
    };
  })
  .directive('podTemplate', function() {
    return {
      restrict: 'E',
      scope: {
        podTemplate: '=',
        imagesByDockerReference: '=',
        builds: '='
      },
      templateUrl: 'views/_pod-template.html'
    };
  })
  .directive('pods', function() {
    return {
      restrict: 'E',
      scope: {
        pods: '=',
        projectName: '@?' //TODO optional for now
      },
      templateUrl: 'views/_pods.html',
      controller: function($scope) {
        $scope.phases = [
          "Failed",
          "Pending",
          "Running",
          "Succeeded",
          "Unknown"
        ];
        $scope.expandedPhase = null;
        $scope.warningsExpanded = false;
        $scope.expandPhase = function(phase, warningsExpanded, $event) {
          $scope.expandedPhase = phase;
          $scope.warningsExpanded = warningsExpanded;
          if ($event) {
            $event.stopPropagation();
          }
        };
      }
    };
  })
  .directive('podContent', function() {
    // sub-directive used by the pods directive
    return {
      restrict: 'E',
      scope: {
        pod: '=',
        troubled: '='
      },
      templateUrl: 'views/directives/_pod-content.html'
    };
  })  
  .directive('triggers', function() {
    var hideBuildKey = function(build) {
      return 'hide/build/' + build.metadata.uid;
    };
    return {
      restrict: 'E',
      scope: {
        triggers: '='
      },
      link: function(scope) {
        scope.isBuildHidden = function(build) {
          var key = hideBuildKey(build);
          return sessionStorage.getItem(key) === 'true';
        };
        scope.hideBuild = function(build) {
          var key = hideBuildKey(build);
          sessionStorage.setItem(key, 'true');
        };
      },
      templateUrl: 'views/_triggers.html'
    };
  })
  .directive('deploymentConfigMetadata', function() {
    return {
      restrict: 'E',
      scope: {
        deploymentConfigId: '=',
        exists: '=',
        differentService: '='
      },
      templateUrl: 'views/_deployment-config-metadata.html'
    };
  })
  .directive('annotations', function() {
    return {
      restrict: 'E',
      scope: {
        annotations: '='
      },
      templateUrl: 'views/directives/annotations.html',
      link: function(scope) {
        scope.expandAnnotations = false;
        scope.toggleAnnotations = function() {
          scope.expandAnnotations = !scope.expandAnnotations;
        };
      }
    };
  });
