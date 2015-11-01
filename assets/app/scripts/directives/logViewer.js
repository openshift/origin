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
          kind: '@',
          name: '=',
          context: '=',
          options: '=?',
          status: '=?',
          start: '=?',
          end: '=?',
          chromeless: '=?'
        },
        controller: [
          '$scope',
          function($scope) {
            $scope.loading = true;

            // Default to false. Let the user click the follow link to start auto-scrolling.
            $scope.autoScroll = false;

            // Set to true when we auto-scroll to follow log content.
            var autoScrolling = false;
            var onScroll = function() {
              // Determine if the user scrolled or we auto-scrolled.
              if (autoScrolling) {
                // Reset the value.
                autoScrolling = false;
              } else {
                // If the user scrolled the window manually, stop auto-scrolling.
                $scope.$evalAsync(function() {
                  $scope.autoScroll = false;
                });
              }
            };
            $(window).scroll(onScroll);

            var scrollBottom = function() {
              // Tell the scroll listener this is an auto-scroll. The listener
              // will reset it to false.
              autoScrolling = true;
              logLinks.scrollBottom();
            };

            var toggleAutoScroll = function() {
              $scope.autoScroll = !$scope.autoScroll;
              if ($scope.autoScroll) {
                // Scroll immediately. Don't wait the next message.
                scrollBottom();
              }
            };

            var scrollTop = function() {
              // Stop auto-scrolling when the user clicks the scroll top link.
              $scope.autoScroll = false;
              logLinks.scrollTop();
            };

            // maintaining one streamer reference & ensuring its closed before we open a new,
            // since the user can (potentially) swap between multiple containers
            var streamer;
            var stopStreaming = function(keepContent) {
              if (streamer) {
                streamer.stop();
                streamer = null;
              }
              if (!keepContent) {
                $('#logContent').empty();
              }
            };

            var streamLogs = function() {
              // Stop any active streamer.
              stopStreaming();

              if (!$scope.name) {
                return;
              }

              $scope.$evalAsync(function() {
                angular.extend($scope, {
                  loading: true,
                  error: false,
                  autoScroll: false,
                  limitReached: false
                });
              });

              var options = angular.extend({
                follow: true,
                tailLines: 1000,
                limitBytes: 10 * 1024 * 1024 // Limit log size to 10 MiB
              }, $scope.options);
              streamer =
                DataService.createStream($scope.kind, $scope.name, $scope.context, options);

              var lastLineNumber = 0;
              streamer.onMessage(function(msg, raw, cumulativeBytes) {
                if (options.limitBytes && cumulativeBytes >= options.limitBytes) {
                  $scope.$evalAsync(function() {
                    $scope.limitReached = true;
                    $scope.loading = false;
                  });
                  stopStreaming(true);
                }

                lastLineNumber++;

                // Manipulate the DOM directly for better performance displaying large log files.
                var logLine = $('<div row class="log-line"/>');
                $('<div class="log-line-number"><div row flex main-axis="end">' + lastLineNumber + '</div></div>').appendTo(logLine);
                $('<div flex class="log-line-text"/>').text(msg).appendTo(logLine);
                logLine.appendTo('#logContent');

                // Follow the bottom of the log if auto-scroll is on.
                if ($scope.autoScroll) {
                  scrollBottom();
                }

                // Show the start and end links if the log is more than 25 lines.
                if (!$scope.showScrollLinks && lastLineNumber > 25) {
                  $scope.$evalAsync(function() {
                    $scope.showScrollLinks = true;
                  });
                }

                // Warn the user if we might be showing a partial log.
                if (!$scope.largeLog && lastLineNumber >= options.tailLines) {
                  $scope.$evalAsync(function() {
                    $scope.largeLog = true;
                  });
                }
              });

              streamer.onClose(function() {
                streamer = null;
                $scope.$evalAsync(function() {
                  angular.extend($scope, {
                    loading: false,
                    autoScroll: false
                  });
                });
              });

              streamer.onError(function() {
                streamer = null;
                $scope.$evalAsync(function() {
                  angular.extend($scope, {
                    loading: false,
                    error: true,
                    autoScroll: false
                  });
                });
              });

              streamer.start();
            };

            $scope.$watchGroup(['name', 'options.container'], streamLogs);

            $scope.$on('$destroy', function() {
              stopStreaming();
              $(window).off('scroll', onScroll);
            });

            angular.extend($scope, {
              ready: true,
              scrollBottom: logLinks.scrollBottom,
              scrollTop: scrollTop,
              toggleAutoScroll: toggleAutoScroll,
              goChromeless: logLinks.chromelessLink,
              restartLogs: streamLogs
            });
          }
        ]
      };
    }
  ]);
