'use strict';

angular.module('openshiftConsole')
.factory('DataService', [function() {
  function Data(array) {
    // TODO just need to check for id until v1beta3
    this._data = {};
    if (array.length > 0 && array[0].id) {
      this._objectsByAttribute(array, "id", this._data);
    }
    else {
      this._objectsByAttribute(array, "metadata.name", this._data);
    }
  }

  Data.prototype.by = function(attr, secondaryAttr) {
    // TODO store already generated indices
    if (attr == "id" || attr == "metadata.name") {
      return this._data;
    }
    var map = {};
    for (var key in this._data) {
      _objectByAttribute(this._data[key], attr, map, null, secondaryAttr);
    }
    return map;
  };

  Data.prototype.update = function(object, action) {
    _objectByAttribute(object, object.id ? "id" : "metadata.name", this._data, action);
  };

  Data.prototype._objectsByAttribute = function(objects, attr, map, actions, secondaryAttr) {
    for (var i = 0; i < objects.length; i++) {
      _objectByAttribute(objects[i], attr, map, actions ? actions[i] : null, secondaryAttr);
    }
  };

  // Handles attr with dot notation
  // TODO write lots of tests for this helper
  // Note: this lives outside the Data prototype for now so it can be used by the helper in DataService as well
  var _objectByAttribute = function(obj, attr, map, action, secondaryAttr) {
    var subAttrs = attr.split(".");
    var attrValue = obj;
    for (var i = 0; i < subAttrs.length; i++) {
      attrValue = attrValue[subAttrs[i]];
      if (!attrValue) {
        return;
      }
    }

    // Split the secondary attribute by dot notation if there is one
    var secondaryAttrValue = obj;
    if (secondaryAttr) {
      // TODO remove this when we don't have to special case id
      if (secondaryAttr == "metadata.name") {
        if (obj.id) {
          secondaryAttr = "id";
        }
      }
      var subSecondaryAttrs = secondaryAttr.split(".");
      for (var i = 0; i < subSecondaryAttrs.length; i++) {
        secondaryAttrValue = secondaryAttrValue[subSecondaryAttrs[i]];
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
            delete map[key][val][secondaryAttrValue];
          }
          else {
            if (!map[key][val]) {
              map[key][val] = {};
            }
            map[key][val][secondaryAttrValue] = obj;
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
          delete map[attrValue][secondaryAttrValue];
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
          map[attrValue][secondaryAttrValue] = obj;
        }
        else {
          map[attrValue] = obj;
        }
      }
    }
  };


  function DataService() {
    this._listCallbacksMap = {};
    this._watchCallbacksMap = {};
    this._watchOperationMap = {};
    this._listOperationMap = {};
    this._resourceVersionMap = {};
    this._dataMap = {};
    this._watchOptionsMap = {};
    this._watchWebsocketsMap = {};
    this._watchPollTimeoutsMap = {};
  }

// type:      API type (e.g. "pods")
// context:   API context (e.g. {project: "..."})
// callback:  function to be called with the list of the requested type and context,
//            parameters passed to the callback:
//            Data:   a Data object containing the (context-qualified) results
//                    which includes a helper method for returning a map indexed
//                    by attribute (e.g. data.by('metadata.name'))
// opts:      options (currently none, placeholder)
  DataService.prototype.list = function(type, context, callback, opts) {
    var callbacks = this._listCallbacks(type, context)
    callbacks.add(callback);

    if (this._watchInFlight(type, context) && this._resourceVersion(type, context)) {
      // A watch operation is running, and we've already received the 
      // initial set of data for this type
      callbacks.fire(this._data(type, context));
      callbacks.empty();
    }
    else if (this._listInFlight(type, context)) {
      // no-op, our callback will get called when listOperation completes
    }
    else {
      this._startListOp(type, context);
    }
  };

// type:      API type (e.g. "pods")
// name:      API name, the unique name for the object 
// context:   API context (e.g. {project: "..."})
// callback:  function to be called with the requested object,
//            parameters passed to the callback:
//            object: the requested object or an error if the object does not exist
// opts:      options (currently none, placeholder)
  DataService.prototype.get = function(type, name, context, callback, opts) {
    if (this._watchInFlight(type, context) && this._resourceVersion(type, context)) {
      // TODO can take out the id bit once v1beta3 is there
      var obj = this._data(type, context).by('metadata.name')[name];
      if (obj) {
        callback(obj);
      }
      else {
        // TODO
        // callback(simulation of API object not found error?)
      }
    }
    else {
      if (context.projectPromise && type !== "projects") {
        context.projectPromise.done($.proxy(function(project) {
          $.ajax({
            url: this._urlForType(type, name, context, false, {namespace: project.metadata.name}),
            success: callback
          });
        }, this));
      }
      else {
        $.ajax({
          url: this._urlForType(type, name, context),
          success: callback
        });
      }
    }
  };

// type:      API type (e.g. "pods")
// context:   API context (e.g. {project: "..."})
// callback:  function to be called with the initial list of the requested type,
//            and when updates are received, parameters passed to the callback:
//            Data:   a Data object containing the (context-qualified) results
//                    which includes a helper method for returning a map indexed
//                    by attribute (e.g. data.by('metadata.name'))
//            event:  specific event that caused this call ("ADDED", "MODIFIED",
//                    "DELETED", or null) callbacks can optionally use this to 
//                    more efficiently process updates
//            obj:    specific object that caused this call (may be null if the
//                    entire list was updated) callbacks can optionally use this 
//                    to more efficiently process updates
// opts:      options 
//            poll:   true | false - whether to poll the server instead of opening
//                    a websocket. Default is false.
//            pollInterval: in milliseconds, how long to wait between polling the server
//                    only applies if poll=true.  Default is 5000.
//
// returns handle to the watch, needed to unwatch e.g.
//        var handle = DataService.watch(type,context,callback[,opts])
//        DataService.unwatch(handle)
  DataService.prototype.watch = function(type, context, callback, opts) {
    opts = opts || {};
    this._watchCallbacks(type, context).add(callback);

    var existingWatchOpts = this._watchOptions(type, context);
    if (existingWatchOpts) {
      // Check any options for compatibility with existing watch
      if (existingWatchOpts.poll != opts.poll) {
        throw "A watch already exists for " + type + " with a different polling option.";
      }
    }
    else {
      this._watchOptions(type, context, opts);      
    }

    if (this._watchInFlight(type, context) && this._resourceVersion(type, context)) {
      callback(this._data(type, context));
    }
    else if (this._listInFlight(type, context)) {
      // no-op, our callback will get called when listOperation completes
    }
    else {
      this._startListOp(type, context);
    }

    // returned handle needs type, context, and callback in order to unwatch
    return {
      type: type,
      context: context,
      callback: callback,
      opts: opts
    };
  };

  DataService.prototype.unwatch = function(handle) {
    var type = handle.type;
    var context = handle.context;
    var callback = handle.callback;
    var opts = handle.opts;
    var callbacks = this._watchCallbacks(type, context);
    callbacks.remove(callback);
    if (!callbacks.has()) {
      if (opts && opts.poll) {
        clearTimeout(this._watchPollTimeouts(type, context));
        this._watchPollTimeouts(type, context, null);
      }
      else {
        this._watchWebsockets(type, context).close();
        this._watchWebsockets(type, context, null);
      }
      
      this._watchInFlight(type, context, false);
      this._watchOptions(type, context, null);
    }
  };


  DataService.prototype._watchCallbacks = function(type, context) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!this._watchCallbacksMap[key]) {
      this._watchCallbacksMap[key] = $.Callbacks();
    }
    return this._watchCallbacksMap[key];
  };

  DataService.prototype._listCallbacks = function(type, context) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!this._listCallbacksMap[key]) {
      this._listCallbacksMap[key] = $.Callbacks();
    }
    return this._listCallbacksMap[key];
  };

  // maybe change these
  DataService.prototype._watchInFlight = function(type, context, op) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!op && op !== false) {
      return this._watchOperationMap[key];
    }
    else {
      this._watchOperationMap[key] = op;
    }
  };

  DataService.prototype._listInFlight = function(type, context, op) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!op && op !== false) {
      return this._listOperationMap[key];
    }
    else {
      this._listOperationMap[key] = op;
    }
  };

  DataService.prototype._resourceVersion = function(type, context, rv) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!rv) {
      return this._resourceVersionMap[key];
    }
    else {
      this._resourceVersionMap[key] = rv;
    }
  };

  DataService.prototype._data = function(type, context, data) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!data) {
      return this._dataMap[key];
    }
    else {
      this._dataMap[key] = new Data(data);
    }
  }; 

  DataService.prototype._watchOptions = function(type, context, opts) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (opts === undefined) {
      return this._watchOptionsMap[key];
    }
    else {
      this._watchOptionsMap[key] = opts;
    }
  }; 

  DataService.prototype._watchPollTimeouts = function(type, context, timeout) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!timeout) {
      return this._watchPollTimeoutsMap[key];
    }
    else {
      this._watchPollTimeoutsMap[key] = timeout;
    }
  }; 

  DataService.prototype._watchWebsockets = function(type, context, timeout) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (!timeout) {
      return this._watchWebsocketsMap[key];
    }
    else {
      this._watchWebsocketsMap[key] = timeout;
    }
  };     

  DataService.prototype._uniqueKeyForTypeContext = function(type, context) {
    // Note: when we start handling selecting multiple projects this
    // will change to include all relevant scope
    return type + context.projectName;
  };

  DataService.prototype._startListOp = function(type, context) {
    // mark the operation as in progress
    this._listInFlight(type, context, true);

    if (context.projectPromise && type !== "projects") {
      context.projectPromise.done($.proxy(function(project) {
        $.ajax({
          url: this._urlForType(type, null, context, false, {namespace: project.metadata.name}),
          success: $.proxy(this, "_listOpComplete", type, context)
        });
      }, this));
    }
    else {
      $.ajax({
        url: this._urlForType(type, null, context),
        success: $.proxy(this, "_listOpComplete", type, context)
      });
    }
  };

  DataService.prototype._listOpComplete = function(type, context, data) {
    this._resourceVersion(type, context, data.resourceVersion || data.metadata.resourceVersion);
    this._data(type, context, data.items);
    this._listCallbacks(type, context).fire(this._data(type, context));
    this._listCallbacks(type, context).empty();    
    this._watchCallbacks(type, context).fire(this._data(type, context));

    // mark list op as complete
    this._listInFlight(type, context, false);

    if (this._watchCallbacks(type, context).has()) {
      var watchOpts = this._watchOptions(type, context) || {};
      if (watchOpts.poll) {
        this._watchInFlight(type, context, true);
        this._watchPollTimeouts(type, context, setTimeout($.proxy(this, "_startListOp", type, context), watchOpts.pollInterval || 5000));
      }
      else if (!this._watchInFlight(type, context)) {
        this._startWatchOp(type, context, this._resourceVersion(type, context));
      }
    }
  };

  DataService.prototype._startWatchOp = function(type, context, resourceVersion) {
    this._watchInFlight(type, context, true);
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
          params.namespace = project.metadata.name;
          var wsUrl = this._urlForType(type, null, context, true, params);
          var ws = new WebSocket(wsUrl);
          this._watchWebsockets(type, context, ws);
          ws.onclose = $.proxy(this, "_watchOpOnClose", type, context);
          ws.onmessage = $.proxy(this, "_watchOpOnMessage", type, context);
        }, this));
      }
      else {
        var wsUrl = this._urlForType(type, null, context, true, params);
        var ws = new WebSocket(wsUrl);
        this._watchWebsockets(type, context, ws);
        ws.onclose = $.proxy(this, "_watchOpOnClose", type, context);
        ws.onmessage = $.proxy(this, "_watchOpOnMessage", type, context);
      }
    }
  };

  DataService.prototype._watchOpOnMessage = function(type, context, event) {
    try {
      var eventData = $.parseJSON(event.data);

      this._resourceVersion(type, context, eventData.object.resourceVersion || eventData.object.metadata.resourceVersion);
      // TODO do we reset all the by() indices, or simply update them, since we should know what keys are there?
      // TODO let the data object handle its own update
      this._data(type, context).update(eventData.object, eventData.type);
      this._watchCallbacks(type, context).fire(this._data(type, context), eventData.type, eventData.object);
    }
    catch (e) {
      // TODO report the JSON parsing exception
    }    
  };

  DataService.prototype._watchOpOnClose = function(type, context, event) {
    // Attempt to re-establish the connection in cases
    // where the socket close was unexpected, i.e. the event's
    // wasClean attribute is false
    if (!event.wasClean && this._watchCallbacks(type, context).has()) {
      this._startWatchOp(type, context, this._resourceVersion(type, context));
    }
  };

  // TODO Possibly remove these from DataService
  DataService.prototype.objectsByAttribute = function(objects, attr, map, actions, secondaryAttr) {
    for (var i = 0; i < objects.length; i++) {
      _objectByAttribute(objects[i], attr, map, actions ? actions[i] : null, secondaryAttr);
    }
  };

  var URL_ROOT_TEMPLATE = "{protocol}://{+serverUrl}{+apiPrefix}/{apiVersion}/";
  var URL_WATCH_LIST = URL_ROOT_TEMPLATE + "watch/{type}{?q*}";
  var URL_GET_LIST = URL_ROOT_TEMPLATE + "{type}{?q*}";
  var URL_GET_OBJECT = URL_ROOT_TEMPLATE + "{type}/{id}{?q*}";


  var apicfg = OPENSHIFT_CONFIG.api;

  // Set the api version the console is currently able to talk to
  apicfg.openshift.version = "v1beta1";
  apicfg.k8s.version = "v1beta1";
  
  // TODO this is not the ideal, issue open to discuss adding
  // an introspection endpoint that would give us this mapping
  // https://github.com/openshift/origin/issues/230
  var SERVER_TYPE_MAP = {
    builds : apicfg.openshift,
    deploymentConfigs : apicfg.openshift,
    images : apicfg.openshift,
    projects : apicfg.openshift,
    pods : apicfg.k8s,
    services : apicfg.k8s,
    replicationControllers: apicfg.k8s
  };

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
      serverUrl: SERVER_TYPE_MAP[type].hostPort,
      apiPrefix: SERVER_TYPE_MAP[type].prefix,
      apiVersion: SERVER_TYPE_MAP[type].version,
      type: type,
      id: id,
      q: params
    });
  };

  return new DataService();
}]);