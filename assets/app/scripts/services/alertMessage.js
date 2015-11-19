'use strict';

angular.module("openshiftConsole")
  .service("AlertMessageService", function(){
    var alerts = [];
    return {
      addAlert: function(alert) {
        alerts.push(alert);
      },
      getAlerts: function() {
        return alerts;
      },
      clearAlerts: function() {
        alerts = [];
      }
    };
  });