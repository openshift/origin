"use strict";

angular.module("openshiftConsole")
  /**
   * Provides a widget for entering route information
   * 
   * model:     The model for the input.  The model will either use or add the
   *            following keys:
   *                      {
   *                        uri:  "",
   *                        customCerts:  false, //true|false
   *                        certificate: "",
   *                        key: "",
   *                        caCertificate: ""
   *                      }
   * uri-disabled:  An expression that will make the URI text input
   *                disabled.  Enabled by default
   * security-model:  A model of the custom certificate and private key in
   *                      the form of: 
   */
  .directive("oscRouting", function(){
    return {
      require: '^form',
      restrict: 'E',
      scope: {
        route: "=model",
        uriDisabled: "=",
        uriRequired: "="
      },
      templateUrl: 'views/directives/osc-routing.html',
      link: function(scope, element, attrs, formCtl){
        scope.form = formCtl;
      }
    };
  });