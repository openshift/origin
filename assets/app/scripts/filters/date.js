angular.module('openshiftConsole')
  .filter('dateRelative', function() {
    return function(timestamp) {
      if (!timestamp) {
        return timestamp;
      }
      return moment(timestamp).fromNow();
    };
  })
  .filter('duration', function() {
    return function(timestampLhs, timestampRhs) {
      if (!timestampLhs) {
        return timestampLhs;
      }
      timestampRhs = timestampRhs || Date();

      var ms = moment(timestampRhs).diff(timestampLhs);
      var duration = moment.duration(ms);
      // the out of the box humanize in moment.js rounds to the nearest time unit
      // but we need more details
      var humanizedDuration = [];
      var years = duration.years();
      var months = duration.months();
      var days = duration.days();
      var hours = duration.hours();
      var minutes = duration.minutes();
      var seconds = duration.seconds();
      if (years > 0) {
        humanizedDuration.push(years + (years == 1 ? " year" : " years"));
      }
      if (months > 0) {
        humanizedDuration.push(months + (months == 1 ? " month" : " months"));
      }
      if (days > 0) {
        humanizedDuration.push(days + (days == 1 ? " day" : " days"));
      }
      if (hours > 0) {
        humanizedDuration.push(hours + (hours == 1 ? " hour" : " hours"));
      }
      if (minutes > 0) {
        humanizedDuration.push(minutes + (minutes == 1 ? " minute" : " minutes"));
      }
      if (seconds > 0) {
        humanizedDuration.push(seconds + (seconds == 1 ? " second" : " seconds"));
      }      

      if (humanizedDuration.length == 0) {
        humanizedDuration.push("0 seconds");
      }

      if (humanizedDuration.length > 2) {
        humanizedDuration.length = 2;
      }

      return humanizedDuration.join(", ");
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
