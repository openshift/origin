angular.module('openshiftConsole')
  .filter('dateRelative', function() {
    return function(timestamp) {
      if (!timestamp) {
        return timestamp;
      }
      return moment(timestamp).fromNow();
    };
  })
  .filter('ageLessThan', function() {
    // ex:  amt = 5  and unit = 'minutes'
    return function(timestamp, amt, unit) {
      return moment().subtract(amt, unit).diff(moment(timestamp)) < 0;
    }
  })
  .filter('orderObjectsByDate', function() {
    return function(items, reverse) {
      var filtered = [];
      angular.forEach(items, function(item) {
        filtered.push(item);
      });
      filtered.sort(function (a, b) {
        if (!a.metadata || !a.metadata.creationTimestamp || !b.metadata || !b.metadata.creationTimestamp) {
          throw "orderObjectsByDate expects all objects to have the field metadata.creationTimestamp";
        }
        return moment(a.metadata.creationTimestamp).diff(moment(b.metadata.creationTimestamp));
      });
      if(reverse) filtered.reverse();
      return filtered;
    }
  });
