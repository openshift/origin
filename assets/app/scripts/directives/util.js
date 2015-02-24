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
  });