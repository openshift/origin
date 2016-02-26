'use strict';

angular.module('openshiftConsole')
  // The HTML5 `autofocus` attribute does not work reliably with Angular,
  // so define our own directive
  .directive('takeFocus', function($timeout) {
    return {
      restrict: 'A',
      link: function(scope, element) {
        // Add a delay to allow other asynchronous components to load.
        $timeout(function() {
          $(element).focus();
        }, 300);
      }
    };
  })
  .directive('selectOnFocus', function() {
    return {
      restrict: 'A',
      link: function($scope, element) {
        $(element).focus(function () {
          $(this).select();
        });
      }
    };
  })
  .directive('focusWhen', function($timeout) {
    return {
      restrict: 'A',
      scope: {
        trigger: '@focusWhen'
      },
      link: function(scope, element) {
        scope.$watch('trigger', function(value) {
          if (value) {
            $timeout(function() {
              $(element).focus();
            });
          }
        });
      }
    };
  })
  .directive('tileClick', function() {
    return {
      restrict: 'AC',
      link: function($scope, element) {
        $(element).click(function (evt) {
          var t = $(evt.target);
          if (t && t.is('a')){
            return;
          }
          $('a.tile-target', element).trigger("click");
        });
      }
    };
  })
  .directive('clickToReveal', function() {
    return {
      restrict: 'A',
      transclude: true,
      scope: {
        linkText: "@"
      },
      templateUrl: 'views/directives/_click-to-reveal.html',
      link: function($scope, element) {
        $('.reveal-contents-link', element).click(function () {
          $(this).hide();
          $('.reveal-contents', element).show();
        });
      }
    };
  })
  .directive('copyToClipboardButton', function(IS_IOS) {
    return {
      restrict: 'E',
      scope: {
        clipboardText: "="
      },
      templateUrl: 'views/directives/_copy-to-clipboard.html',
      controller: function($scope) {
        $scope.id = _.uniqueId('clipboardJs');
      },
      link: function($scope, element) {
        if (IS_IOS) {
          $scope.hidden = true;
          return;
        }

        var node = $('button', element);
        var clipboard = new Clipboard( node.get(0) );
        clipboard.on('success', function (e) {
          $(e.trigger)
            .attr('title', 'Copied!')
            .tooltip('fixTitle')
            .tooltip('show')
            .attr('title', 'Copy to clipboard')
            .tooltip('fixTitle');
          e.clearSelection();
        });
        clipboard.on('error', function (e) {
          var fallbackMsg = /Mac/i.test(navigator.userAgent) ? 'Press \u2318C to copy' : 'Press Ctrl-C to copy';
          $(e.trigger)
            .attr('title', fallbackMsg)
            .tooltip('fixTitle')
            .tooltip('show')
            .attr('title', 'Copy to clipboard')
            .tooltip('fixTitle');
        });
        element.on('$destroy', function() {
          clipboard.destroy();
        });
      }
    };
  })
  .directive('shortId', function() {
    return {
      restrict:'E',
      scope: {
        id: '@'
      },
      template: '<code class="short-id" title="{{id}}">{{id.substring(0, 6)}}</code>'
    };
  })
  .directive('customIcon', function() {
    return {
      restrict:'E',
      scope: {
        resource: '=',
        kind: '@',
        tag: '=?'
      },
      controller: function($scope, $filter) {
        if ($scope.tag) {
          $scope.icon = $filter('imageStreamTagAnnotation')($scope.resource, "icon", $scope.tag);
        } else {
          $scope.icon = $filter('annotation')($scope.resource, "icon");
        }
        $scope.isDataIcon = $scope.icon && ($scope.icon.indexOf("data:") === 0);
        if (!$scope.isDataIcon) {
          // The icon class filter will at worst return the default icon for the given kind
          if ($scope.tag) {
            $scope.icon = $filter('imageStreamTagIconClass')($scope.resource, $scope.tag);
          } else {
            $scope.icon = $filter('iconClass')($scope.resource, $scope.kind);
          }
        }
      },
      templateUrl: 'views/directives/_custom-icon.html'
    };
  })
  .directive('bottomOfWindow', function() {
    return {
      restrict:'A',
      link: function(scope, element) {
        function resized() {
          var height = $(window).height() - element[0].getBoundingClientRect().top;
          element.css('height', (height - 10) + "px");
        }

        $(window).on("resize", resized);
        resized();

        element.on("$destroy", function() {
          $(window).off("resize", resized);
        });
      }
    };
  }).directive('onEnter', function(){
    return function (scope, element, attrs) {
      element.bind("keydown keypress", function (event) {
        if(event.which === 13 && !scope.form.$invalid) {
          scope.$apply(function (){
            scope.$eval(attrs.onEnter);
          });
          event.preventDefault();
        }
      });
    };
  });
