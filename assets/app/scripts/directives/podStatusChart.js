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
        // The phases to show (in order).
        var phases = ["Running", "Warning", "Failed", "Pending", "Succeeded", "Unknown"];

        lastId++;
        $scope.chartId = 'pods-donut-chart-' + lastId;

        // c3.js config for the pods donut chart
        $scope.chartConfig = {
          type: "donut",
          donut: {
            width: 8,
            label: {
              show: false
            }
          },
          size: {
            height: 130,
            width: 130
          },
          legend: {
            show: false
          },
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
             Running: "#6eb664",
             Warning: "#f9d67a",
             Failed: "#d9534f",
             Pending: "#e8e8e8",
             Succeeded: "#0088ce",
             Unknown: "#f9d67a"
            }
          }
        };

        var updateCenterText = function() {
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
          donutChartTitle.html('');
          donutChartTitle.insert('tspan').text(total).classed('pod-count donut-title-big-pf', true).attr('dy', 0).attr('x', 0);
          donutChartTitle.insert('tspan').text(smallText).classed('donut-title-small-pf', true).attr('dy', 20).attr('x', 0);
        };

        var updateChart = function() {
          // Make sure we're inside a digest loop.
          $scope.$evalAsync(function() {
            var countByPhase = {};
            var config = $scope.chartConfig;
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

            config.data.columns = [];
            angular.forEach(phases, function(phase) {
              var count = countByPhase[phase];
              if (count) {
                config.data.columns.push([phase, count]);
              }
            });

            config.oninit = updateCenterText;
          });
        };

        $scope.$watch('pods', updateChart, true);
        $scope.$watch('desired', updateCenterText);
      }
    };
  });
