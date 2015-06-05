'use strict';

angular.module('openshiftConsole')
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
  .directive('copyToClipboardButton', function() {
    return {
      restrict: 'E',
      scope: {
        clipboardText: "="
      },
      templateUrl: 'views/directives/_copy-to-clipboard.html',
      link: function($scope, element) {
        if (ZeroClipboard.isFlashUnusable()) {
          $(element).hide();
        }
        else {
          new ZeroClipboard( $('button', element) );
          $("#global-zeroclipboard-html-bridge").tooltip({title: "Copy to clipboard", placement: 'bottom'});
        }
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
  });
