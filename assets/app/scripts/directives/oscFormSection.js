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
        expand: "@"
      },
      templateUrl: "views/directives/osc-form-section.html",
      link: function(scope, element, attrs){
        if(!attrs.editText) {
           attrs.editText="Edit";
        }
        scope.expand = attrs.expand ? true : false;
        scope.toggle = function(){
          scope.expand = !scope.expand;
        };
      }
    };
  });
