'use strict';

angular.module('openshiftConsole')
.factory('DataService', [function() {
  // TODO this is not the ideal, issue open to discuss adding
  // an introspection endpoint that would give us this mapping
  // https://github.com/openshift/origin/issues/230
  var TYPE_MAP = {
    builds : "osapi",    
    deployments : "osapi",
    deploymentConfigs : "osapi",
    images : "osapi",
    projects : "osapi",
    pods : "api",
    services : "api",
    replicationControllers: "api"
  };

  function DataService() {
    this._subscriptions = {};
    this._subscriptionsPolling = {};
    this._openConnections = {};
  }

  // Note: temporarily supressing jshint warning for unused opts, since we intend to use opts
  // for various things in the future
  DataService.prototype.getList = function(type, callback, context, opts) { // jshint ignore:line
    // TODO where do we track resourceVersion
    // TODO how do we handle failures
    // Check if the project deferred exists on the context, if it does then don't
    // request the data till the project is known
    if (callback.fire) {
      // callback is a $.Callbacks() list
      var callbackList = callback;
      callback = function(data) {callbackList.fire(data);};
    }
    else if (callback.resolve) {
      // callback is a $.Deferred() promise
      var deferred = callback;
      callback = function(data) {deferred.resolve(data);};
    }
    if (context.projectPromise && type !== "projects") {
      context.projectPromise.done($.proxy(function(project) {
        $.ajax({
          url: this._urlForType(type, null, context, false, {namespace: project.metadata.namespace}),
          success: callback
        });
      }, this));
    }
    else {
      $.ajax({
        url: this._urlForType(type, null, context),
        success: callback
      });
    }
  };

  // Note: temporarily supressing jshint warning for unused opts, since we intend to use opts
  // for various things in the future
  DataService.prototype.getObject = function(type, id, callback, context, opts) { // jshint ignore:line
    // TODO where do we track resourceVersion
    // TODO how do we handle failures
    if (context.projectPromise && type !== "projects") {
      context.projectPromise.done($.proxy(function(project) {
        $.ajax({
          url: this._urlForType(type, id, context, false, {namespace: project.metadata.namespace}),
          success: callback
        });
      }, this));
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
  // Note: temporarily supressing jshint warning for unused opts, since we intend to use opts
  // for various things in the future  
  DataService.prototype.subscribe = function(type, callback, context, opts) { // jshint ignore:line
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

  // returns the object needed for unsubscribing, currently
  // this is the callback itself
  // Note: temporarily supressing jshint warning for unused opts, since we intend to use opts
  // for various things in the future  
  DataService.prototype.subscribePolling = function(type, callback, context, opts) { // jshint ignore:line
    if (!this._subscriptionsPolling[type]) {
      this._subscriptionsPolling[type] = $.Callbacks();
      this._subscriptionsPolling[type].add(callback);
      // TODO restrict to resourceVersion that we get back from initial ajax request
      this._listenForUpdatesPolling(type, context);
    }
    else {
      this._subscriptionsPolling[type].add(callback);
    }
    return callback;
  };

  DataService.prototype.unsubscribePolling = function(type, callback) {
    if (this._subscriptionsPolling[type] && this._subscriptionsPolling[type].has()){
      this._subscriptionsPolling[type].remove(callback);
      if (!this._subscriptionsPolling[type].has()) {
        this._stopListeningForUpdatesPolling(type);
      }
    }
  };

  DataService.prototype.objectsByAttribute = function(objects, attr, map, actions, secondaryAttr) {
    for (var i = 0; i < objects.length; i++) {
      this.objectByAttribute(objects[i], attr, map, actions ? actions[i] : null, secondaryAttr);
    }
  };

  // Handles attr with dot notation
  // TODO write lots of tests for this helper
  DataService.prototype.objectByAttribute = function(obj, attr, map, action, secondaryAttr) {
    var subAttrs = attr.split(".");
    var attrValue = obj;
    for (var i = 0; i < subAttrs.length; i++) {
      attrValue = attrValue[subAttrs[i]];
      if (!attrValue) {
        return;
      }
    }
    if ($.isArray(attrValue)) {
      // TODO implement this when we actually need it
    }
    else if ($.isPlainObject(attrValue)) {
      for (var key in attrValue) {
        var val = attrValue[key];
        if (!map[key]) {
          map[key] = {};
        }
        if (secondaryAttr) {
          if (action == "DELETED") {
            delete map[key][val][secondaryAttr];
          }
          else {
            if (!map[key][val]) {
              map[key][val] = {};
            }
            map[key][val][obj[secondaryAttr]] = obj;
          }
        }
        else {
          if (action == "DELETED") {
            delete map[key][val];
          }
          else {
            map[key][val] = obj;
          }
        }
      }
    }
    else {
      if (action == "DELETED") {
        if (secondaryAttr) {
          delete map[attrValue][obj[secondaryAttr]];
        }
        else {
          delete map[attrValue];
        }
      }
      else {
        if (secondaryAttr) {
          if (!map[attrValue]) {
            map[attrValue] = {};
          }
          map[attrValue][obj[secondaryAttr]] = obj;
        }
        else {
          map[attrValue] = obj;
        }
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
      if (context.projectPromise && type !== "projects") {
        context.projectPromise.done($.proxy(function(project) {
          params.namespace = project.metadata.namespace;
          var wsUrl = this._urlForType(type, null, context, true, params);
          var ws = this._openConnections[type] = new WebSocket(wsUrl);
          ws.onclose = $.proxy(this, "_onSocketClose", type, context);
          ws.onmessage = $.proxy(this, "_onSocketMessage", type, context);
        }, this));
      }
      else {
        var wsUrl = this._urlForType(type, null, context, true, params);

        var ws = this._openConnections[type] = new WebSocket(wsUrl);
        ws.onclose = $.proxy(this, "_onSocketClose", type, context);
        ws.onmessage = $.proxy(this, "_onSocketMessage", type, context);
      }
    }
  };

  DataService.prototype._stopListeningForUpdatesPolling = function(type) {
    //TODO implement this
  };

  DataService.prototype._listenForUpdatesPolling = function(type, context) {
    this.getList(type, this._subscriptionsPolling[type], context);
    setTimeout($.proxy(this, "_listenForUpdatesPolling", type, context), 5000);
  };

  DataService.prototype._onSocketClose = function(type, context, event) {
    // Attempt to re-establish the connection in cases
    // where the socket close was unexpected, i.e. the event's
    // wasClean attribute is false
    if (!event.wasClean) {
      // TODO should track latest resourceVersion we know about
      // for a type so we only reload what we need
      this._listenForUpdates(type, context);
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

  var URL_ROOT_TEMPLATE = "{protocol}://{+serverUrl}/{apiRoot}/{apiVersion}/";
  var URL_WATCH_LIST = URL_ROOT_TEMPLATE + "watch/{type}{?q*}";
  var URL_GET_LIST = URL_ROOT_TEMPLATE + "{type}{?q*}";
  var URL_GET_OBJECT = URL_ROOT_TEMPLATE + "{type}/{id}{?q*}";

  DataService.prototype._urlForType = function(type, id, context, isWebsocket, params) {
    var protocol;
    if (isWebsocket) {
      protocol = window.location.protocol === "http:" ? "ws" : "wss";
    }
    else {
      protocol = window.location.protocol === "http:" ? "http" : "https"; 
    }

    var template;
    if (isWebsocket) {
      template = URL_WATCH_LIST;
    }
    else if (id) {
      template = URL_GET_OBJECT;
    }
    else {
      template = URL_GET_LIST;
    }

    // TODO where do we specify what the server URL and api version should be
    return URI.expand(template, {
      protocol: protocol,
      serverUrl: "localhost:8080",
      apiRoot: TYPE_MAP[type],
      apiVersion: "v1beta1",
      type: type,
      id: id,
      q: params
    });
  };

  return new DataService();
}]);