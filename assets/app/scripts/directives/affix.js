'use strict';

angular.module('openshiftConsole')
  .directive('affix', function() {
    return {
      restrict: 'AE',
      scope: {
        offsetTop: '=',
        offsetBottom: '='
      },
      controller: [
        function() {
        //'$window',
        //function($window) {
          this.init = function(/* affixedEl */) {
            // FOR DEBUGGING
            // angular
            // .element($window)
            // .bind("scroll", function() {
            //   var pos = affixedEl.affix('checkPosition')[0];
            //     console.log('affix position', pos.offsetWidth, pos.offsetHeight, pos.offsetTop, pos.offsetBottom);
            //     //scope.$apply();
            // });
          };
        }
      ],
      link: function($scope, $elem, $attrs, ctrl) {
        ctrl.init(
            $elem.affix({
              offset: {
                top: $attrs.offsetTop,
                bottom: $attrs.offsetBottom
              }
            }));
      }
    };
  });

