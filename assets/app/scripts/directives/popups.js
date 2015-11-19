'use strict';

angular.module('openshiftConsole')
  // This triggers when an element has either a toggle or data-toggle attribute set on it
  .directive('toggle', function() {
    return {
      restrict: 'A',
      scope: {
        dynamicContent: '@?'
      },
      link: function($scope, element, attrs) {
        if (attrs) {
          switch(attrs.toggle) {
            case "popover":
              // If dynamic-content attr is set at all, even if it hasn't evaluated to a value
              if (attrs.dynamicContent || attrs.dynamicContent === "") {
                $scope.$watch('dynamicContent', function() {
                  $(element)
                    .attr("data-content", $scope.dynamicContent)
                    .popover("destroy")
                    .popover();                
                });                  
              }
              $(element).popover();
              break;
            case "tooltip":
              // If dynamic-content attr is set at all, even if it hasn't evaluated to a value
              if (attrs.dynamicContent || attrs.dynamicContent === "") {
                $scope.$watch('dynamicContent', function() {
                  $(element)
                    .attr("title", $scope.dynamicContent)
                    .tooltip("destroy")
                    .tooltip();                
                });                  
              }
              $(element).tooltip();
              break;
            case "dropdown":
              if (attrs.hover === "dropdown") {
                $(element).dropdownHover({delay: 200});
                $(element).dropdown();
              }
              break;
          }
        }
      }
    };
  })
  .directive('podWarnings', function(podWarningsFilter) {
    return {
      restrict:'E',
      scope: {
        pod: '='
      },
      link: function($scope, element) {
        var warnings = podWarningsFilter($scope.pod);
        var content = "";
        angular.forEach(warnings, function(warning) {
          content += warning.message + "<br>";
        });       
        $('.pficon-warning-triangle-o', element)
          .attr("data-content", content)
          .popover("destroy")
          .popover();
      },
      templateUrl: 'views/directives/_pod-warnings.html'
    };
  });
