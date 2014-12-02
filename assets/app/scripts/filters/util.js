angular.module('openshiftConsole')
  .filter('hashSize', function() {
    return function(hash) {
      return Object.keys(hash).length;
    };
  });