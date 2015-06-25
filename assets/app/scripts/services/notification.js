'use strict';
/* jshint newcap: false */

angular.module('openshiftConsole')
.factory('Notification', function($rootScope) {
  function Notification() {
    this.messenger = Messenger({
      extraClasses: 'messenger-fixed messenger-on-bottom messenger-on-right',
      theme: 'flat',
      messageDefaults: {
        showCloseButton: true,
        hideAfter: 10
      }
    });
    var self = this;
    $rootScope.$on( "$routeChangeStart", function() {
      self.clear();
    });
  }

  // Opts:
  //    id - if an id is passed only one message with this id will ever be shown
  //    mustDismiss - the user must explicitly dismiss the message, it will not auto-hide
  Notification.prototype.notify = function(type, message, opts) {
    opts = opts || {};
    var notifyOpts = {
      type: type,
      message: message,
      id: opts.id,
      actions: opts.actions
    };
    if (opts.mustDismiss) {
      notifyOpts.hideAfter = false;
    }
    this.messenger.post(notifyOpts);
  };

  Notification.prototype.success = function(message, opts) {
    this.notify("success", message, opts);
  };

  Notification.prototype.info = function(message, opts) {
    this.notify("info", message, opts);
  };

  Notification.prototype.error = function(message, opts) {
    this.notify("error", message, opts);
  };

  Notification.prototype.warning = function(message, opts) {
    this.notify("warning", message, opts);
  };

  Notification.prototype.clear = function() {
    this.messenger.hideAll();
  };

  return new Notification();
});
