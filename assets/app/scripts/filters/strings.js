'use strict';
angular.module('openshiftConsole')
  .filter('sentenceCase', function() {
    // Converts a camel case string to sentence case
    return function(str) {
      if (!str) {
        return str;
      }

      // Unfortunately, _.lowerCase() and _.upperFirst() aren't in our lodash version.
      var lower = _.startCase(str).toLowerCase();
      return lower.charAt(0).toUpperCase() + lower.slice(1);
    };
  })
  .filter('startCase', function () {
    return function(str) {
      if (!str) {
        return str;
      }

      // https://lodash.com/docs#startCase
      return _.startCase(str);
    };
  })
  .filter('capitalize', function() {
    return function(input) {
      return _.capitalize(input);
    };
  });
