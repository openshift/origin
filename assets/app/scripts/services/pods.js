'use strict';

angular.module("openshiftConsole")
  .factory("PodsService", function($filter) {
    var getLabel = $filter('label');
    var debugLabelKey = _.constant('debug.openshift.io/name');

    return {
      getDebugLabel: function(pod) {
        return getLabel(pod, debugLabelKey());
      },

      // Generates a copy of pod for debugging crash loops.
      generateDebugPod: function(pod, containerName) {
        var container = _.find(pod.spec.containers, { name: containerName });
        if (!container) {
          return null;
        }

        // Copy the pod and make some changes for debugging. Use the same
        // metadata as `oc debug`.
        var debugPod = angular.copy(pod);
        debugPod.metadata = {
          name: pod.metadata.name + "-debug",
          annotations: {
            "debug.openshift.io/source-container": containerName,
            "debug.openshift.io/source-resource": "pod/" + pod.metadata.name
          },
          labels: {}
        };
        debugPod.metadata.labels[debugLabelKey()] = pod.metadata.name;

        // Never restart.
        debugPod.spec.restartPolicy = "Never";
        debugPod.status = {};
        delete container.readinessProbe;
        delete container.livenessProbe;

        // Prevent container from stopping immediately.
        container.command = ['sleep'];
        // Sleep for one hour. This will cause the container to stop after one
        // hour if for some reason the pod is not deleted.
        container.args = ['' + (60 * 60)];
        debugPod.spec.containers = [container];

        return debugPod;
      }
    };
  });
