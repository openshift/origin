'use strict';

angular.module("openshiftConsole")
  .factory("MetricsService", function($http, $q, APIDiscovery) {
    var COUNTER_TEMPLATE = "/counters/{containerName}%2F{podUID}%2F{metric}/data";
    var GAUGE_TEMPLATE = "/gauges/{containerName}%2F{podUID}%2F{metric}/data";

    // URL template to show for each type of metric.
    var templateByMetric = {
      "cpu/usage": COUNTER_TEMPLATE,
      "memory/usage": GAUGE_TEMPLATE,
      "network/rx": COUNTER_TEMPLATE,
      "network/tx": COUNTER_TEMPLATE
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
    function millicoresUsed(point) {
      // Is there a gap in the data?
      if (!point.min || !point.max || point.samples < 2) {
        return null;
      }

      var timeInMillis = point.end - point.start;
      // Find the usage for just this bucket by comparing it to the last value.
      // Values are in nanoseconds. Calculate usage in millis.
      var usageInMillis = (point.max - point.min) / 1000000;
      // Convert to millicores.
      return (usageInMillis / timeInMillis) * 1000;
    }

    // Convert cumulative usage to usage rate, doesn't change units.
    function bytesUsed(point) {
      // Is there a gap in the data?
      if (!point.min || !point.max || point.samples < 2) {
        return null;
      }

      return point.max - point.min;
    }

    function normalize(data, metricID) {
      // Track the previous value for CPU usage calculations.
      var lastValue;

      if (!data.length) {
        return;
      }

      angular.forEach(data, function(point) {
        // Calculate a timestamp based on the midtime if missing.
        if (!point.timestamp) {
          point.timestamp = midtime(point);
        }

        // Set point.value to the average or null if no average.
        if (!point.value || point.value === "NaN") {
          var avg = point.avg;
          point.value = (avg && avg !== "NaN") ? avg : null;
        }

        if (metricID === 'cpu/usage') {
          point.value = millicoresUsed(point, lastValue);
        }

        // Network is cumulative, convert to amount per point.
        if (/network\/rx|tx/.test(metricID)) {
          point.value = bytesUsed(point);
        }
      });

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
              buckets = 60;

          reqURL = URI.expand(template, {
            podUID: config.pod.metadata.uid,
            containerName: config.containerName,
            metric: config.metric
          }).toString();

          var params = {
            buckets: buckets,
            start: config.start
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
              metricID: config.metric,
              data: normalize(response.data, config.metric)
            });
          });
        });
      }
    };
  });
