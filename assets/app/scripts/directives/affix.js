'use strict';
/* jshint unused: false */

angular.module('openshiftConsole')
  .directive('affix', function($window) {
    return {
      restrict: 'AE',
      scope: {
        offsetTop: '@',
        offsetBottom: '@'
      },
      link: function($scope, $elem, $attrs, ctrl) {
        // for debugging
        // angular
        // .element($window)
        // .bind("scroll", function() {
        //   var pos = $elem.affix('checkPosition')[0];
        //     console.log('affix position', pos.offsetWidth, pos.offsetHeight, pos.offsetTop, pos.offsetBottom);
        //     //$scope.$apply();
        // });
        $elem.affix({
          offset: {
            top: $attrs.offsetTop,
            bottom: $attrs.offsetBottom
          }
        });
      }
    };
  });
