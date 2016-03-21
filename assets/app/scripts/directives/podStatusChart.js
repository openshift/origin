"use strict";

angular.module('openshiftConsole')
  .directive('podStatusChart', function($timeout,
                                        hashSizeFilter,
                                        isPullingImageFilter,
                                        isTerminatingFilter,
                                        isTroubledPodFilter,
                                        numContainersReadyFilter,
                                        Logger,
                                        ChartsService) {
    // Make sure our charts always have unique IDs even if the same deployment
    // or monopod is shown on the overview more than once.

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
        var phases = ["Running", "Not Ready", "Warning", "Failed", "Pulling", "Pending", "Succeeded", "Terminating", "Unknown"];

        $scope.chartId = _.uniqueId('pods-donut-chart-');

        function updateCenterText() {
          var total = hashSizeFilter($scope.pods), smallText;
          if (!angular.isNumber($scope.desired) || $scope.desired === total) {
            smallText = (total === 1) ? "pod" : "pods";
          } else {
            smallText = "scaling to " + $scope.desired + "...";
          }

          ChartsService.updateDonutCenterText(element[0], total, smallText);
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
                // We add all phases to the data, even if count 0, to force a cut-line at the top of the donut.
                // Don't show tooltips for phases with 0 count.
                if (!value) {
                  return undefined;
                }

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
          transition: {
            duration: 350
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
              "Not Ready": "#beedf9",
              Warning: "#f9d67a",
              Failed: "#d9534f",
              Pulling: "#d1d1d1",
              Pending: "#ededed",
              Succeeded: "#3f9c35",
              Terminating: "#00659c",
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

          // Add to scope for sr-only text.
          $scope.podStatusData = data.columns;
        }

        function isReady(pod) {
          var numReady = numContainersReadyFilter(pod);
          var total = pod.spec.containers.length;

          return numReady === total;
        }

        function getPhase(pod) {
          if (isTerminatingFilter(pod)) {
            return 'Terminating';
          }

          if (isTroubledPodFilter(pod)) {
            return 'Warning';
          }

          if (isPullingImageFilter(pod)) {
            return 'Pulling';
          }

          // Also count running, but not ready, as its own phase.
          if (pod.status.phase === 'Running' && !isReady(pod)) {
            return 'Not Ready';
          }

          return _.get(pod, 'status.phase', 'Unknown');
        }

        function countPodPhases() {
          var countByPhase = {};

          angular.forEach($scope.pods, function(pod) {
            var phase = getPhase(pod);
            countByPhase[phase] = (countByPhase[phase] || 0) + 1;
          });

          return countByPhase;
        }

        var debounceUpdate = _.debounce(updateChart, 350, { maxWait: 500 });
        $scope.$watch(countPodPhases, debounceUpdate, true);
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
