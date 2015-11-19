"use strict";

angular.module("openshiftConsole")
  .directive("oscImageSummary", function() {
    return {
      restrict: "E",
      scope: {
        resource: "=",
        name: "="
      },
      templateUrl: "views/directives/osc-image-summary.html"
    };
});