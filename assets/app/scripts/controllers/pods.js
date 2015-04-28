'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodsController', function ($scope, DataService, $filter, LabelFilter, Logger, ImageStreamResolver, ProxyPod) {
    $scope.pods = {};
    $scope.unfilteredPods = {};
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};    
    $scope.labelSuggestions = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";
    var watches = [];

    $scope.gotoContainerView = function(container, jolokiaUrl) {
      // console.log("OPENSHIFT_CONFIG: ", OPENSHIFT_CONFIG);
      // console.log("Clicked! container: ", container, " jolokiaUrl: ", jolokiaUrl);
      var returnTo = window.location.href;
      var title = Core.pathGet(container, ['name']) || 'Untitled Container';
      var targetURI = new URI();
      targetURI.path('/console/java/')
               .query({
                 jolokiaUrl: jolokiaUrl,
                 title: title,
                 returnTo: returnTo
               });
      // console.log("TargetURI: ", targetURI.toString());
      window.location.href = targetURI.toString();
    };

    watches.push(DataService.watch("pods", $scope, function(pods) {
      $scope.unfilteredPods = pods.by("metadata.name");
      // Add a jolokia URL for any container that has a jolokia port
      _.forIn($scope.unfilteredPods, function(pod, id) {
        var namespace = Core.pathGet(pod, ['metadata', 'namespace']) || 'default';
        var containers = Core.pathGet(pod, ['spec', 'containers']) || [];
        _.forEach(containers, function(container, index) {
          var containerState = Core.pathGet(pod, ["status", "containerStatuses", index]);
          if (!containerState) {
            return;
          }
          if (!Core.pathGet(containerState, ['state', 'running'])) {
            return;
          }
          var ports = Core.pathGet(container, ['ports']);
          if (ports && ports.length > 0) {
            var jolokiaPort = _.find(ports, function(port) {
              return port.name && port.name.toLowerCase() === 'jolokia';
            });
            if (jolokiaPort) {
              container.jolokiaUrl = UrlHelpers.join(ProxyPod(namespace, id, jolokiaPort.containerPort), 'jolokia') + '/';
            }
          }
        });
      });

      ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredPods, $scope.labelSuggestions);
      LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      $scope.pods = LabelFilter.getLabelSelector().select($scope.unfilteredPods);
      $scope.emptyMessage = "No pods to show";
      updateFilterWarning();
      Logger.log("pods (subscribe)", $scope.unfilteredPods);
    }));    

    // Sets up subscription for imageStreams
    watches.push(DataService.watch("imageStreams", $scope, function(imageStreams) {
      $scope.imageStreams = imageStreams.by("metadata.name");
      ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
      ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
      Logger.log("imageStreams (subscribe)", $scope.imageStreams);
    })); 

    watches.push(DataService.watch("builds", $scope, function(builds) {
      $scope.builds = builds.by("metadata.name");
      Logger.log("builds (subscribe)", $scope.builds);
    }));   

    var updateFilterWarning = function() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.pods) && !$.isEmptyObject($scope.unfilteredPods)) {
        $scope.alerts["pods"] = {
          type: "warning",
          details: "The active filters are hiding all pods."
        };
      }
      else {
        delete $scope.alerts["pods"];
      }       
    };

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.pods = labelSelector.select($scope.unfilteredPods);
        updateFilterWarning();
      });
    });   

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });     
  });
