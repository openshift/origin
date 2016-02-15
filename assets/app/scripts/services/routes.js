'use strict';

angular.module("openshiftConsole")
  .factory("RoutesService", function() {
    var isPortNamed = function(port) {
      return angular.isString(port);
    };

    var getServicePortForRoute = function(targetPort, service) {
      return _.find(service.spec.ports, function(servicePort) {
        if (isPortNamed(targetPort)) {
          // When using a named port in the route target port, it refers to the service port.
          return servicePort.name === targetPort;
        }

        // Otherwise it refers to the container port (the service target port).
        // If service target port is a string, we won't be able to correlate the route port.
        return servicePort.targetPort === targetPort;
      });
    };

    var addRouteTargetWarnings = function(route, service, warnings) {
      // Has the service been deleted?
      if (!service) {
        warnings.push('Routes to service "' + route.spec.to.name + '", but service does not exist.');
        return;
      }

      var targetPort = route.spec.port ? route.spec.port.targetPort : null;
      if (!targetPort) {
        if (service.spec.ports.length > 1) {
          warnings.push('Route has no target port, but service "' + service.metadata.name + '" has multiple ports. ' +
                       'The route will round robin traffic across all exposed ports on the service.');
        }

        // Nothing else to check.
        return;
      }

      // Warn when service doesn't have a port that matches target port.
      var servicePort = getServicePortForRoute(targetPort, service);
      if (!servicePort) {
        if (isPortNamed(targetPort)) {
          warnings.push('Route target port is set to "' + targetPort + '", but service "' + service.metadata.name + '" has no port with that name.');
        } else {
          warnings.push('Route target port is set to "' + targetPort + '", but service "' + service.metadata.name + '" does not expose that port.');
        }
      }
    };

    var addTLSWarnings = function(route, warnings) {
      if (!route.spec.tls) {
        return;
      }

      if (!route.spec.tls.termination) {
        warnings.push('Route has a TLS configuration, but no TLS termination type is specified. TLS will not be enabled until a termination type is set.');
      }

      if (route.spec.tls.termination === 'passthrough' && route.spec.path) {
        warnings.push('Route path "' + route.spec.path + '" will be ignored since the route uses passthrough termination.');
      }
    };

    return {
      // Gets warnings about a route.
      //
      // Parameters:
      //   route   - the route (required)
      //   service - the service routed to
      //             If null, assumes service does not exist.
      //
      // Returns: Array of warning messages.
      getRouteWarnings: function(route, service) {
        var warnings = [];

        if (route.spec.to.kind === 'Service') {
          addRouteTargetWarnings(route, service, warnings);
        }
        addTLSWarnings(route, warnings);

        return warnings;
      },

      getServicePortForRoute: getServicePortForRoute
    };
  });
