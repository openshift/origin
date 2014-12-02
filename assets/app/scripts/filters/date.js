angular.module('openshiftConsole')
  .filter('dateRelative', function() {
    return function(timestamp) {
      return moment(timestamp).fromNow();
    };
  })
  .filter('ageLessThan', function() {
    // ex:  amt = 5  and unit = 'minutes'
    return function(timestamp, amt, unit) {
      return moment().subtract(amt, unit).diff(moment(timestamp)) < 0;
    }
  });