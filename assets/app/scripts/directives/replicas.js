'use strict';

angular.module('openshiftConsole')
  .directive('replicas', function() {
    return {
      restrict: 'E',
      scope: {
        status: "=?",
        spec: "=",
        disableScaling: "=?",
        scaleFn: "&?"
      },
      templateUrl: 'views/directives/replicas.html',
      link: function(scope) {
        scope.model = {
          editing: false
        };

        scope.scale = function() {
          if (scope.form.scaling.$valid) {
            scope.scaleFn({ replicas: scope.model.desired });
            scope.model.editing = false;
          }
        };

        scope.cancel = function() {
          scope.model.editing = false;
        };
      }
    };
  });
