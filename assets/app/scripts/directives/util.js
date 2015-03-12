angular.module('openshiftConsole')
  .directive('selectOnFocus', function() {
    return {
      restrict: 'A', 
      link: function($scope, element, attrs) {
        $(element).focus(function () {
          $(this).select();
        });
      }
    };
  })
  .directive('tileClick', function() {
    return {
      restrict: 'AC', 
      link: function($scope, element, attrs) {
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
      link: function($scope, element, attrs) {
        $('.reveal-contents-link', element).click(function (evt) {
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
      link: function($scope, element, attrs) {
        if (ZeroClipboard.isFlashUnusable()) {
          $(element).hide();
        }
        else {
          new ZeroClipboard( $('button', element) );
          $("#global-zeroclipboard-html-bridge").tooltip({title: "Copy to clipboard", placement: 'bottom'});
        }
      }
    }
  });