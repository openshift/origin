angular.module('openshiftConsole')
  .filter('hashSize', function() {
    return function(hash) {
      if(!hash) return 0;
      return Object.keys(hash).length;
    };
  })
  .filter('usageValue', function() {
    return function(value) {
      if (!value) {
        return value;
      }
      var split = /(-?[0-9\.]+)\s*(.*)/.exec(value);
      if (!split) {
        // We didn't get an amount? shouldn't happen but just in case
        return value;
      }
      var number = split[1];
      if (number.indexOf(".") >= 0) {
        return parseFloat(number);
      }
      else {
        return parseInt(split[1]);
      }
    }
  })  
  .filter('usageWithUnits', function() {
    return function(value, type) {
      if (!value) {
        return value;
      }
      // only special casing memory at the moment
      var split = /(-?[0-9\.]+)\s*(.*)/.exec(value);
      if (!split) {
        // We didn't get an amount? shouldn't happen but just in case
        return value;
      }
      var amount = split[1];
      var unit = split[2];
      switch(type) {
        case "memory":
          unit += "B";
          break;
        case "cpu":
          if (unit == "m") {
            unit = "milli";
          }
          unit += (amount == "1" ? "core" : "cores")
          break;
      }
      return amount + (unit != "" ? " " + unit : "");
    }
  })
  .filter('helpLink', function() {
    return function(type) {
      switch(type) {
        case "webhooks":
          return "http://docs.openshift.org/latest/using_openshift/builds.html#webhook-triggers"
        default:
          return "http://docs.openshift.org/latest/welcome/index.html";
      }
    };
  });
