'use strict';

angular.module('openshiftConsole')
  .directive('labels', function($location, $timeout, LabelFilter) {
    return {
      restrict: 'E',
      scope: {
        labels: '=',
        // if you specify clickable, then everything below is required unless specified as optional
        clickable: "@?",
        kind: "@?",
        projectName: "@?",
        limit: '=?',
        titleKind: '@?',   // optional, instead of putting kind into that part of the hover
                           // title, it will put this string instead, e.g. if you want 'builds for build config foo'
        navigateUrl: '@?'  // optional to override the default
      },
      templateUrl: 'views/directives/labels.html',
      link: function(scope) {
        scope.filterAndNavigate = function(key, value) {
          if (scope.kind && scope.projectName) {
            $location.url(scope.navigateUrl || ("/project/" + scope.projectName + "/browse/" + scope.kind));
            $timeout(function() {
              var selector = {};
              selector[key] = value;
              LabelFilter.setLabelSelector(new LabelSelector(selector, true));
            }, 1);            
          }
        };
      }
    };
  })
  .directive('labelEditor', function() {
    return {
      restrict: 'E',
      scope: {
        labels: "=",
        expand: "=?",
        canToggle: "=?"
      },
      templateUrl: 'views/directives/label-editor.html',
      link: function(scope, element, attrs) {
        if (!angular.isDefined(attrs.canToggle)) {
          scope.canToggle = true;
        }
      }
    };
  });
