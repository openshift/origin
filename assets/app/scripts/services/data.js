'use strict';

angular.module('openshiftConsole')
.factory('DataService', [function() {
  // TODO this is not the ideal, issue open to discuss adding
  // an introspection endpoint that would give us this mapping
  // https://github.com/openshift/origin/issues/230
  var TYPE_MAP = {
    projects : "osapi",
    pods : "api"
  };

  function DataService() {
    this._subscriptions = {};
    this._openConnections = {};
  }

  DataService.prototype.getList = function(type, callback, context, opts) {
    // TODO where do we track resourceVersion
    // TODO how do we handle failures
    // Check if the project deferred exists on the context, if it does then don't
    // request the data till the project is known
    if (context.project && type != "projects") {
      context.project.done($.proxy(this, function(project) {
        $.ajax({
          url: this._urlForType(type, null, context, false, {namespace: project.namespace}),
          success: callback
        });
      }));
    }
    else {
      $.ajax({
        url: this._urlForType(type, null, context),
        success: callback
      });
    }
  };

  DataService.prototype.getObject = function(type, id, callback, context, opts) {
    // TODO where do we track resourceVersion
    // TODO how do we handle failures
    if (context.project && type != "projects") {
      context.project.done($.proxy(this, function(project) {
        $.ajax({
          url: this._urlForType(type, id, context, false, {namespace: project.namespace}),
          success: callback
        });
      }));
    }
    else {
      $.ajax({
        url: this._urlForType(type, id, context),
        success: callback
      });
    }
  };

  // returns the object needed for unsubscribing, currently
  // this is the callback itself
  DataService.prototype.subscribe = function(type, callback, context, opts) {
    if (!this._subscriptions[type]) {
      this._subscriptions[type] = $.Callbacks();
      this._subscriptions[type].add(callback);
      // TODO restrict to resourceVersion that we get back from initial ajax request
      this._listenForUpdates(type, context);
    }
    else {
      this._subscriptions[type].add(callback);
    }
    return callback;
  };

  DataService.prototype.unsubscribe = function(type, callback) {
    if (this._subscriptions[type] && this._subscriptions[type].has()){
      this._subscriptions[type].remove(callback);
      if (!this._subscriptions[type].has()) {
        this._stopListeningForUpdates(type);
      }
    }
  };

  DataService.prototype._stopListeningForUpdates = function(type) {
    if (this._openConnections[type]) {
     this._openConnections[type].close();
     // TODO can we use delete here instead, or will that screw up the onclose event
     this._openConnections[type] = null;
    }
  };

  DataService.prototype._listenForUpdates = function(type, context, resourceVersion) {
    // Note: current impl uses one websocket per type
    // eventually want a single websocket connection that we
    // send a subscription request to for each type

    // Only listen for updates if websockets are available
    if (WebSocket) {
      var params = {};
      if (resourceVersion) {
        params.resourceVersion = resourceVersion;
      }
      if (context.project) {
        context.project.done($.proxy(function(project) {
          params.namespace = project.namespace;
          var wsUrl = this._urlForType(type, null, context, true, params);
          var ws = this._openConnections[type] = new WebSocket(wsUrl);
          ws.onclose = $.proxy(this, "_onSocketClose", type);
          ws.onmessage = $.proxy(this, "_onSocketMessage", type, context);
        }, this));
      }
      else {
        var wsUrl = this._urlForType(type, null, context, true, params);

        var ws = this._openConnections[type] = new WebSocket(wsUrl);
        ws.onclose = $.proxy(this, "_onSocketClose", type);
        ws.onmessage = $.proxy(this, "_onSocketMessage", type, context);
      }
    }
  };

  DataService.prototype._onSocketClose = function(type, event) {
    // Attempt to re-establish the connection in cases
    // where the socket close was unexpected, i.e. the event's
    // wasClean attribute is false
    if (!event.wasClean) {
      // TODO should track latest resourceVersion we know about
      // for a type so we only reload what we need
      this._listenForUpdates(type);
    }
  };

  DataService.prototype._onSocketMessage = function(type, context, event) {
    try {
      var eventData = $.parseJSON(event.data);
      if (this._subscriptions[type].has()) {
        // eventData.type will be one of ADDED, MODIFIED, DELETED
        this._subscriptions[type].fire(eventData.type, eventData.object);
      }
    }
    catch (e) {
      // TODO report the JSON parsing exception
    }
  };

  DataService.prototype._urlForType = function(type, id, context, isWebsocket, params) {
    // TODO where do we specify what the server URL should be    
    var serverUrl = "localhost:8080";
    // TODO where do we specify what API version
    var apiVersion = "v1beta1";
    var protocol;
    if (isWebsocket) {
      protocol = window.location.protocol === "http:" ? "ws://" : "wss://";
    }
    else {
      protocol = window.location.protocol === "http:" ? "http://" : "https://"; 
    }
    var url = protocol + serverUrl + "/" + TYPE_MAP[type] + "/" + apiVersion + "/";
    if (isWebsocket) {
      url += "watch/";
    }
    url += type;
    if (id) {
      url += "/" + id;
    }
    if (params && !$.isEmptyObject(params)) {
      url += "?" + $.param(params);
    }
    return url;
  };

  return new DataService();
}]);