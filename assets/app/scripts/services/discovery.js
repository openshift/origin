'use strict';

angular.module('openshiftConsole')
  .factory('APIDiscovery', ['LOGGING_URL', 'METRICS_URL', '$q', function(LOGGING_URL, METRICS_URL, $q) {
    return {
      getLoggingURL: function() {
        return $q.when(LOGGING_URL);
      },
      getMetricsURL: function() {
        return $q.when(METRICS_URL);
      }
    };
  }]);
