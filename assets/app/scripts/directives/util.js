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
  });