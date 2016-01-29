'use strict';

angular.module('openshiftConsole')
  // Truncates text to a length, adding a tooltip and an ellipsis if truncated.
  // Different than `text-overflow: ellipsis` because it allows for multiline text.
  .directive('truncateLongText', function(truncateFilter) {
    return {
      restrict: 'E',
      scope: {
        content: '=',
        limit: '=',
        newlineLimit: '=',
        useWordBoundary: '=',
        expandable: '=',
        prettifyJson: '=' // prettifies JSON blobs when expanded, only used if expandable is true
      },
      templateUrl: 'views/directives/truncate-long-text.html',
      link: function(scope) {
        scope.toggles = {expanded: false};
        scope.$watch('content', function(content) {
          if (content) {
            scope.truncatedContent = truncateFilter(content, scope.limit, scope.useWordBoundary, scope.newlineLimit);
            scope.truncated = scope.truncatedContent.length !== content.length;
          }
          else {
            scope.truncatedContent = null;
            scope.truncated = false;
          }
        });
      }
    };
  });
