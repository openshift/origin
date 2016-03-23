"use strict";

angular.module('openshiftConsole')
  .directive('quotaUsageChart', function($filter, Logger) {
    return {
      restrict: 'E',
      scope: {
        used: '=',
        total: '=',
        // 'cpu' or 'memory'
        type: '@',
        // Defaults to 'bottom'.
        // http://c3js.org/reference.html#legend-position
        legendPosition: '@?'
      },
      // Replace the element so it can be centered using class="center-block".
      replace: true,
      templateUrl: 'views/_quota-usage-chart.html',
      link: function($scope, element) {
        var usageValue = $filter('usageValue');
        var usageWithUnits = $filter('usageWithUnits');
        var amountAndUnit = $filter('amountAndUnit');
        function updateCenterText() {
          var donutChartTitle = d3.select(element[0]).select('text.c3-chart-arcs-title');
          if (!donutChartTitle) {
            Logger.warn("Can't select donut title element");
            return;
          }

          var replaceText = _.spread(function(amount, unit) {
            // Replace donut title content.
            donutChartTitle.selectAll('*').remove();
            donutChartTitle
              .insert('tspan')
              .text(amount)
              .classed('pod-count donut-title-big-pf', true)
              .attr('dy', 0)
              .attr('x', 0);
            donutChartTitle
              .insert('tspan')
              .text(unit)
              .classed('donut-title-small-pf', true)
              .attr('dy', 20)
              .attr('x', 0);
          });
          replaceText(amountAndUnit($scope.total, $scope.type, true));
        }

        // Adjust size based on legend position.
        if ($scope.legendPosition === 'right') {
          $scope.height = 175;
          $scope.width = 250;
        } else {
          $scope.height = 200;
          $scope.width = 175;
        }

        var percentage = function(value) {
          if (!value) {
            return "0%";
          }

          return (Number(value) * 100).toFixed(1) + "%";
        };

        // Chart configuration, see http://c3js.org/reference.html
        $scope.chartID = _.uniqueId('quota-usage-chart-');
        var config = {
          type: "donut",
          bindto: '#' + $scope.chartID,
          donut: {
            label: {
              show: false
            },
            width: 10
          },
          size: {
            height: $scope.height,
            width: $scope.width
          },
          legend: {
            show: true,
            position: $scope.legendPosition || 'bottom',
            item: {
              // Don't hide arcs when clicking the legend.
              onclick: _.noop
            }
          },
          onrendered: updateCenterText,
          tooltip: {
            position: function() {
              return { top: 0, left: 0 };
            },
            // Use custom tooltip HTML to avoid problems with content wrapping.
            // For example,
            //
            // <table class="c3-tooltip" style="width: 175px;">
            //   <tr>
            //     <td class="name nowrap">
            //       <span style="background-color: rgb(31, 119, 180);"></span>
            //       <span>Used</span>
            //     </td>
            //   </tr>
            //   <tr>
            //     <td class="value" style="text-align: left;">34% of 1 GiB</td>
            //   </tr>
            // </table>
            contents: function(d, defaultTitleFormat, defaultValueFormat, color) {
              var table = $('<table class="c3-tooltip"></table>')
                .css({ width: $scope.width + 'px' });

              var trName = $('<tr/>').appendTo(table);
              var tdName = $('<td class="name nowrap"></td>').appendTo(trName);

              // Color
              $('<span/>')
                .css({
                  'background-color': color(d[0].id)
                })
                .appendTo(tdName);

              // Name
              $('<span/>')
                .text(d[0].name)
                .appendTo(tdName);

              // Value
              var value;
              if (!$scope.total) {
                value = usageWithUnits($scope.used, $scope.type);
              } else {
                value = percentage(d[0].value / usageValue($scope.total)) + " of " + usageWithUnits($scope.total, $scope.type);
              }

              var trValue = $('<tr/>').appendTo(table);
              $('<td class="value" style="text-align: left;"></td>')
                .text(value)
                .appendTo(trValue);

              return table.get(0).outerHTML;
            }
          },
          data: {
            type: "donut",
            // Keep groups in our order.
            order: null
          }
        };

        var chart;
        var updateChart = function() {
          var used = usageValue($scope.used) || 0,
              available = Math.max(usageValue($scope.total) - used, 0),
              data = {
                columns: [
                  ['Used', used],
                  ['Available', available]
                ],
                // https://www.patternfly.org/styles/color-palette/
                colors: {
                  // Orange if at quota, blue otherwise
                  Used: available ? "#0088ce" : "#ec7a08",
                  // Gray
                  Available: "#d1d1d1"
                }
              };

          if (!chart) {
            _.assign(config.data, data);
            chart = c3.generate(config);
          } else {
            chart.load(data);
          }
        };
        $scope.$watchGroup(['used', 'total'], _.debounce(updateChart, 300));
      }
    };
  });
