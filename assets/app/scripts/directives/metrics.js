'use strict';

angular.module('openshiftConsole')
  .directive('podMetrics', function($interval,
                                    $parse,
                                    $timeout,
                                    $q,
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
            units: "MiB",
            chartPrefix: "memory-",
            convert: bytesToMiB,
            containerMetric: true,
            datasets: [
              {
                id: "memory/usage",
                data: []
              }
            ]
          },
          {
            label: "CPU",
            units: "millicores",
            chartPrefix: "cpu-",
            containerMetric: true,
            datasets: [
              {
                id: "cpu/usage",
                data: []
              }
            ]
          },
          {
            label: "Network",
            units: "MiB",
            chartPrefix: "network-",
            chartType: "line",
            convert: bytesToMiB,
            datasets: [
              {
                id: "network/tx",
                label: "Sent",
                data: []
              },
              {
                id: "network/rx",
                label: "Received",
                data: []
              }
            ]
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
            label: "Last hour",
            value: 60
          }, {
            label: "Last 4 hours",
            value: 4 * 60
          }, {
            label: "Last day",
            value: 24 * 60
          }, {
            label: "Last 3 days",
            value: 3 * 24 * 60
          }, {
            label: "Last week",
            value: 7 * 24 * 60
          }]
        };
        // Show last hour by default.
        scope.options.timeRange = scope.options.rangeOptions[0];

        scope.usageByMetric = {};

        scope.anyUsageByMetric = function(metric) {
          return _.some(_.map(metric.datasets, 'id'), function(metricID) { return scope.usageByMetric[metricID] !== undefined; });
        };

        var createDonutConfig = function(metric) {
          var chartID = '#' + metric.chartPrefix + scope.uniqueID + '-donut';
          return {
            bindto: chartID,
            onrendered: function() {
              var used = scope.usageByMetric[metric.datasets[0].id].used;
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
                    return d3.round(value, 2);
                  }
                }
              }
            },
            legend: {
              show: metric.datasets.length > 1
            },
            point: {
              show: false
            },
            size: {
              height: 160
            }
          };
        };

        function getLimit(metricID) {
          var container = scope.options.selectedContainer;
          switch (metricID) {
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

        function updateChart(metric) {
          var dates, values = {};

          angular.forEach(metric.datasets, function(dataset) {
            var metricID = dataset.id, metricData = dataset.data;

            dates = ['dates'], values[metricID] = [dataset.label || metricID];

            var usage = scope.usageByMetric[metricID] = {
              total: getLimit(metricID)
            };

            var mostRecentValue = _.last(metricData).value;
            if (isNaN(mostRecentValue)) {
              mostRecentValue = 0;
            }
            if (metric.convert) {
              mostRecentValue = metric.convert(mostRecentValue);
            }

            // Round to the closest whole number for the utilization chart.
            usage.used = d3.round(mostRecentValue);
            if (usage.total) {
              usage.available = Math.max(usage.total - usage.used, 0);
            }

            angular.forEach(metricData, function(point) {
              dates.push(point.timestamp);
              if (point.value === undefined || point.value === null) {
                // Don't attempt to round null values. These appear as gaps in the chart.
                values[metricID].push(point.value);
              } else {
                var value = metric.convert ? metric.convert(point.value) : point.value;
                switch (metricID) {
                  case 'memory/usage':
                  case 'network/rx':
                  case 'network/tx':
                    values[metricID].push(d3.round(value, 2));
                    break;
                  default:
                    values[metricID].push(d3.round(value));
                }
              }
            });

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

              if (!donutByMetric[metricID]) {
                donutConfig = createDonutConfig(metric);
                donutConfig.data = donutData;
                $timeout(function() {
                  donutByMetric[metricID] = c3.generate(donutConfig);
                });
              } else {
                donutByMetric[metricID].load(donutData);
              }
            }
          });

          var columns = [dates].concat(_.values(values));

          // Sparkline
          var sparklineConfig, sparklineData = {
            type: metric.chartType || 'area',
            x: 'dates',
            columns: columns
          };

          var chartId = metric.chartPrefix + "sparkline";

          if (!sparklineByMetric[chartId]) {
            sparklineConfig = createSparklineConfig(metric);
            sparklineConfig.data = sparklineData;
            if (metric.chartDataColors) {
              sparklineConfig.color = { pattern: metric.chartDataColors };
            }
            $timeout(function() {
              sparklineByMetric[chartId] = c3.generate(sparklineConfig);
            });
          } else {
            sparklineByMetric[chartId].load(sparklineData);
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
            var promises = [];

            // On metrics that require more than one set of data (e.g. network
            // incoming and outgoing traffic) we perform one request for each,
            // but collect and handle all requests in one single promise below.
            // It's important that every metric uses the same 'start' timestamp
            // and number of buckets, so that the returned data for every metric
            // fit in the same collection of 'dates' and can be displayed in
            // exactly the same point in time in the graph.
            angular.forEach(_.map(metric.datasets, 'id'), function(metricID) {
              promises.push(MetricsService.get({
                pod: pod,
                // some metrics (network, disk) are not available at container
                // level (only at pod and node level)
                containerName: metric.containerMetric ? container.name : "pod",
                metric: metricID,
                start: start
              }));
            });

            // Collect all promises from every metric requested into one, so we
            // have all data the chart wants at the time of the chart creation
            // (or timeout updates, etc).
            $q.all(promises).then(
              // success
              function(responses) {
                angular.forEach(responses, function(response) {
                  _.find(metric.datasets, {'id': response.metricID}).data = response.data;
                });
                updateChart(metric);
              },
              // failure
              function(responses) {
                angular.forEach(responses, function(response) {
                  scope.metricsError = {
                    status: response.status,
                    details: _.get(response, 'data.errorMsg') || response.statusText || "Status code " + response.status
                  };
                });
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
        // Also update every 15 seconds.
        intervalPromise = $interval(update, 15 * 1000, false);

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
