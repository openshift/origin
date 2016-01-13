'use strict';

angular.module("openshiftConsole")
  .factory("StorageService", function() {
    return {
      createVolume: function(name, persistentVolumeClaim) {
        return {
          name: name,
          persistentVolumeClaim: {
            claimName: persistentVolumeClaim.metadata.name
          }
        };
      },
      createVolumeMount: function(name, mountPath) {
        return {
          name: name,
          mountPath: mountPath
        };
      }
    };
  });
