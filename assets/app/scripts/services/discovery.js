'use strict';

angular.module('openshiftConsole')
  .factory('APIDiscovery', ['LOGGING_URL', 'METRICS_URL', '$q', function(LOGGING_URL, METRICS_URL, $q) {
    return {
      // Simulate asynchronous requests for now. If these are ever updated to call to a discovery
      // endpoint, we need to make sure to trigger a digest loop using (or update all callers).
      getLoggingURL: function() {
        return $q.when(LOGGING_URL);
      },
      getMetricsURL: function() {
        return $q.when(METRICS_URL);
      }
    };
  }]);
