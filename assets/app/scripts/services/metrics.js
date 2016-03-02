'use strict';

angular.module("openshiftConsole")
  .factory("MetricsService", function($http, $q, APIDiscovery) {
    var COUNTER_TEMPLATE = "/counters/{containerName}%2F{podUID}%2F{metric}/data";
    var GAUGE_TEMPLATE = "/gauges/{containerName}%2F{podUID}%2F{metric}/data";

    // URL template to show for each type of metric.
    var templateByMetric = {
      "cpu/usage": COUNTER_TEMPLATE,
      "memory/usage": GAUGE_TEMPLATE
    };

    var metricsURL;
    function getMetricsURL() {
      if (angular.isDefined(metricsURL)) {
        return $q.when(metricsURL);
      }

      return APIDiscovery.getMetricsURL().then(function(url) {
        // Remove trailing slash if present.
        metricsURL = (url || '').replace(/\/$/, "");
        return metricsURL;
      });
    }

    // Calculate the midtime from a point's start and end.
    function midtime(point) {
      return point.start + (point.end - point.start) / 2;
    }

    // Convert cumulative CPU usage in nanoseconds to millicores.
    function millicoresUsed(point, lastValue) {
      // Is there a gap in the data?
      if (!lastValue || !point.value) {
        return null;
      }

      // When a container restarts, the cumulative CPU usage resets to 0. As a
      // result, lastValue can be greater than the current value.  Throw out
      // those data points since we can't calculate usage as millicores.
      if (lastValue > point.value) {
        return null;
      }

      var timeInMillis = point.end - point.start;
      // Find the usage for just this bucket by comparing it to the last value.
      // Values are in nanoseconds. Calculate usage in millis.
      var usageInMillis = (point.value - lastValue) / 1000000;
      // Convert to millicores.
      return (usageInMillis / timeInMillis) * 1000;
    }

    function normalize(data, metric) {
      // Track the previous value for CPU usage calculations.
      var lastValue;

      if (!data.length) {
        return;
      }

      angular.forEach(data, function(point) {
        var value;

        // Calculate a timestamp based on the midtime if missing.
        if (!point.timestamp) {
          point.timestamp = midtime(point);
        }

        // Set point.value to the average or null if no average.
        if (!point.value || point.value === "NaN") {
          var avg = point.avg;
          point.value = (avg && avg !== "NaN") ? avg : null;
        }

        if (metric === 'cpu/usage') {
          // Save the raw value before we change it.
          value = point.value;
          point.value = millicoresUsed(point, lastValue);
          lastValue = value;
        }
      });

      // Remove the first value since it can't be used CPU utilization.
      // We want the same number of data points for all charts.
      data.shift();
      return data;
    }

    return {
      // Check if the metrics service is available. The service is considered
      // available if a metrics URL is set. Returns a promise resolved with a
      // boolean value.
      isAvailable: function() {
        return getMetricsURL().then(function(url) {
          return !!url;
        });
      },

      getMetricsURL: getMetricsURL,

      // Get metrics data for a container.
      //
      // config keyword arguments
      //   pod:            the pod object
      //   containerName:  the container name
      //   metric:         the metric to check, e.g. "memory/usage"
      //   start:          start time in millis
      //   end:            end time in millis
      //
      // Returns a promise resolved with the metrics data.
      get: function(config) {
        return getMetricsURL().then(function(metricsURL) {
          var reqURL,
              template = metricsURL + templateByMetric[config.metric],
              buckets = 60,
              start,
              end = config.end || Date.now();

          reqURL = URI.expand(template, {
            podUID: config.pod.metadata.uid,
            containerName: config.containerName,
            metric: config.metric
          }).toString();

          // Request an earlier start time and one extra bucket since we throw
          // the first data point away calculating CPU usage.
          // See normalize().
          start = Math.floor(config.start - (end - config.start) / buckets);
          var params = {
            buckets: buckets + 1,
            start: start
          };

          if (config.end) {
            params.end = config.end;
          }

          return $http.get(reqURL, {
            auth: {},
            headers: {
              Accept: 'application/json',
              'Hawkular-Tenant': config.pod.metadata.namespace
            },
            params: params
          }).then(function(response) {
            return _.assign(response, {
              data: normalize(response.data, config.metric)
            });
          });
        });
      }
    };
  });
