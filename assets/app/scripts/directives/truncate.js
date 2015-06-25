'use strict';

angular.module('openshiftConsole')
  // Truncates text to a length, adding a tooltip and an ellipsis if truncated.
  // Different than `text-overflow: ellipsis` because it allows for multiline text.
  .directive('truncateLongText', function() {
    return {
      restrict: 'E',
      scope: {
        content: '=',
        limit: '=',
        useWordBoundary: '='
      },
      template: '<span ng-attr-title="{{content}}">{{visibleContent}}<span ng-if="truncated">&hellip;</span></span>',
      link: function(scope, elem, attr) {
        scope.visibleContent = scope.content;
        scope.$watch('content', function(content) {
          if (!scope.limit || !content || content.length <= scope.limit) {
            scope.truncated = false;
            scope.visibleContent = content;
            return;
          }

          scope.truncated = true;
          scope.visibleContent = content.substring(0, scope.limit);
          if (scope.useWordBoundary !== false) {
            // Find the last word break, but don't look more than 10 characters back.
            // Make sure we show at least the first 5 characters.
            var startIndex = Math.max(4, scope.limit - 10);
            var lastSpace = scope.visibleContent.lastIndexOf(' ', startIndex);
            if (lastSpace !== -1) {
              scope.visibleContent = scope.visibleContent.substring(0, lastSpace);
            }
          }
        });
      }
    };
  });
