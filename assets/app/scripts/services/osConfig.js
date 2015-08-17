'use strict';

angular
  .module('openshiftConsole')
  .factory('osConfig', [
    function() {

      var getConfig = function() {
        return window.OPENSHIFT_CONFIG;
      };

      var getProtocol = function() {
        return window.location.protocol === "http:" ?
                "http" :
                "https";
      };

      return {
        getConfig: getConfig,
        getProtocol: getProtocol
      };
    }
  ]);
