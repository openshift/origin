"use strict";

angular.module('openshiftConsole')
  .directive('editProbe', function() {
    return {
      restrict: 'E',
      scope: {
        probe: '=',
        exposedPorts: '='
      },
      templateUrl: 'views/directives/_edit-probe.html',
      link: function(scope) {
        scope.id = _.uniqueId('edit-probe-');
        scope.probe = scope.probe || {};

        // Map of previous probes by type so switching to a different type and
        // back remembers the previous values.
        scope.previousProbes = {};

        // Only allow selecting TCP ports for HTTP and TCP socket checks.
        scope.tcpPorts = _.filter(scope.exposedPorts, { protocol: "TCP" });

        var updateSelectedType = function(newType, oldType) {
          scope.probe = scope.probe || {};

          // Remember the values entered for `oldType`.
          scope.previousProbes[oldType] = scope.probe[oldType];
          delete scope.probe[oldType];

          // Use previous values when switching back to `newType` if found.
          scope.probe[newType] = scope.previousProbes[newType];

          // If no previous values, set the defaults.
          if (!scope.probe[newType]) {
            switch (newType) {
            case 'httpGet':
            case 'tcpSocket':
              var firstPort = _.head(scope.tcpPorts);
              scope.probe[newType] = {
                port: firstPort ? firstPort.containerPort : ''
              };
              break;
            case 'exec':
              scope.probe = {
                exec: {
                  command: []
                }
              };
              break;
            }
          }
        };

        // Initialize type from existing data.
        if (scope.probe.httpGet) {
          scope.type = 'httpGet';
        } else if (scope.probe.exec) {
          scope.type = 'exec';
        } else if (scope.probe.tcpSocket) {
          scope.type = 'tcpSocket';
        } else {
          // Set defaults for new probe.
          scope.type = 'httpGet';
          updateSelectedType('httpGet');
        }

        scope.$watch('type', function(newType, oldType) {
          if (newType === oldType) {
            return;
          }

          updateSelectedType(newType, oldType);
        });
      }
    };
  });
