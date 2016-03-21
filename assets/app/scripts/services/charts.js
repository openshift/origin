'use strict';

angular.module("openshiftConsole")
  .factory("ChartsService", function(Logger) {
    return {
      updateDonutCenterText: function(element, titleBig, titleSmall) {
        var donutChartTitle = d3.select(element).select('text.c3-chart-arcs-title');
        if (!donutChartTitle) {
          Logger.warn("Can't select donut title element");
          return;
        }

        // Replace donut title content.
        donutChartTitle.selectAll('*').remove();
        donutChartTitle
          .insert('tspan')
          .text(titleBig)
          .classed('donut-title-big-pf', true)
          .attr('dy', 0)
          .attr('x', 0);
        donutChartTitle
          .insert('tspan')
          .text(titleSmall)
          .classed('donut-title-small-pf', true)
          .attr('dy', 20)
          .attr('x', 0);
      }
    };
  });

