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
        // Set the line number as a data attribute and display it using the
        // ::before pseudo-element in CSS so it isn't copied. Works around
        // this webkit bug with user-select: none;
        //   https://bugs.webkit.org/show_bug.cgi?id=80159
        line.firstChild.setAttribute('data-line-number', lineNumber);

        // Escape ANSI color codes
        var escaped = ansi_up.escape_for_html(text);
        var html = ansi_up.ansi_to_html(escaped);
        var linkifiedHTML = ansi_up.linkify(html);
        line.lastChild.innerHTML = linkifiedHTML;

        return line;
      };


      return {
        restrict: 'AE',
        transclude: true,
        templateUrl: 'views/directives/logs/_log-viewer.html',
        scope: {
          followAffixTop: '=?',
          followAffixBottom: '=?',
          resource: '@',
          name: '=',
          context: '=',
          options: '=?',
          chromeless: '=?',
          empty: '=?',        // boolean, let the parent know when the log is empty
          run: '=?'           // boolean, logs will not run until this is truthy
        },
        controller: [
          '$scope',
          function($scope) {
            // cached node's are set by the directive's postLink fn after render (see link: func below)
            // A jQuery wrapped verison is cached in var of same name w/$
            var cachedLogNode;
            var cachedScrollableNode;
            var $cachedScrollableNode;
            var scrollableDOMNode;
            var $affixableNode;
            var html = document.documentElement;

            $scope.empty = true;

            // are we going to scroll the window, or the DOM node?
            var detectScrollableNode = function() {
              if(window.innerWidth < BREAKPOINTS.screenSmMin) {
                scrollableDOMNode = null;
              } else {
                scrollableDOMNode = cachedScrollableNode;
              }
            };



            // is just toggling show/hide, nothing else.
            var updateScrollLinksVisibility = function() {
              $scope.$apply(function() {
                // Show scroll links if the top or bottom of the log is off screen.
                var r = cachedLogNode.getBoundingClientRect();
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
                  $scope.autoScrollActive = false;
                });
              }
            };


            var attachScrollEvents = function() {
              // always clear all scroll listeners before reattaching
              $cachedScrollableNode.off('scroll', onScroll);
              $win.off('scroll', onScroll);

              // only add the appropriate event
              if(window.innerWidth <= BREAKPOINTS.screenSmMin) {
                $win.on('scroll', onScroll);
              } else {
                $cachedScrollableNode.on('scroll', onScroll);
              }
            };


            // the class .target-logger-node is needed to adjust some
            // css when the target is not the window.
            // TODO: resize event breaks the affix, even with this if/else.
            // however, on first load of either mobile or non this works fine.
            var affix = function() {
              if(window.innerWidth < BREAKPOINTS.screenSmMin) {
                $affixableNode
                  .removeClass('target-logger-node')
                  .affix({
                    target:  window,
                    offset: {
                        top:  $scope.followAffixTop || 0, // 390,
                        bottom: $scope.followAffixBottom || 0 // 90
                    }
                  });
              } else {
                $affixableNode
                  .addClass('target-logger-node')
                  .affix({
                    target:  $cachedScrollableNode,
                    offset: {
                        top: $scope.followAffixTop || 0, // 390,
                        bottom: $scope.followAffixBottom || 0 // 90
                    }
                  });
              }
            };

            var fillHeight = function(animate) {
              var content = $('.log-view-output');
              var contentTop = content.offset().top;
              if (contentTop < 0) {
                // Content top is off the page already.
                return;
              }

              var fill = Math.floor($(window).height() - contentTop);
              if (!$scope.chromeless) {
                // Add some bottom margin if not chromeless.
                fill = fill - 35;
              }
              if (animate) {
                content.animate({ 'min-height': fill +'px' }, 'fast');
              } else {
                content.css('min-height', fill + 'px');
              }
            };

            // roll up & debounce the various fns to call on resize
            var onResize = _.debounce(function() {
              fillHeight(true);
              // update scroll handlers
              detectScrollableNode();
              attachScrollEvents();
              updateScrollLinksVisibility();  // toggles show/hide
              affix();
              // toggle off the follow behavior if the user resizes the window
              onScroll();
            }, 100);

            $win.on('resize', onResize);



            // STREAMER & DOM NODE HANDLING ------------------------------------


            var autoScrollBottom = function() {
              // Tell the scroll listener this is an auto-scroll. The listener
              // will reset it to false.
              autoScrollingNow = true;
              logLinks.scrollBottom(scrollableDOMNode);
            };


            var toggleAutoScroll = function() {
              $scope.autoScrollActive = !$scope.autoScrollActive;
              if ($scope.autoScrollActive) {
                // Scroll immediately. Don't wait the next message.
                autoScrollBottom();
              }
            };

            var buffer = document.createDocumentFragment();

            var update = _.debounce(function() {
              cachedLogNode.appendChild(buffer);
              buffer = document.createDocumentFragment();

              // Follow the bottom of the log if auto-scroll is on.
              if ($scope.autoScrollActive) {
                autoScrollBottom();
              }

              if (!$scope.showScrollLinks) {
                updateScrollLinksVisibility(); // toggles show/hide
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
                autoScroll: false,
                limitReached: false,
                showScrollLinks: false
              });

              var options = angular.extend({
                follow: true,
                tailLines: 1000,
                limitBytes: 10 * 1024 * 1024 // Limit log size to 10 MiB
              }, $scope.options);

              streamer = DataService.createStream($scope.resource, $scope.name, $scope.context, options);

              var lastLineNumber = 0;
              var addLine = function(text) {
                lastLineNumber++;
                // Append the line to the document fragment buffer.
                buffer.appendChild(buildLogLineNode(lastLineNumber, text));
                update();
              };

              streamer.onMessage(function(msg, raw, cumulativeBytes) {
                // ensures the digest loop will catch the state change.
                $scope.$evalAsync(function() {
                  $scope.empty = false;
                  if($scope.state !== 'logs') {
                    $scope.state = 'logs';
                    // setTimeout so that the log content is visible to correctly calculate fill height.
                    setTimeout(fillHeight);
                  }
                });

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
                  $scope.autoScrollActive = false;
                  // - if no logs, they have already been archived.
                  // - if emptyStateMessage has already been set, it means the onError
                  //   callback has already fired.  onError message takes priority in severity.
                  // - at present we are using the same error message in both onError and onClose
                  //   because we dont have enough information to give the user something better.
                  if((lastLineNumber === 0) && (!$scope.emptyStateMessage)) {
                    $scope.state = 'empty';
                    $scope.emptyStateMessage = 'The logs are no longer available or could not be loaded.';
                  }
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
                    autoScroll: false
                  });
                  // if logs err before we get anything, will show an empty state message
                  if(lastLineNumber === 0) {
                    $scope.state = 'empty';
                    $scope.emptyStateMessage = 'The logs are no longer available or could not be loaded.';
                  } else {
                    // if logs were running but something went wrong, will
                    // show what we have & give option to retry
                    $scope.errorWhileRunning = true;
                  }
                });
              });

              streamer.start();
            };





            // initial $scope setup --------------------------------------------

            angular.extend($scope, {
              ready: true,
              loading: true,
              autoScroll: false,
              state: false, // show nothing initially to avoid flicker
              onScrollBottom: function() {
                logLinks.scrollBottom(scrollableDOMNode);
              },
              onScrollTop: function() {
                $scope.autoScrollActive = false;
                logLinks.scrollTop(scrollableDOMNode);
              },
              toggleAutoScroll: toggleAutoScroll,
              goChromeless: logLinks.chromelessLink,
              restartLogs: streamLogs
            });

            $scope.$watchGroup(['name', 'options.container', 'run'], streamLogs);




            // tear down -------------------------------------------------------

            $scope.$on('$destroy', function() {
              // close streamer or no-op
              stopStreaming();
              // clean up all the listeners
              $win.off('resize', onResize);
              $win.off('scroll', onScroll);
              $cachedScrollableNode.off('scroll', onScroll);
            });




            // Kibana archives -------------------------------------------------

            APIDiscovery
              .getLoggingURL()
              .then(function(url) {
                var projectName = _.get($scope.context, 'project.metadata.name');
                var containerName = _.get($scope.options, 'container');

                if(!(projectName && containerName && $scope.name && url)) {
                  return;
                }

                // 3 things needed:
                // - kibanaAuthUrl to authorize user
                // - access_token
                // - kibanaArchiveUrl for the final destination once auth'd
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
                    kibanaArchiveUrl: $sce.trustAsResourceUrl(logLinks.archiveUri({
                                        namespace: $scope.context.project.metadata.name,
                                        podname: $scope.name,
                                        containername: $scope.options.container,
                                        backlink: URI.encode($window.location.href)
                                      }))
                  });
                });
              });




              // PUBLIC API ----------------------------------------------------

              // scrollable node is a parent div#scrollable-content, but may be window
              // if we are currently mobile
              this.cacheScollableNode = function(node) {
                cachedScrollableNode = node;
                $cachedScrollableNode = $(cachedScrollableNode);
              };

              this.cacheLogNode = function(node) {
                cachedLogNode = node; // no jQuery, optimized
              };

              this.cacheAffixable = function(node) {
                $affixableNode = $(node); // jQuery is fine
              };

              this.start = function() {
                detectScrollableNode();
                attachScrollEvents();
                affix();
              };
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
          ctrl.cacheAffixable(document.getElementById('affixedFollow'));
          ctrl.start();
        }
      };
    }
  ]);
