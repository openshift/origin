'use strict';

angular.module('openshiftConsole')
  .directive('logViewer', [
    'DataService',
    'logLinks',
    function(DataService, logLinks) {
      return {
        restrict: 'AE',
        transclude: true,
        templateUrl: 'views/directives/logs/_log-viewer.html',
        scope: {
          logs: '=',
          loading: '=',
          links: '=',
          name: '=',
          download: '=',
          start: '=',
          end: '='
        },
        controller: [
          '$scope',
          function($scope) {
            angular.extend($scope, {
              ready: true,
              canDownload: logLinks.canDownload(),
              makeDownload: _.flow(function(arr) {
                                    return _.reduce(
                                              arr,
                                              function(memo, next, i) {
                                                return i <= arr.length ?
                                                        memo + next.text :
                                                        memo;
                                              }, '');
                                  }, logLinks.makeDownload),
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
