'use strict';

angular.module('openshiftConsole')
  .directive('podMetrics', function($interval, $parse, MetricsService, usageValueFilter) {
    return {
      restrict: 'E',
      scope: {
        pod: '=',
        alerts: '='
      },
      templateUrl: 'views/directives/_pod-metrics.html',
      link: function(scope) {
        var intervalPromise;
        var getMemoryLimit = $parse('resources.limits.memory');
        var getCPULimit = $parse('resources.limits.cpu');

        function bytesToMiB(value) {
          if (!value) {
            return value;
          }

          return value / (1024 * 1024);
        }

        // Metrics to display.
        scope.metrics = [
          {
            label: "Memory",
            id: "memory/usage",
            units: "MiB",
            chartId: "memory-chart"
          },
          {
            label: "CPU",
            id: "cpu/usage",
            units: "millicores",
            chartId: "cpu-chart"
          }
        ];

        // Set to true when any data has been loaded (or failed to load).
        scope.loaded = false;

        // Relative time options.
        scope.options = {
          rangeOptions: [{
            label: "Last 30 minutes",
            value: 30
          }, {
            label: "Last hour",
            value: 60
          }, {
            label: "Last 4 hours",
            value: 4 * 60
          }, {
            label: "Last day",
            value: 24 * 60
          }, {
            label: "Last week",
            value: 7 * 24 * 60
          }]
        };
        // Show last 30 minutes by default.
        scope.options.timeRange = scope.options.rangeOptions[0];

        scope.utilizationConfigByMetric = {};
        scope.donutConfigByMetric = {};
        scope.chartDataByMetric = {};
        scope.sparklineConfigByMetric = {};

        // Base config for sparkline charts. It's copied to add in specific
        // options like the units for the metric we're displaying.
        var baseSparklineConfig = {
          axis: {
            x: {
              show: true,
              // With default padding you can have negative axis tick values.
              padding: {
                left: 0,
                bottom: 0
              },
              tick: {
                count: 30,
                culling: true,
                fit: true,
                type: 'timeseries',
                format: '%H:%M'
              }
            },
            y: {
              label: {
                position: 'outer-center'
              },
              min: 0,
              // With default padding you can have negative axis tick values.
              padding: {
                left: 0,
                top: 0,
                bottom: 0
              },
              show: true,
              tick: {
                count: 5,
                fit: true,
                format: function(value) {
                  return d3.round(value, 1);
                }
              }
            }
          },
          point: {
            show: false
          }
        };

        // Initialize the chart configurations for each metric.
        angular.forEach(scope.metrics, function(metric) {
          scope.utilizationConfigByMetric[metric.id] = {
            units: metric.units
          };

          scope.donutConfigByMetric[metric.id] = {
            units: metric.units,
            chartId: metric.chartId
          };

          // Make a copy of the base sparkline config to modify for each metric later.
          scope.sparklineConfigByMetric[metric.id] = angular.copy(baseSparklineConfig);
        });


        function updateSparklineConfig(metric, limit) {
          var sparklineConfig = scope.sparklineConfigByMetric[metric.id];
          sparklineConfig.chartId = metric.chartId;
          sparklineConfig.units = metric.units;
          sparklineConfig.axis.y.label = metric.units;

          // If we're showing data from another day, add the abbreviated day name
          // to the time format.
          var start = moment().subtract(scope.options.timeRange.value, 'minutes');
          if (start.isSame(moment(), 'day')) {
            sparklineConfig.axis.x.tick.format = '%H:%M';
          } else {
            sparklineConfig.axis.x.tick.format = '%a %H:%M';
          }

          if (limit) {
            // The utilization sparkline chart is compressed, so show fewer ticks.
            sparklineConfig.axis.y.tick.count = 2;
          }
        }

        function getLimit(metric) {
          var container = scope.options.selectedContainer;
          switch (metric.id) {
          case 'memory/usage':
            var memLimit = getMemoryLimit(container);
            if (memLimit) {
              // Convert to MiB. usageValueFilter returns bytes.
              return bytesToMiB(usageValueFilter(memLimit));
            }
            break;
          case 'cpu/usage':
            var cpuLimit = getCPULimit(container);
            if (cpuLimit) {
              // Convert cores to millicores.
              return usageValueFilter(cpuLimit) * 1000;
            }
            break;
          }

          return null;
        }

        function updateChart(data, metric) {
          var chartData = scope.chartDataByMetric[metric.id] = {
            xData: ['dates'],
            yData: [metric.units],
            total: getLimit(metric)
          };
          updateSparklineConfig(metric, chartData.total);

          var mostRecentValue = data[data.length - 1].value;
          if (isNaN(mostRecentValue)) {
            mostRecentValue = 0;
          }
          if (metric.id === 'memory/usage') {
            mostRecentValue = bytesToMiB(mostRecentValue);
          }
          // Round to the closest whole number for the utilization chart.
          // This avoids weird rounding errors in the patternfly utilization
          // chart directive with floating point arithmetic.
          chartData.used = d3.round(mostRecentValue);

          angular.forEach(data, function(point) {
            chartData.xData.push(point.timestamp);
            switch (metric.id) {
              case 'memory/usage':
                chartData.yData.push(d3.round(bytesToMiB(point.value), 2));
              break;
              case 'cpu/usage':
                chartData.yData.push(d3.round(point.value));
              break;
            }
          });
        }

        function update() {
          var pod = scope.pod,
              container = scope.options.selectedContainer,
              start = Date.now() - scope.options.timeRange.value * 60 * 1000;

          if (!pod || !container) {
            return;
          }

          // Leave the end time off to use the server's current time as the end
          // time. This prevents an issue where the donut chart shows 0 for
          // current usage if the client clock is ahead of the server clock.
          angular.forEach(scope.metrics, function(metric) {
            MetricsService.get({
              pod: pod,
              containerName: container.name,
              metric: metric.id,
              start: start
            }).then(
              // success
              function(response) {
                updateChart(response.data, metric);
              },
              // failure
              function(response) {
                var alert = {
                  type: "error",
                  message: "Error fetching " + metric.id + " for container " + container.name + "."
                };

                if (response.data && response.data.errorMsg) {
                  alert.details = response.data.errorMsg;
                } else if (response.status === 0) {
                  alert.details = "Could not connect to metrics service.";
                } else {
                  alert.details = response.statusText || "Status code " + response.status;
                }

                scope.alerts["metrics"] = alert;
              }
            ).finally(function() {
              // Even on errors mark metrics as loaded to replace the
              // "Loading..." message with "No metrics to display."
              scope.loaded = true;
            });
          });
        }

        // Updates immediately and then on options changes.
        scope.$watch('options', update, true);
        // Also update every 30 seconds.
        intervalPromise = $interval(update, 30 * 1000, false);

        scope.$on('$destroy', function() {
          if (intervalPromise) {
            $interval.cancel(intervalPromise);
            intervalPromise = null;
          }
        });
      }
    };
  });
