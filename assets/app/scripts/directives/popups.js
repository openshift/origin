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
                  $(element).popover("destroy");
                  // Destroy is asynchronous. Wait for it to complete before updating content.
                  // See https://github.com/twbs/bootstrap/issues/16376
                  //     https://github.com/twbs/bootstrap/issues/15607
                  //     http://stackoverflow.com/questions/27238938/bootstrap-popover-destroy-recreate-works-only-every-second-time
                  // Destroy calls hide, which takes 150ms to complete.
                  //     https://github.com/twbs/bootstrap/blob/87121181c8a4b63192865587381d4b8ada8de30c/js/tooltip.js#L31
                  setTimeout(function() {
                    $(element)
                      .attr("data-content", $scope.dynamicContent)
                      .popover();
                  }, 200);
                });
              }
              $(element).popover();
              break;
            case "tooltip":
              // If dynamic-content attr is set at all, even if it hasn't evaluated to a value
              if (attrs.dynamicContent || attrs.dynamicContent === "") {
                $scope.$watch('dynamicContent', function() {
                  $(element).tooltip("destroy");
                  // Destroy is asynchronous. Wait for it to complete before updating content.
                  // See https://github.com/twbs/bootstrap/issues/16376
                  //     https://github.com/twbs/bootstrap/issues/15607
                  //     http://stackoverflow.com/questions/27238938/bootstrap-popover-destroy-recreate-works-only-every-second-time
                  // Destroy calls hide, which takes 150ms to complete.
                  //     https://github.com/twbs/bootstrap/blob/87121181c8a4b63192865587381d4b8ada8de30c/js/tooltip.js#L31
                  setTimeout(function() {
                    $(element)
                      .attr("title", $scope.dynamicContent)
                      .tooltip();
                  }, 200);
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
      link: function($scope) {
        var i, content = '', warnings = podWarningsFilter($scope.pod);
        for (i = 0; i < warnings.length; i++) {
          if (content) {
            content += '<br>';
          }
          content += warnings[i].message;
        }
        $scope.content = content;
      },
      templateUrl: 'views/directives/_warnings-popover.html'
    };
  })
  .directive('routeWarnings', function(RoutesService) {
    return {
      restrict: 'E',
      scope: {
        route: '=',
        service: '='
      },
      link: function($scope) {
        var updateWarnings = function() {
          var warnings = RoutesService.getRouteWarnings($scope.route, $scope.service);
          $scope.content = warnings.join('<br>');
        };
        $scope.$watch('route', updateWarnings, true);
        $scope.$watch('service', updateWarnings, true);
      },
      templateUrl: 'views/directives/_warnings-popover.html'
    };
  });
