'use strict';

angular.module("openshiftConsole")
  .factory("PodsService", function() {
    return {
      // Generates a copy of pod for debugging crash loops.
      generateDebugPod: function(pod, containerName) {
        var container = _.find(pod.spec.containers, { name: containerName });
        if (!container) {
          return null;
        }

        // Copy the pod and make some changes for debugging.
        var debugPod = angular.copy(pod);
        debugPod.metadata = {
          // Use same naming convention as `oc debug`
          name: "debug-" + pod.metadata.name + "-" + containerName,
          annotations: {
            "debug.openshift.io/source-container": containerName,
            "debug.openshift.io/source-resource": "pod/" + pod.metadata.name
          }
        };
        debugPod.labels = {
          'debug.openshift.io/name': pod.metadata.name
        }
        debugPod.spec.restartPolicy = "Never";

        // Prevent container from stopping immediately.
        container.command = ['tail'];
        container.args = ['-f', '/dev/null'];
        debugPod.spec.containers = [container];

        return debugPod;
      }
    };
  });

