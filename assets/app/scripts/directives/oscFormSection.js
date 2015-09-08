"use strict";

angular.module("openshiftConsole")
  .directive("oscFormSection", function () {
    return {
      restrict: "E",
      transclude: true,
      scope: {
        header: "@",
        about: "@",
        aboutTitle: "@",
        editText: "@",
        expand: "=?",
        canToggle: "=?"
      },
      templateUrl: "views/directives/osc-form-section.html",
      link: function(scope, element, attrs) {
        if(!attrs.editText) {
           attrs.editText="Edit";
        }

        if (!angular.isDefined(attrs.canToggle)) {
          scope.canToggle = true;
        }

        scope.toggle = function(){
          scope.expand = !scope.expand;
        };
      }
    };
  });
