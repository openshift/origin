"use strict";

angular.module('openshiftConsole')
  .directive('podStatusChart', function($timeout, hashSizeFilter, isTroubledPodFilter, Logger) {
    // Make sure our charts always have unique IDs even if the same deployment
    // or monopod is shown on the overview more than once.
    var lastId = 0;

    return {
      restrict: 'E',
      scope: {
        pods: '=',
        desired: '=?'
      },
      templateUrl: 'views/_pod-status-chart.html',
      link: function($scope, element) {
        var chart, config;

        // The phases to show (in order).
        var phases = ["Running", "Warning", "Failed", "Pending", "Succeeded", "Unknown"];

        lastId++;
        $scope.chartId = 'pods-donut-chart-' + lastId;

        function updateCenterText() {
          var donutChartTitle = d3.select(element[0]).select('text.c3-chart-arcs-title'),
            total = hashSizeFilter($scope.pods),
            smallText;
          if (!donutChartTitle) {
            Logger.warn("Can't select donut title element");
            return;
          }

          if (!angular.isNumber($scope.desired) || $scope.desired === total) {
            smallText = (total === 1) ? "pod" : "pods";
          } else {
            smallText = "scaling to " + $scope.desired + "...";
          }

          // Replace donut title content.
          donutChartTitle.selectAll('*').remove();
          donutChartTitle.insert('tspan').text(total).classed('pod-count donut-title-big-pf', true).attr('dy', 0).attr('x', 0);
          donutChartTitle.insert('tspan').text(smallText).classed('donut-title-small-pf', true).attr('dy', 20).attr('x', 0);
        }

        // c3.js config for the pods donut chart
        config = {
          type: "donut",
          bindto: '#' + $scope.chartId,
          donut: {
            expand: false,
            label: {
              show: false
            },
            width: 10
          },
          size: {
            height: 150,
            width: 150
          },
          legend: {
            show: false
          },
          onrendered: updateCenterText,
          tooltip: {
            format: {
              value: function(value, ratio, id) {
                // Disable the tooltip for empty donuts.
                if (id === "Empty") {
                  return undefined;
                }

                // Show the count rather than a percentage.
                return value;
              }
            },
            position: function() {
              // Position in the top-left to avoid problems with tooltip text wrapping.
              return { top: 0, left: 0 };
            }
          },
          data: {
            type: "donut",
            groups: [ phases ],
            // Keep groups in our order.
            order: null,
            colors: {
              // Dummy group for an empty chart. Gray outline added in CSS.
              Empty: "#ffffff",
              Running: "#00b9e4",
              Warning: "#f9d67a",
              Failed: "#d9534f",
              Pending: "#e8e8e8",
              Succeeded: "#3f9c35",
              Unknown: "#f9d67a"
            },
            selection: {
              enabled: false
            }
          }
        };

        function updateChart(countByPhase) {
          var data = {
            columns: []
          };
          angular.forEach(phases, function(phase) {
            data.columns.push([phase, countByPhase[phase] || 0]);
          });

          if (hashSizeFilter(countByPhase) === 0) {
            // Add a dummy group to draw an arc, which we style in CSS.
            data.columns.push(["Empty", 1]);
          } else {
            // Unload the dummy group if present when there's real data.
            data.unload = "Empty";
          }

          if (!chart) {
            config.data.columns = data.columns;
            chart = c3.generate(config);
          } else {
            chart.load(data);
          }
        }

        function countPodPhases() {
          var countByPhase = {};
          var incrementCount = function(phase) {
            countByPhase[phase] = (countByPhase[phase] || 0) + 1;
          };

          angular.forEach($scope.pods, function(pod) {
            // Count 'Warning' as its own phase, even if not strictly accurate,
            // so it appears in the donut chart. Warnings are too important not
            // to call out.
            if (isTroubledPodFilter(pod)) {
              incrementCount('Warning');
            } else {
              incrementCount(pod.status.phase);
            }
          });

          return countByPhase;
        }

        $scope.$watch(countPodPhases, updateChart, true);
        $scope.$watch('desired', updateCenterText);

        $scope.$on('destroy', function() {
          if (chart) {
            // http://c3js.org/reference.html#api-destroy
            chart = chart.destroy();
          }
        });
      }
    };
  });
