"use strict";

angular.module('openshiftConsole')
  .directive('editCommand', function() {
    return {
      restrict: 'E',
      scope: {
        args: '=',
        isRequired: '='
      },
      templateUrl: 'views/directives/_edit-command.html',
      link: function(scope) {
        scope.id = _.uniqueId('edit-command-');
        scope.input = {};

        var inputChanged;
        scope.$watch('args', function() {
          if (inputChanged) {
            inputChanged = false;
            return;
          }

          if (!_.isEmpty(scope.args)) {
            // Convert the array of string to an array of objects internally to
            // avoid problems dragging/dropping duplicate values, which
            // ng-sortable doesn't handle well.
            scope.input.args = _.map(scope.args, function(arg) {
              return { value: arg };
            });
          }
        }, true);

        scope.$watch('input.args', function(newValue, oldValue) {
          if (newValue === oldValue) {
            return;
          }

          inputChanged = true;
          scope.args = _.map(scope.input.args, function(arg) {
            return arg.value;
          });
          scope.form.command.$setDirty();
        }, true);

        scope.addArg = function() {
          if (!scope.nextArg) {
            return;
          }

          scope.input.args = scope.input.args || [];
          scope.input.args.push({ value: scope.nextArg });
          scope.nextArg = '';
        };

        scope.removeArg = function(index) {
          scope.input.args.splice(index, 1);
          if (_.isEmpty(scope.input.args)) {
            // Needs to be null rather than empty for validation.
            scope.input.args = null;
          }
        };

        scope.clear = function() {
          scope.input.args = null;
        };
      }
    };
  });
