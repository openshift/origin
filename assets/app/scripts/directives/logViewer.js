'use strict';

angular.module('openshiftConsole')
  .directive('logViewer', [
    'DataService',
    'logLinks',
    'logUtils',
    function(DataService, logLinks, logUtils) {
      return {
        restrict: 'AE',
        templateUrl: '/views/directives/logs/_log-viewer.html',
        scope: {
          logs: '=',
          loading: '=',
          links: '=',
          download: '='
        },
        controller: [
          '$scope',
          function($scope) {
            angular.extend($scope, {
              ready: true,
              canDownload: logLinks.canDownload(),
              makeDownload: _.flow(logUtils.toStr, logLinks.makeDownload),
              scrollTo: logLinks.scrollTo,
              scrollTop: logLinks.scrollTop,
              scrollBottom: logLinks.scrollBottom,
              goFull: logLinks.fullPageLink,
              goChromeless: logLinks.chromelessLink,
              goText: logLinks.textOnlyLink
            });
          }
        ]
      };
    }
  ]);
