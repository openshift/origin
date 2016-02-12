'use strict';

angular.module('openshiftConsole')
  .filter('dateRelative', function() {
    // dropSuffix will tell moment whether to include the "ago" text
    return function(timestamp, dropSuffix) {
      if (!timestamp) {
        return timestamp;
      }
      return moment(timestamp).fromNow(dropSuffix);
    };
  })
  .filter('duration', function() {
    return function(timestampLhs, timestampRhs, omitSingle) {
      if (!timestampLhs) {
        return timestampLhs;
      }
      timestampRhs = timestampRhs || new Date(); // moment expects either an ISO format string or a Date object

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

      function add(count, singularText, pluralText) {
        if (count > 0) {
          if (omitSingle && count === 1) {
            humanizedDuration.push(singularText);
          } else {
            humanizedDuration.push(count + ' ' + (count === 1 ? singularText : pluralText));
          }
        }
      }

      add(years, "year", "years");
      add(months, "month", "months");
      add(days, "day", "days");
      add(hours, "hour", "hours");
      add(minutes, "minute", "minutes");
      add(seconds, "second", "seconds");

      if (humanizedDuration.length === 0) {
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
    };
  })
  .filter('orderObjectsByDate', function(toArrayFilter) {
    return function(items, reverse) {
      items = toArrayFilter(items);

      /*
       * Note: This is a hotspot in our code. We sort frequently by date on
       *       the overview and browse pages.
       */

      items.sort(function (a, b) {
        if (!a.metadata || !a.metadata.creationTimestamp || !b.metadata || !b.metadata.creationTimestamp) {
          throw "orderObjectsByDate expects all objects to have the field metadata.creationTimestamp";
        }

        // The date format can be sorted using straight string comparison.
        // Compare as strings for performance.
        // Example Date: 2016-02-02T21:53:07Z
        if (a.metadata.creationTimestamp < b.metadata.creationTimestamp) {
          return reverse ? 1 : -1;
        }

        if (a.metadata.creationTimestamp > b.metadata.creationTimestamp) {
          return reverse ? -1 : 1;
        }

        return 0;
      });

      return items;
    };
  });
