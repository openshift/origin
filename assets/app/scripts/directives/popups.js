'use strict';

angular.module('openshiftConsole')
  // This triggers when an element has either a toggle or data-toggle attribute set on it
  .directive('toggle', function() {
    return {
      restrict: 'A',
      link: function($scope, element, attrs) {
        if (attrs) {
          switch(attrs.toggle) {
            case "popover":
              $(element).popover();
              break;
            case "tooltip":
              $(element).tooltip();
              break;
          }

        }
      }
    };
  });
