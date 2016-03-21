'use strict';

angular.module('openshiftConsole')
  .directive('podMetrics', function($interval,
                                    $parse,
                                    $timeout,
                                    ChartsService,
                                    MetricsService,
                                    usageValueFilter) {
    return {
      restrict: 'E',
      scope: {
        pod: '='
      },
      templateUrl: 'views/directives/_pod-metrics.html',
      link: function(scope) {
        var donutByMetric = {}, sparklineByMetric = {};
        var intervalPromise;
        var getMemoryLimit = $parse('resources.limits.memory');
        var getCPULimit = $parse('resources.limits.cpu');

        function bytesToMiB(value) {
          if (!value) {
            return value;
          }

          return value / (1024 * 1024);
        }

        scope.uniqueID = _.uniqueId('metrics-chart-');

        // Metrics to display.
        scope.metrics = [
          {
            label: "Memory",
            id: "memory/usage",
            units: "MiB",
            chartPrefix: "memory-"
          },
          {
            label: "CPU",
            id: "cpu/usage",
            units: "millicores",
            chartPrefix: "cpu-"
          }
        ];

        // Set to true when any data has been loaded (or failed to load).
        scope.loaded = false;

        // Get the URL to show in error messages.
        MetricsService.getMetricsURL().then(function(url) {
          scope.metricsURL = url;
        });

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

        scope.usageByMetric = {};

        var createDonutConfig = function(metric) {
          var chartID = '#' + metric.chartPrefix + scope.uniqueID + '-donut';
          return {
            bindto: chartID,
            onrendered: function() {
              var used = scope.usageByMetric[metric.id].used;
              ChartsService.updateDonutCenterText(chartID, used, metric.units);
            },
            donut: {
              label: {
                show: false
              },
              width: 10
            },
            legend: {
              show: false
            },
            size: {
              height: 175,
              widht: 175
            }
          };
        };

        var createSparklineConfig = function(metric) {
          return {
            bindto: '#' + metric.chartPrefix + scope.uniqueID + '-sparkline',
            axis: {
              x: {
                show: true,
                type: 'timeseries',
                // With default padding you can have negative axis tick values.
                padding: {
                  left: 0,
                  bottom: 0
                },
                tick: {
                  type: 'timeseries',
                  format: '%a %H:%M'
                }
              },
              y: {
                label: metric.units,
                min: 0,
                // With default padding you can have negative axis tick values.
                padding: {
                  left: 0,
                  top: 0,
                  bottom: 0
                },
                show: true,
                tick: {
                  format: function(value) {
                    return d3.round(value, 1);
                  }
                }
              }
            },
            legend: {
              show: false
            },
            point: {
              show: false
            },
            size: {
              height: 160
            }
          };
        };

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
          var dates = ['dates'], values = [metric.units];
          var usage = scope.usageByMetric[metric.id] = {
            total: getLimit(metric)
          };

          var mostRecentValue = data[data.length - 1].value;
          if (isNaN(mostRecentValue)) {
            mostRecentValue = 0;
          }
          if (metric.id === 'memory/usage') {
            mostRecentValue = bytesToMiB(mostRecentValue);
          }

          // Round to the closest whole number for the utilization chart.
          usage.used = d3.round(mostRecentValue);
          if (usage.total) {
            usage.available = Math.max(usage.total - usage.used, 0);
          }

          angular.forEach(data, function(point) {
            dates.push(point.timestamp);
            if (point.value === undefined || point.value === null) {
              // Don't attempt to round null values. These appear as gaps in the chart.
              values.push(point.value);
            } else {
              switch (metric.id) {
                case 'memory/usage':
                  values.push(d3.round(bytesToMiB(point.value), 2));
                break;
                case 'cpu/usage':
                  values.push(d3.round(point.value));
                break;
              }
            }
          });

          // Sparkline
          var sparklineConfig, sparklineData = {
            type: 'area',
            x: 'dates',
            columns: [ dates, values ]
          };
          if (!sparklineByMetric[metric.id]) {
            sparklineConfig = createSparklineConfig(metric);
            sparklineConfig.data = sparklineData;
            $timeout(function() {
              sparklineByMetric[metric.id] = c3.generate(sparklineConfig);
            });
          } else {
            sparklineByMetric[metric.id].load(sparklineData);
          }

          // Donut
          var donutConfig, donutData;
          if (usage.total) {
            donutData = {
              type: 'donut',
              columns: [
                ['Used', usage.used],
                ['Available', usage.available]
              ],
              colors: {
                Used: "#0088ce",      // Blue
                Available: "#d1d1d1"  // Gray
              }
            };

            if (!donutByMetric[metric.id]) {
              donutConfig = createDonutConfig(metric);
              donutConfig.data = donutData;
              $timeout(function() {
                donutByMetric[metric.id] = c3.generate(donutConfig);
              });
            } else {
              donutByMetric[metric.id].load(donutData);
            }
          }
        }

        function update() {
          var pod = scope.pod,
              container = scope.options.selectedContainer,
              start = Date.now() - scope.options.timeRange.value * 60 * 1000;

          if (!pod || !container || scope.metricsError) {
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
                scope.metricsError = {
                  status: response.status,
                  details: _.get(response, 'data.errorMsg') || response.statusText || "Status code " + response.status
                };
              }
            ).finally(function() {
              // Even on errors mark metrics as loaded to replace the
              // "Loading..." message with "No metrics to display."
              scope.loaded = true;
            });
          });
        }

        // Updates immediately and then on options changes.
        scope.$watch('options', function() {
          delete scope.metricsError;
          update();
        }, true);
        // Also update every 30 seconds.
        intervalPromise = $interval(update, 30 * 1000, false);

        scope.$on('$destroy', function() {
          if (intervalPromise) {
            $interval.cancel(intervalPromise);
            intervalPromise = null;
          }

          angular.forEach(donutByMetric, function(chart) {
            chart.destroy();
          });
          donutByMetric = null;

          angular.forEach(sparklineByMetric, function(chart) {
            chart.destroy();
          });
          sparklineByMetric = null;
        });
      }
    };
  });
