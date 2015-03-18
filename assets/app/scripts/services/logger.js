'use strict';

angular.module('openshiftConsole')
.provider('Logger', function() {
  this.$get = function() {
    // Wraps the global Logger from https://github.com/jonnyreeves/js-logger
    var OSLogger = Logger.get("OpenShift");
    var logger = {
      get: function(name) {
        var logger = Logger.get("OpenShift/" + name);
        logger.setLevel(Logger[localStorage['OpenShiftLogLevel.' + name] || "OFF"]);      
        return logger;
      },
      log: function() {
        OSLogger.log.apply(OSLogger, arguments);
      },
      info: function() {
        OSLogger.info.apply(OSLogger, arguments);
      },      
      debug: function() {
        OSLogger.debug.apply(OSLogger, arguments);
      },
      warn: function() {
        OSLogger.warn.apply(OSLogger, arguments);
      },
      error: function() {
        OSLogger.error.apply(OSLogger, arguments);
      }
    };

    // Set default log level
    OSLogger.setLevel(Logger[localStorage['OpenShiftLogLevel.main'] || "OFF"]);      
    return logger;
  };
});