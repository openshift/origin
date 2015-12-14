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
   *       tls.termination: "",
   *       tls.insecureEdgeTerminationPolicy: "",
   *       tls.certificate: "",
   *       tls.key: "",
   *       tls.caCertificate: "",
   *       tls.destinationCACertificate: ""
   *     }
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
        showNameInput: "=",
        routingDisabled: "="
      },
      templateUrl: 'views/directives/osc-routing.html',
      link: function(scope, element, attrs, formCtl){
        scope.form = formCtl;

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
          scope.showCertificatesNotUsedWarning = showCertificateWarning();
        });
      }
    };
  });
