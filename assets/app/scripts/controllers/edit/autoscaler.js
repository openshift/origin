'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:EditAutoscalerController
 * @description
 * # EditAutoscalerController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('EditAutoscalerController',
              function ($scope,
                        $filter,
                        $routeParams,
                        $window,
                        APIService,
                        DataService,
                        HPAService,
                        MetricsService,
                        Navigate,
                        ProjectsService) {
    if (!$routeParams.kind && !$routeParams.name) {
      Navigate.toErrorPage("Kind and name parameters missing.");
      return;
    }

    if (['ReplicationController',
         'DeploymentConfig',
         'HorizontalPodAutoscaler'].indexOf($routeParams.kind) === -1) {
      Navigate.toErrorPage("Autoscaling not supported for kind " + $routeParams.kind + ".");
      return;
    }

    $scope.kind = $routeParams.kind;
    $scope.name = $routeParams.name;
    if ($routeParams.kind === "HorizontalPodAutoscaler") {
      // Wait for the HPA data to load before enabling the form controls.
      // This is only necessary when editing an existing HPA.
      $scope.disableInputs = true;
    } else {
      $scope.targetKind = $routeParams.kind;
      $scope.targetName = $routeParams.name;
    }

    $scope.autoscaling = {
      name: $scope.name,
      labels: {}
    };

    // Warn if metrics aren't configured when setting autoscaling options.
    MetricsService.isAvailable().then(function(available) {
      $scope.metricsWarning = !available;
    });

    $scope.alerts = {};

    // More breadcrumbs inserted later as data loads.
    $scope.breadcrumbs = [{
      title: $routeParams.project,
      link: "project/" + $routeParams.project
    }, {
      title: "Deployments",
      link: "project/" + $routeParams.project + "/browse/deployments"
    }, {
      title: "Autoscale"
    }];

    var getErrorDetails = $filter('getErrorDetails');

    var displayError = function(errorMessage, result) {
      $scope.alerts['autoscaling'] = {
        type: "error",
        message: errorMessage,
        details: getErrorDetails(result)
      };
    };

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        // Update project breadcrumb with display name.
        $scope.breadcrumbs[0].title = $filter('displayName')(project);
        $scope.project = project;

        var createHPA = function() {
          $scope.disableInputs = true;
          var hpa = {
            apiVersion: "extensions/v1beta1",
            kind: "HorizontalPodAutoscaler",
            metadata: {
              name: $scope.autoscaling.name,
              labels: $scope.autoscaling.labels
            },
            spec: {
              scaleRef: {
                kind: $routeParams.kind,
                name: $routeParams.name,
                apiVersion: "extensions/v1beta1",
                subresource: "scale"
              },
              minReplicas: $scope.autoscaling.minReplicas,
              maxReplicas: $scope.autoscaling.maxReplicas,
              cpuUtilization: {
                targetPercentage: $scope.autoscaling.targetCPU || $scope.autoscaling.defaultTargetCPU
              }
            }
          };

          DataService.create({
            resource: 'horizontalpodautoscalers',
            group: 'extensions'
          }, null, hpa, context)
            .then(function() { // Success
              // Return to the previous page
              $window.history.back();
            }, function(result) { // Failure
              $scope.disableInputs = false;
              displayError('An error occurred creating the horizontal pod autoscaler.', result);
            });
        };

        var updateHPA = function(hpa) {
          $scope.disableInputs = true;

          hpa = angular.copy(hpa);
          hpa.metadata.labels = $scope.autoscaling.labels;
          hpa.spec.minReplicas = $scope.autoscaling.minReplicas;
          hpa.spec.maxReplicas = $scope.autoscaling.maxReplicas;
          hpa.spec.cpuUtilization = {
            targetPercentage: $scope.autoscaling.targetCPU || $scope.autoscaling.defaultTargetCPU
          };

          DataService.update({
            resource: 'horizontalpodautoscalers',
            group: 'extensions'
          }, hpa.metadata.name, hpa, context)
            .then(function() { // Success
              // Return to the previous page
              $window.history.back();
            }, function(result) { // Failure
              $scope.disableInputs = false;
              displayError('An error occurred updating horizontal pod autoscaler "' + hpa.metadata.name + '".', result);
            });
        };

        var resourceGroup = {
          resource: APIService.kindToResource($routeParams.kind),
          group: $routeParams.group
        };

        DataService.get(resourceGroup, $routeParams.name, context).then(function(resource) {
          $scope.autoscaling.labels = _.get(resource, 'metadata.labels', {});

          // Are we editing an existing HPA?
          if ($routeParams.kind === "HorizontalPodAutoscaler") {
            $scope.targetKind = _.get(resource, 'spec.scaleRef.kind');
            $scope.targetName = _.get(resource, 'spec.scaleRef.name');
            _.assign($scope.autoscaling, {
              minReplicas: _.get(resource, 'spec.minReplicas'),
              maxReplicas: _.get(resource, 'spec.maxReplicas'),
              targetCPU: _.get(resource, 'spec.cpuUtilization.targetPercentage')
            });
            $scope.disableInputs = false;

            $scope.breadcrumbs.splice(2, 0, {
              title: $scope.targetName,
              link: Navigate.resourceURL($scope.targetName, $scope.targetKind, $routeParams.project)
            });

            // Update the existing HPA.
            $scope.save = function() {
              updateHPA(resource);
            };
          } else {
            $scope.breadcrumbs.splice(2, 0, {
              title: resource.metadata.name,
              link: Navigate.resourceURL(resource)
            });

            // Create a new HPA.
            $scope.save = createHPA;

            var limitRanges = {};
            var checkCPURequest = function() {
              var containers = _.get(resource, 'spec.template.spec.containers', []);
              $scope.showCPURequestWarning = !HPAService.hasCPURequest(containers, limitRanges, project);
            };

            // List limit ranges in this project to determine if there is a default
            // CPU request for autoscaling.
            DataService.list("limitranges", context, function(response) {
              limitRanges = response.by("metadata.name");
              checkCPURequest();
            });
          }
        });
    }));
  });

