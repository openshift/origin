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
  .filter('humanizeUnit', function() {
    return function(unit, type, singular) {
      switch(type) {
      case "memory":
      case "limits.memory":
      case "requests.memory":
      case "storage":
        if (!unit) {
          return unit;
        }
        return unit + "B";
      case "cpu":
      case "limits.cpu":
      case "requests.cpu":
        if (unit === "m") {
          unit = "milli";
        }
        var suffix = (singular) ? 'core' : 'cores';
        return (unit || '') + suffix;
      default:
        return unit;
      }
    };
  })
  // Returns the amount and unit for compute resources, normalizing the unit.
  .filter('amountAndUnit', function(humanizeUnitFilter) {
    return function(value, type, humanizeUnits) {
      if (!value) {
        return [value, null];
      }
      var split = /(-?[0-9\.]+)\s*(.*)/.exec(value);
      if (!split) {
        // We didn't get an amount? shouldn't happen but just in case
        return [value, null];
      }

      var amount = split[1];
      var unit = split[2];
      if (humanizeUnits) {
        unit = humanizeUnitFilter(unit, type, amount === "1");
      }

      return [amount, unit];
    };
  })
  // Formats a compute resource value for display.
  .filter('usageWithUnits', function(amountAndUnitFilter) {
    return function(value, type) {
      var toString = _.spread(function(amount, unit) {
        if (!unit) {
          return amount;
        }

        return amount + " " + unit;
      });

      return toString(amountAndUnitFilter(value, type, true));
    };
  })
  .filter('humanizeSize', function() {
    return function(bytes) {
      if (bytes === null || bytes === undefined || bytes === '') {
        return bytes;
      }

      bytes = Number(bytes);
      if (bytes < 1024) {
        return bytes + " bytes";
      }

      var KiB = bytes / 1024;
      if (KiB < 1024) {
        return KiB.toFixed(1) + " KiB";
      }

      var MiB = KiB / 1024;
      if (MiB < 1024) {
        return MiB.toFixed(1) + " MiB";
      }

      var GiB = MiB / 1024;
      return GiB.toFixed(1) + " GiB";
    };
  })
  .filter('computeResourceLabel', function() {
    return function(computeResourceType, capitalize) {
      switch (computeResourceType) {
      case 'cpu':
        return 'CPU';
      case 'memory':
        return capitalize ? 'Memory' : 'memory';
      default:
        return computeResourceType;
      }
    };
  })
  .filter('helpLink', function(Constants) {
    return function(type) {
      return Constants.HELP[type] || Constants.HELP["default"];
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
    return function(link, ref, contextDir) {
      var m = link.match(/^(?:https?:\/\/|git:\/\/|git\+ssh:\/\/|git\+https:\/\/)?(?:[^@]+@)?github\.com[:\/]([^\/]+\/[^\/]+?)(\/|(?:\.git(#.*)?))?$/);
      if (m) {
        link = "https://github.com/" + m[1];
        // Remove leading / if there is one
        if (contextDir && contextDir.charAt(0) === "/") {
          contextDir = contextDir.substring(1);
        }

        // always use /tree for a generic ref instead of /commit or /blob, /tree will always resolve to the right thing.
        if (contextDir) {
          // Encode it in case there are funky characters in the folder names
          contextDir = encodeURIComponent(contextDir);
          // But then unencode the / characters
          contextDir = contextDir.replace("%2F", "/");
          link += "/tree/" + (encodeURIComponent(ref || "master")) + "/" + contextDir;
        }
        else if (ref && ref !== "master") {
          link += "/tree/" + encodeURIComponent(ref);
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
  })
  // Remove "sha256:" from the start of an identifier if present.
  .filter("stripSHAPrefix", function() {
    return function(id) {
      if (!id) {
        return id;
      }

      return id.replace(/^sha256:/, "");
    };
  })
  // Like limitTo, except if the limit is undefined, return all items instead of none.
  // TODO: Remove when we upgrade Angular since you can pass undefined to limitTo in newer versions.
  //       See https://github.com/angular/angular.js/pull/10510
  .filter("limitToOrAll", function(limitToFilter) {
    return function(input, limit) {
      if (isNaN(limit)) {
        return input;
      }

      return limitToFilter(input, limit);
    };
  })
  .filter("getErrorDetails", function() {
    return function(result) {
      var error = result.data || {};
      if (error.message) {
        return error.message;
      }

      var status = result.status || error.status;
      if (status) {
        return "Status: " + status;
      }

      return "";
    };
  })
  .filter('humanize', function() {
    return function(kind) {
      return kind
          // insert a space between lower & upper
          .replace(/([a-z])([A-Z])/g, '$1 $2')
          // space before last upper in a sequence followed by lower
          .replace(/\b([A-Z]+)([A-Z])([a-z])/, '$1 $2$3')
          // uppercase the first character
          .replace(/^./, function(str){ return str.toUpperCase(); });
    };
  })
  .filter('parseJSON', function() {
    return function(json) {
      // return original value if its null or undefined
      if (!json) {
        return null;
      }

      // return the parsed obj if its valid
      try {
        var jsonObj = JSON.parse(json);
        if (typeof jsonObj === "object") {
          return jsonObj;
        }
        else {
          return null;
        }
      }
      catch (e) {
        // it wasn't valid json
        return null;
      }      
    };
  })
  .filter('prettifyJSON', function(parseJSONFilter) {
    return function(json) {
      var jsonObj = parseJSONFilter(json);
      if (jsonObj) {
        return JSON.stringify(jsonObj, null, 4);
      }
      else {
        // it wasn't a json object, return the original value
        return json;
      }
    };
  })
  // Resource is either a resource object, or a name.  If resource is a name, kind and namespace must be specified
  // Note that builds and deployments can only have their URL built correctly (including their config in the URL)
  // if resource is an object
  .filter('navigateResourceURL', function(Navigate) {
    return function(resource, kind, namespace) {
      return Navigate.resourceURL(resource, kind, namespace);
    };
  })
  .filter('editResourceURL', function(Navigate) {
    return function(resource, kind, namespace) {
      var url = Navigate.resourceURL(resource, kind, namespace, "edit");
      return url;
    };
  })
  .filter('join', function() {
    return function(array, separator) {
      if (!separator) {
        separator = ',';
      }
      return array.join(separator);
    };
  })
  .filter('generateName', function() {
    return function(prefix, length) {
      if (!prefix) {
        prefix = "";
      }
      if (!length) {
        length = 5;
      }
      var randomString = Math.round((Math.pow(36, length + 1) - Math.random() * Math.pow(36, length))).toString(36).slice(1);
      return prefix + randomString;
    };
  })
  .filter('accessModes', function() {
    return function(value, format) {
      if (!value) {
        return value;
      }  
      var accessModes = [];
      angular.forEach(value, function(item) {
        var accessModeString;
        var longForm = format === "long";
        switch(item) {
          case "ReadWriteOnce":
            accessModeString = longForm ? "RWO (Read-Write-Once)" : "Read-Write-Once";
            break;
          case "ReadOnlyMany":
            accessModeString = longForm ? "ROX (Read-Only-Many)" : "Read-Only-Many";
            break;
          case "ReadWriteMany":
            accessModeString = longForm ? "RWX (Read-Write-Many)" : "Read-Write-Many";
            break;
          default:
            accessModeString = item;
        }
        accessModes.push(accessModeString);
      });
      return _.uniq(accessModes);
    };
  })
  .filter('truncate', function() {
    return function(str, charLimit, useWordBoundary, newlineLimit) {
      if (!str) {
        return str;
      }

      var truncated = str;

      if (charLimit) {
        truncated = truncated.substring(0, charLimit);
      }

      if (newlineLimit) {
        var nthNewline = str.split("\n", newlineLimit).join("\n").length;
        truncated = truncated.substring(0, nthNewline);
      }

      if (useWordBoundary !== false) {
        // Find the last word break, but don't look more than 10 characters back.
        // Make sure we show at least the first 5 characters.
        var startIndex = Math.max(4, charLimit - 10);
        var lastSpace = truncated.lastIndexOf(/\s/, startIndex);
        if (lastSpace !== -1) {
          truncated = truncated.substring(0, lastSpace);
        }
      }      

      return truncated;
    };
  });
