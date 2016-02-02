'use strict';
/*jshint -W030 */

angular.module('openshiftConsole')
  .directive('logViewer', [
    '$sce',
    '$timeout',
    '$window',
    'AuthService',
    'APIDiscovery',
    'DataService',
    'logLinks',
    'BREAKPOINTS',
    function($sce, $timeout, $window, AuthService, APIDiscovery, DataService, logLinks, BREAKPOINTS) {
      // cache the jQuery win, but not clobber angular's $window
      var $win = $(window);
      // Keep a reference the DOM node rather than the jQuery object for cloneNode.
      var logLineTemplate =
        $('<tr class="log-line">' +
          '<td class="log-line-number"></td>' +
          '<td class="log-line-text"></td>' +
          '</tr>').get(0);
      var buildLogLineNode = function(lineNumber, text) {
        var line = logLineTemplate.cloneNode(true);
        line.firstChild.appendChild(document.createTextNode(lineNumber));
        line.lastChild.appendChild(document.createTextNode(text));

        return line;
      };


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
          chromeless: '=?',
          run: '=?'  // will wait for run to be truthy before requesting logs
        },
        controller: [
          '$scope',
          function($scope) {
            // cached node's are set by the directive's postLink fn after render (see link: func below)
            var cachedLogNode;
            var cachedScrollableNode;
            var scrollableDOMNode;


            angular.extend($scope, {
              loading: true,
              autoScroll: false
            });


            var updateScrollLinks = function() {
              $scope.$apply(function() {
                // Show scroll links if the top or bottom of the log is off screen.
                var html = document.documentElement, r = cachedLogNode.getBoundingClientRect();
                $scope.showScrollLinks = r && ((r.top < 0) || (r.bottom > html.clientHeight));
              });
            };


            // Set to true before auto-scrolling.
            var autoScrollingNow = false;
            var onScroll = function() {
              // Determine if the user scrolled or we auto-scrolled.
              if (autoScrollingNow) {
                // Reset the value.
                autoScrollingNow = false;
              } else {
                // If the user scrolled the window manually, stop auto-scrolling.
                $scope.$evalAsync(function() {
                  $scope.autoScroll = false;
                });
              }
            };
            $win.scroll(onScroll);


            var onResize = _.debounce(updateScrollLinks, 50);
            $win.on('resize', onResize);


            var autoScrollBottom = function() {
              // Tell the scroll listener this is an auto-scroll. The listener
              // will reset it to false.
              autoScrollingNow = true;
              logLinks.scrollBottom(scrollableDOMNode);
            };

            var toggleAutoScroll = function() {
              $scope.autoScroll = !$scope.autoScroll;
              if ($scope.autoScroll) {
                // Scroll immediately. Don't wait the next message.
                autoScrollBottom();
              }
            };


            var buffer = document.createDocumentFragment();

            var update = _.debounce(function() {
              cachedLogNode.appendChild(buffer);
              buffer = document.createDocumentFragment();

              // Follow the bottom of the log if auto-scroll is on.
              if ($scope.autoScroll) {
                autoScrollBottom();
              }

              if (!$scope.showScrollLinks) {
                updateScrollLinks();
              }
            }, 100, { maxWait: 300 });


            // maintaining one streamer reference & ensuring its closed before we open a new,
            // since the user can (potentially) swap between multiple containers
            var streamer;
            var stopStreaming = function(keepContent) {
              if (streamer) {
                streamer.stop();
                streamer = null;
              }

              if (!keepContent) {
                // Cancel any pending updates. (No-op if none pending.)
                update.cancel();
                cachedLogNode && (cachedLogNode.innerHTML = '');
                buffer = document.createDocumentFragment();
              }
            };

            var streamLogs = function() {
              // Stop any active streamer.
              stopStreaming();

              if (!$scope.name) {
                return;
              }

              if(!$scope.run) {
                return;
              }

              angular.extend($scope, {
                loading: true,
                error: false,
                autoScroll: false,
                limitReached: false,
                showScrollLinks: false
              });

              var options = angular.extend({
                follow: true,
                tailLines: 1000,
                limitBytes: 10 * 1024 * 1024 // Limit log size to 10 MiB
              }, $scope.options);
              streamer =
                DataService.createStream($scope.kind, $scope.name, $scope.context, options);

              var lastLineNumber = 0;
              var addLine = function(text) {
                lastLineNumber++;
                // Append the line to the document fragment buffer.
                buffer.appendChild(buildLogLineNode(lastLineNumber, text));
                update();
              };

              streamer.onMessage(function(msg, raw, cumulativeBytes) {
                if (options.limitBytes && cumulativeBytes >= options.limitBytes) {
                  $scope.$evalAsync(function() {
                    $scope.limitReached = true;
                    $scope.loading = false;
                  });
                  stopStreaming(true);
                }

                addLine(msg);

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
                  $scope.autoScroll = false;
                });

                // Wrap in a timeout so that content displays before we remove the loading ellipses.
                $timeout(function() {
                  $scope.loading = false;
                }, 100);
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

            $scope.$watchGroup(['name', 'options.container', 'run'], streamLogs);

            $scope.$on('$destroy', function() {
              // Close streamer if open. (No-op if not streaming.)
              stopStreaming();

              // Stop listening for scroll and resize events.
              $win.off('scroll', onScroll);
              $win.off('resize', onResize);
            });


            angular.extend($scope, {
              ready: true,
              onScrollBottom: function() {
                logLinks.scrollBottom(scrollableDOMNode);
              },
              onScrollTop: function() {
                $scope.autoScroll = false;
                logLinks.scrollTop(scrollableDOMNode);
              },
              toggleAutoScroll: toggleAutoScroll,
              goChromeless: logLinks.chromelessLink,
              restartLogs: streamLogs
            });



            APIDiscovery
              .getLoggingURL()
              .then(function(url) {
                var projectName = _.get($scope.context, 'project.metadata.name');
                var containerName = _.get($scope.options, 'container');

                if(!(projectName && containerName && $scope.name && url)) {
                  return;
                }

                angular.extend($scope, {
                  kibanaAuthUrl: $sce.trustAsResourceUrl(URI(url)
                                                          .segment('auth').segment('token')
                                                          .normalizePathname().toString()),
                  access_token: AuthService.UserStore().getToken()
                });

                $scope.$watchGroup(['context.project.metadata.name', 'options.container', 'name'], function() {
                  angular.extend($scope, {
                    // The archive URL violates angular's built in same origin policy.
                    // Need to explicitly tell it to trust this location or it will throw errors.
                    archiveLocation: $sce.trustAsResourceUrl(logLinks.archiveUri({
                                        namespace: $scope.context.project.metadata.name,
                                        podname: $scope.name,
                                        containername: $scope.options.container,
                                        backlink: URI.encode($window.location.href)
                                      }))
                  });
                });
              });



              // scrollable node is window if mobile, else a particular DOM node.
              var detectScrollableNode = function() {
                if(window.innerWidth < BREAKPOINTS.screenSmMin) {
                  scrollableDOMNode = null;
                } else {
                  scrollableDOMNode = cachedScrollableNode;
                }
              };

              var debounceScrollable = _.debounce(detectScrollableNode, 200);

              // API to share w/link fn
              this.cacheScollableNode = function(node) {
                cachedScrollableNode = node;
                detectScrollableNode();
              };

              this.cacheLogNode = function(node) {
                cachedLogNode = node;
              };
              // maintain the correct scrollable node
              $win.on('resize', debounceScrollable);
              $scope.$on('$destroy', function() {
                $win.off('resize', debounceScrollable);
              });
          }
        ],
        require: 'logViewer',
        link: function($scope, $elem, $attrs, ctrl) {
          // TODO:
          // unfortuntely this directive has to search for a parent elem to use as scrollable :(
          // would be better if 'scrollable' was a directive on a parent div
          // and we were sending it messages telling it when to scroll.
          ctrl.cacheScollableNode(document.getElementById('scrollable-content'));
          ctrl.cacheLogNode(document.getElementById('logContent'));
        }
      };
    }
  ]);
