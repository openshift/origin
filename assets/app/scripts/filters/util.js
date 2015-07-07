'use strict';

angular.module('openshiftConsole')
  /**
   * Replace special chars with underscore (e.g. '.')
   * @returns {Function}
   */
  .filter("underscore", function(){
    return function(value){
      return value.replace(/\./g, '_');
    };
  })
  .filter("defaultIfBlank", function(){
    return function(input, defaultValue){
      if(input === null) {
        return defaultValue;
      }
      if(typeof input !== "string"){
        input = String(input);
      }
      if(input.trim().length === 0){
        return defaultValue;
      }
      return input;
    };
  })
  .filter('hashSize', function() {
    return function(hash) {
      if(!hash) { return 0; }
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
        number = parseFloat(number);
      }
      else {
        number =  parseInt(split[1]);
      }
      var siSuffix = split[2];
      var multiplier = 1;
      switch(siSuffix) {
        case 'E':
          multiplier = Math.pow(1000, 6);
          break;
        case 'P':
          multiplier = Math.pow(1000, 5);
          break;
        case 'T':
          multiplier = Math.pow(1000, 4);
          break;
        case 'G':
          multiplier = Math.pow(1000, 3);
          break;
        case 'M':
          multiplier = Math.pow(1000, 2);
          break;
        case 'K':
          multiplier = 1000;
          break;
        case 'm':
          multiplier = 0.001;
          break;
        case 'Ei':
          multiplier = Math.pow(1024, 6);
          break;
        case 'Pi':
          multiplier = Math.pow(1024, 5);
          break;
        case 'Ti':
          multiplier = Math.pow(1024, 4);
          break;
        case 'Gi':
          multiplier = Math.pow(1024, 3);
          break;
        case 'Mi':
          multiplier = Math.pow(1024, 2);
          break;
        case 'Ki':
          multiplier = 1024;
          break;
      }

      return number * multiplier;
    };
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
          if (unit === "m") {
            unit = "milli";
          }
          unit += (amount === "1" ? "core" : "cores");
          break;
      }
      return amount + (unit !== "" ? " " + unit : "");
    };
  })
  .filter('helpLink', function() {
    return function(type) {
      switch(type) {
        case "webhooks":
          return "http://docs.openshift.org/latest/dev_guide/builds.html#webhook-triggers";
        case "start-build":
          return "http://docs.openshift.org/latest/dev_guide/builds.html#starting-a-build";
        case "deployment-operations":
          return "http://docs.openshift.org/latest/cli_reference/basic_cli_operations.html#deployment-operations";
        default:
          return "http://docs.openshift.org/latest/welcome/index.html";
      }
    };
  })
  .filter('taskTitle', function() {
    return function(task) {
      if (task.status !== "completed") {
        return task.titles.started;
      }
      else {
        if (task.hasErrors) {
          return task.titles.failure;
        }
        else {
          return task.titles.success;
        }
      }
    };
  })
  .filter('httpHttps', function() {
    return function(isSecure) {
        return isSecure ? 'https://' : 'http://';
    };
  })
  .filter('githubLink', function() {
    return function(link, commit) {
      var m = link.match(/^(?:https?:\/\/|git:\/\/|git\+ssh:\/\/|git\+https:\/\/)?(?:[^@]+@)?github\.com[:\/]([^\/]+\/[^\/]+?)(?:\.git(#.*)?)?$/);
      if (m) {
        link = "https://github.com/" + m[1];
        if (commit) {
          link += "/commit/" + commit;
        }
      }
      return link;
    };
  })
  .filter('yesNo', function() {
      return function(isTrue) {
          return isTrue ? 'Yes' : 'No';
      };
  })
  /**
   * Filter a hash of values
   *
   * @param {Hash} entries  A Hash to filter
   * @param {String} keys    A comma delimited string of keys to evaluate against
   * @returns {Hash} A filtered set where the keys of those in keys
   */
  .filter("valuesIn", function(){
    return function(entries, keys){
      var readonly = keys.split(",");
      var result = {};
      angular.forEach(entries, function(value, key){
        if( readonly.indexOf(key) !== -1){
          result[key] = value;
        }
      });
      return result;
    };
  })
    /**
   * Filter a hash of values
   *
   * @param {Hash} entries  A Hash to filter
   * @param {String} keys    A comma delimited string of keys to evaluate against
   * @returns {Hash} A filtered set where the keys of those not in keys
   */
  .filter("valuesNotIn", function(){
    return function(entries, keys){
      var readonly = keys.split(",");
      var result = {};
      angular.forEach(entries, function(value, key){
        if( readonly.indexOf(key) === -1){
          result[key] = value;
        }
      });
      return result;
    };
  })
  .filter("toArray", function() {
    return function(items) {
      if (!items) {
        return [];
      }

      if (angular.isArray(items)) {
        return items;
      }

      var itemsArray = [];
      angular.forEach(items, function(item) {
        itemsArray.push(item);
      });

      return itemsArray;
    };
  });
