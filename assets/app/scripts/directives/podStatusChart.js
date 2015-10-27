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
            width: 10,
            label: {
              show: false
            }
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
              value: function(value) {
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
             Running: "#00b9e4",
             Warning: "#f9d67a",
             Failed: "#d9534f",
             Pending: "#e8e8e8",
             Succeeded: "#3f9c35",
             Unknown: "#f9d67a"
            }
          }
        };

        function updateChart(countByPhase) {
          var columns = [];
          angular.forEach(phases, function(phase) {
            columns.push([phase, countByPhase[phase] || 0]);
          });

          if (!chart) {
            config.data.columns = columns;
            chart = c3.generate(config);
          } else {
            chart.load({ columns: columns });
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
      }
    };
  });
