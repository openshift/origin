'use strict';
// oscUnique is a validation directive
// use:
// Put it on an input or other DOM node with an ng-model attribute.
// Pass a list (array, or object) via osc-unique="list"
//
// Sets model $valid true||false
// - model is valid so long as the item is not already in the list
//
// Key off $valid to enable/disable/sow/etc other objects
//
// Validates that the ng-model is unique in a list of values.
// ng-model: 'foo'
// oscUnique: ['foo', 'bar', 'baz']       // false, the string 'foo' is in the list
// oscUnique: [1,2,4]                     // true, the string 'foo' is not in the list
// oscUnique: {foo: true, bar: false}     // false, the object has key 'foo'
// NOTES:
// - non-array values passed to oscUnqiue will be transformed into an array.
//   - oscUnqiue: 'foo' => [0,1,2]  (probably not what you want, so don't pass a string)
// - objects passed will be converted to a list of object keys.
//   - { foo: false } would still be invalid, because the key exists (value is ignored)
//   - recommended to pass an array
//
// Example:
// - prevent a button from being clickable if the input value has already been used
// <input ng-model="key" osc-unique="keys" />
// <button ng-disabled="form.key.$error.oscUnique" ng-click="submit()">Submit</button>
//
angular.module('openshiftConsole')
  .directive('oscUnique', function() {
    return {
      restrict: 'A',
      scope: {
        oscUnique: '='
      },
      require: 'ngModel',
      link: function($scope, $elem, $attrs, ctrl) {
        var list = [];

        $scope.$watchCollection('oscUnique', function(newVal) {
          list = _.isArray(newVal) ?
                    newVal :
                    _.keys(newVal);
        });

        ctrl.$parsers.unshift(function(value) {
          // is valid so long as it doesn't already exist
          ctrl.$setValidity('oscUnique', !_.includes(list, value));
          return value;
        });
      }
    };
  });
