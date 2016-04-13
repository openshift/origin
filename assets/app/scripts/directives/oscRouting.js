"use strict";

angular.module("openshiftConsole")
  /**
   * Widget for entering route information
   *
   * model:
   *   The model for the input. The model will either use or add the following keys:
   *     {
   *       name: "",
   *       host: "",
   *       path: "",
   *       service: {}, // selected service object if services passed to directive
   *       tls.termination: "",
   *       tls.insecureEdgeTerminationPolicy: "",
   *       tls.certificate: "",
   *       tls.key: "",
   *       tls.caCertificate: "",
   *       tls.destinationCACertificate: ""
   *     }
   *
   * services:
   *   Collection of services to choose from for the route (optional)
   *
   * showNameInput:
   *   Whether to prompt the user for a route name (default: false)
   *
   * routingDisabled:
   *   An expression that will disable the form (default: false)
   */
  .directive("oscRouting", function(){
    return {
      require: '^form',
      restrict: 'E',
      scope: {
        route: "=model",
        services: "=",
        showNameInput: "=",
        routingDisabled: "="
      },
      templateUrl: 'views/directives/osc-routing.html',
      controller: function($scope) {
        $scope.disableCertificateInputs = function() {
          var termination = _.get($scope, 'route.tls.termination');
          return !termination || termination === 'passthrough';
        };
      },
      link: function(scope, element, attrs, formCtl){
        scope.form = formCtl;

        var updatePortOptions = function(service) {
          if (!service) {
            return;
          }

          scope.unnamedServicePort = service.spec.ports.length === 1 && !service.spec.ports[0].name;

          // Only show port options when there is more than one port or when a
          // single service port has a name. We want to use the service port
          // name when creating a route. (Port name is required for services
          // with more than one port.)
          if (service.spec.ports.length && !scope.unnamedServicePort) {
            scope.route.portOptions = _.map(service.spec.ports, function(portMapping) {
              return {
                port: portMapping.name,
                // \u2192 is a Unicode right arrow.
                label: portMapping.port + " \u2192 " +
                       portMapping.targetPort + " (" + portMapping.protocol + ")"
              };
            });
          } else {
            scope.route.portOptions = [];
          }
        };

        if (scope.services && !scope.route.service) {
          // Use _.find to get the first item.
          scope.route.service = _.find(scope.services);
        }

        scope.$watch('route.service', function(newValue, oldValue) {
          updatePortOptions(scope.route.service);
          // Don't overwrite the target port when editing an existing route unless the user picked a
          // different service.
          if (newValue !== oldValue || !scope.route.targetPort) {
            scope.route.targetPort = _.get(scope, 'route.portOptions[0].port');
          }
        });

        var showCertificateWarning = function() {
          if (!scope.route.tls) {
            return false;
          }

          if (scope.route.tls.termination && scope.route.tls.termination !== 'passthrough') {
            return false;
          }

          // Check if any certificate or key is set with an incompatible termination.
          return scope.route.tls.certificate ||
                 scope.route.tls.key ||
                 scope.route.tls.caCertificate ||
                 scope.route.tls.destinationCACertificate;
        };

        // Show a warning if previously-set certificates won't be used because
        // the TLS termination is now incompatible.
        scope.$watch('route.tls.termination', function() {
          if (_.get(scope, 'route.tls.termination')) {
            // If editing a route with TLS termination already set, expand the secure route options.
            scope.showSecureRouteOptions = true;
          }
          scope.showCertificatesNotUsedWarning = showCertificateWarning();
        });
      }
    };
  });
