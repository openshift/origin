'use strict';

angular.module('openshiftConsole')
.factory('DataService', function($http, $ws, $rootScope, $q, API_CFG, Notification, Logger) {

  function Data(array) {
    this._data = {};
    this._objectsByAttribute(array, "metadata.name", this._data);
  }

  Data.prototype.by = function(attr) {
    // TODO store already generated indices
    if (attr === "metadata.name") {
      return this._data;
    }
    var map = {};
    for (var key in this._data) {
      _objectByAttribute(this._data[key], attr, map, null);
    }
    return map;
  };

  Data.prototype.update = function(object, action) {
    _objectByAttribute(object, "metadata.name", this._data, action);
  };


  // actions is whether the object was (ADDED|DELETED|MODIFIED).  ADDED is assumed if actions is not
  // passed.  If objects is a hash then actions must be a hash with the same keys.  If objects is an array
  // then actions must be an array of the same order and length.
  Data.prototype._objectsByAttribute = function(objects, attr, map, actions) {
    angular.forEach(objects, function(obj, key) {
      _objectByAttribute(obj, attr, map, actions ? actions[key] : null);
    });
  };

  // Handles attr with dot notation
  // TODO write lots of tests for this helper
  // Note: this lives outside the Data prototype for now so it can be used by the helper in DataService as well
  var _objectByAttribute = function(obj, attr, map, action) {
    var subAttrs = attr.split(".");
    var attrValue = obj;
    for (var i = 0; i < subAttrs.length; i++) {
      attrValue = attrValue[subAttrs[i]];
      if (attrValue === undefined) {
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
        if (action === "DELETED") {
          delete map[key][val];
        }
        else {
          map[key][val] = obj;
        }
      }
    }
    else {
      if (action === "DELETED") {
        delete map[attrValue];
      }
      else {
        map[attrValue] = obj;
      }
    }
  };

  var normalizeType = function(type) {
     if (!type) return type;
     var lower = type.toLowerCase();
     if (type !== lower) {
       Logger.warn('Non-lower case type "' + type + '"');
     }

     return lower;
  }

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
    this._watchWebsocketRetriesMap = {};

    var self = this;
    $rootScope.$on( "$routeChangeStart", function(event, next, current) {
      self._watchWebsocketRetriesMap = {};
    });

    this.osApiVersion = "v1beta3";
    this.k8sApiVersion = "v1beta3";

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
    type = normalizeType(type);
    var callbacks = this._listCallbacks(type, context);
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
// opts:      http - options to pass to the inner $http call
// Returns a promise resolved with response data or rejected with {data:..., status:..., headers:..., config:...} when the delete call completes.
  DataService.prototype.delete = function(type, name, context, opts) {
    type = normalizeType(type);
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(type, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'DELETE',
        url: self._urlForType(type, name, context, false, ns)
      }, opts.http || {}))
      .success(function(data, status, headerFunc, config, statusText) {
        deferred.resolve(data);
      })
      .error(function(data, status, headers, config) {
        deferred.reject({
          data: data,
          status: status,
          headers: headers,
          config: config
        });
      });
    });
    return deferred.promise;
  };

// type:      API type (e.g. "pods")
// name:      API name, the unique name for the object.
//            In case the name of the Object is provided, expected format of 'type' parameter is 'type/subresource', eg: 'buildconfigs/instantiate'.
// object:    API object data(eg. { kind: "Build", parameters: { ... } } )
// context:   API context (e.g. {project: "..."})
// opts:      http - options to pass to the inner $http call
// Returns a promise resolved with response data or rejected with {data:..., status:..., headers:..., config:...} when the delete call completes.
  DataService.prototype.create = function(type, name, object, context, opts) {
    type = normalizeType(type);
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(type, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'POST',
        data: object,
        url: self._urlForType(type, name, context, false, ns)
      }, opts.http || {}))
      .success(function(data, status, headerFunc, config, statusText) {
        deferred.resolve(data);
      })
      .error(function(data, status, headers, config) {
        deferred.reject({
          data: data,
          status: status,
          headers: headers,
          config: config
        });
      });
    });
    return deferred.promise;
  };

  // objects:   Array of API object data(eg. [{ kind: "Build", parameters: { ... } }] )
  // context:   API context (e.g. {project: "..."})
  // opts:      http - options to pass to the inner $http call
  // Returns a promise resolved with an an object like: { success: [], failure: [] }
  // where success and failure contain an array of results from the individual
  // create calls.
  DataService.prototype.createList = function(objects, context, opts) {
    var result = $q.defer();
    var successResults = [];
    var failureResults = [];
    var self = this;
    var remaining = objects.length;

    function _checkDone() {
      if (remaining === 0) {
        result.resolve({ success: successResults, failure: failureResults });
      }
    }

    objects.forEach(function(object) {
      self.create(self._objectType(object.kind), null, object, context, opts).then(
        function (data) {
          successResults.push(data);
          remaining--;
          _checkDone();
        },
        function (data) {
          failureResults.push(data);
          remaining--;
          _checkDone();
        }
      );
    });
    return result.promise;
  };

// type:      API type (e.g. "pods")
// name:      API name, the unique name for the object
// context:   API context (e.g. {project: "..."})
// opts:      force - always request (default is false)
//            http - options to pass to the inner $http call
//            errorNotification - will popup an error notification if the API request fails (default true)
  DataService.prototype.get = function(type, name, context, opts) {
    if(this._objectType(type) !== undefined){
      type = this._objectType(type);
    } else {
      type = normalizeType(type);
    }
    opts = opts || {};

    var force = !!opts.force;
    delete opts.force;

    var deferred = $q.defer();

    var existingData = this._data(type, context);

    // If this is a cached type (immutable types only), ignore the force parameter
    if (this._isTypeCached(type) && existingData && existingData.by('metadata.name')[name]) {
      deferred.resolve(existingData.by('metadata.name')[name]);
    }
    else if (!force && this._watchInFlight(type, context) && this._resourceVersion(type, context)) {
      var obj = existingData.by('metadata.name')[name];
      if (obj) {
        $rootScope.$apply(function(){
          deferred.resolve(obj);
        });
      }
      else {
        $rootScope.$apply(function(){
          // simulation of API object not found
          deferred.reject({
            data: {},
            status: 404,
            headers: function() { return null; },
            config: {}
          });
        });
      }
    }
    else {
      var self = this;
      this._getNamespace(type, context, opts).then(function(ns){
        $http(angular.extend({
          method: 'GET',
          url: self._urlForType(type, name, context, false, ns)
        }, opts.http || {}))
        .success(function(data, status, headerFunc, config, statusText) {
          if (self._isTypeCached(type)) {
            if (!existingData) {
              self._data(type, context, [data]);
            }
            else {
              existingData.update(data, "ADDED");
            }
          }
          deferred.resolve(data);
        })
        .error(function(data, status, headers, config) {
          if (opts.errorNotification !== false) {
            var msg = "Failed to get " + type + "/" + name;
            if (status !== 0) {
              msg += " (" + status + ")"
            }
            Notification.error(msg);
          }
          deferred.reject({
            data: data,
            status: status,
            headers: headers,
            config: config
          });
        });
      });
    }
    return deferred.promise;
  };

// type:      API type (e.g. "pods")
// context:   API context (e.g. {project: "..."})
// callback:  optional function to be called with the initial list of the requested type,
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
    type = normalizeType(type);
    opts = opts || {};

    if (callback) {
      // If we were given a callback, add it
      this._watchCallbacks(type, context).add(callback);
    }
    else if (!this._watchCallbacks(type, context).has()) {
      // We can be called with no callback in order to re-run a list/watch sequence for existing callbacks
      // If there are no existing callbacks, return
      return {};
    }

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
      if (callback) {
        callback(this._data(type, context));
      }
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
    if (callback) {
      callbacks.remove(callback);
    }
    if (!callbacks.has()) {
      if (opts && opts.poll) {
        clearTimeout(this._watchPollTimeouts(type, context));
        this._watchPollTimeouts(type, context, null);
      }
      else if (this._watchWebsockets(type, context)){
        // watchWebsockets may not have been set up yet if the projectPromise never resolves
        var ws = this._watchWebsockets(type, context);
        // Make sure the onclose listener doesn't reopen this websocket.
        ws.shouldClose = true;
        ws.close();
        this._watchWebsockets(type, context, null);
      }

      this._watchInFlight(type, context, false);
      this._watchOptions(type, context, null);
    }
  };

  // Takes an array of watch handles and unwatches them
  DataService.prototype.unwatchAll = function(handles) {
    for (var i = 0; i < handles.length; i++) {
      this.unwatch(handles[i]);
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

  DataService.prototype._watchWebsocketRetries = function(type, context, retry) {
    var key = this._uniqueKeyForTypeContext(type, context);
    if (retry === undefined) {
      return this._watchWebsocketRetriesMap[key];
    }
    else {
      this._watchWebsocketRetriesMap[key] = retry;
    }
  };

  DataService.prototype._uniqueKeyForTypeContext = function(type, context) {
    // Note: when we start handling selecting multiple projects this
    // will change to include all relevant scope
    return type + "/" + context.projectName;
  };

  DataService.prototype._startListOp = function(type, context) {
    // mark the operation as in progress
    this._listInFlight(type, context, true);

    var self = this;
    if (context.projectPromise && type !== "projects") {
      context.projectPromise.done(function(project) {
        $http({
          method: 'GET',
          url: self._urlForType(type, null, context, false, {namespace: project.metadata.name})
        }).success(function(data, status, headerFunc, config, statusText) {
          self._listOpComplete(type, context, data);
        }).error(function(data, status, headers, config) {
          var msg = "Failed to list " + type;
          if (status !== 0) {
            msg += " (" + status + ")"
          }
          // TODO would like to make this optional with an errorNotification option, see get for an example
          Notification.error(msg);
        });
      });
    }
    else {
      $http({
        method: 'GET',
        url: this._urlForType(type, null, context),
      }).success(function(data, status, headerFunc, config, statusText) {
        self._listOpComplete(type, context, data);
      }).error(function(data, status, headers, config) {
        var msg = "Failed to list " + type;
        if (status !== 0) {
          msg += " (" + status + ")"
        }
        // TODO would like to make this optional with an errorNotification option, see get for an example
        Notification.error(msg);
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
    if ($ws.available()) {
      var self = this;
      var params = {};
      if (resourceVersion) {
        params.resourceVersion = resourceVersion;
      }
      if (context.projectPromise && type !== "projects") {
        context.projectPromise.done(function(project) {
          params.namespace = project.metadata.name;
          $ws({
            method: "WATCH",
            url: self._urlForType(type, null, context, true, params),
            onclose: $.proxy(self, "_watchOpOnClose", type, context),
            onmessage: $.proxy(self, "_watchOpOnMessage", type, context),
            onopen: $.proxy(self, "_watchOpOnOpen", type, context)
          }).then(function(ws) {
            Logger.log("Watching", ws);
            self._watchWebsockets(type, context, ws);
          });
        });
      }
      else {
        $ws({
          method: "WATCH",
          url: self._urlForType(type, null, context, true, params),
          onclose: $.proxy(self, "_watchOpOnClose", type, context),
          onmessage: $.proxy(self, "_watchOpOnMessage", type, context),
          onopen: $.proxy(self, "_watchOpOnOpen", type, context)
        }).then(function(ws){
          Logger.log("Watching", ws);
          self._watchWebsockets(type, context, ws);
        });
      }
    }
  };

  DataService.prototype._watchOpOnOpen = function(type, context, event) {
    // If we opened the websocket cleanly, set retries to 0
    this._watchWebsocketRetries(type, context, 0);
  };

  DataService.prototype._watchOpOnMessage = function(type, context, event) {
    try {
      var eventData = $.parseJSON(event.data);

      if (eventData.type == "ERROR") {
        Logger.log("Watch window expired for type/context", type, context);
        if (event.target) {
          event.target.shouldRelist = true;
        }
        return;
      }

      this._resourceVersion(type, context, eventData.object.resourceVersion || eventData.object.metadata.resourceVersion);
      // TODO do we reset all the by() indices, or simply update them, since we should know what keys are there?
      // TODO let the data object handle its own update
      this._data(type, context).update(eventData.object, eventData.type);
      var self = this;
      // Wrap in $apply to mirror $http callback behavior
      $rootScope.$apply(function() {
        self._watchCallbacks(type, context).fire(self._data(type, context), eventData.type, eventData.object);
      });
    }
    catch (e) {
      // TODO report the JSON parsing exception
    }
  };

  DataService.prototype._watchOpOnClose = function(type, context, event) {
    var eventWS = event.target;
    if (!eventWS) {
      Logger.log("Skipping reopen, no eventWS in event", event);
      return;
    }

    var registeredWS = this._watchWebsockets(type, context);
    if (!registeredWS) {
      Logger.log("Skipping reopen, no registeredWS for type/context", type, context);
      return;
    }

    // Don't reopen a web socket that is no longer registered for this type/context
    if (eventWS !== registeredWS) {
      Logger.log("Skipping reopen, eventWS does not match registeredWS", eventWS, registeredWS);
      return;
    }

    // We are the registered web socket for this type/context, and we are no longer in flight
    // Unlock this type/context in case we decide not to reopen
    this._watchInFlight(type, context, false);

    // Don't reopen web sockets we closed ourselves
    if (eventWS.shouldClose) {
      Logger.log("Skipping reopen, eventWS was explicitly closed", eventWS);
      return;
    }

    // Don't reopen clean closes (for example, navigating away from the page to example.com)
    if (event.wasClean) {
      Logger.log("Skipping reopen, clean close", event);
      return;
    }

    // Don't reopen if no one is listening for this data any more
    if (!this._watchCallbacks(type, context).has()) {
      Logger.log("Skipping reopen, no listeners registered for type/context", type, context);
      return;
    }

    // Don't reopen if we've failed this type/context 5+ times in a row
    var retries = this._watchWebsocketRetries(type, context) || 0;
    if (retries >= 5) {
      Logger.log("Skipping reopen, already retried type/context 5+ times", type, context, retries);
      Notification.error("Server connection interrupted.", {
        id: "websocket_retry_halted",
        mustDismiss: true,
        actions: {
          refresh: {label: "Refresh", action: function() { window.location.reload(); }}
        }
      });
      return;
    }

    // Keep track of this failure
    this._watchWebsocketRetries(type, context, retries + 1);

    // If our watch window expired, we have to relist to get a new resource version to watch from
    if (eventWS.shouldRelist) {
      Logger.log("Relisting for type/context", type, context);
      // Restart a watch() from the beginning, which triggers a list/watch sequence
      // The watch() call is responsible for setting _watchInFlight back to true
      this.watch(type, context);
      return;
    }

    // Attempt to re-establish the connection after a one second back-off
    // Re-mark ourselves as in-flight to prevent other callers from jumping in in the meantime
    Logger.log("Rewatching for type/context", type, context);
    this._watchInFlight(type, context, true);
    setTimeout(
      $.proxy(this, "_startWatchOp", type, context, this._resourceVersion(type, context)),
      1000
    );
  };

  var URL_ROOT_TEMPLATE = "{protocol}://{+serverUrl}{+apiPrefix}/{apiVersion}/";
  var URL_WATCH_LIST = URL_ROOT_TEMPLATE + "watch/{type}{?q*}";
  var URL_GET_LIST = URL_ROOT_TEMPLATE + "{type}{?q*}";
  var URL_GET_OBJECT = URL_ROOT_TEMPLATE + "{type}/{id}{?q*}";
  var URL_OBJECT_SUBRESOURCE = URL_ROOT_TEMPLATE + "{type}/{id}/{subresource}{?q*}";
  var URL_NAMESPACED_WATCH_LIST = URL_ROOT_TEMPLATE + "watch/namespaces/{namespace}/{type}{?q*}";
  var URL_NAMESPACED_GET_LIST = URL_ROOT_TEMPLATE + "namespaces/{namespace}/{type}{?q*}";
  var URL_NAMESPACED_GET_OBJECT = URL_ROOT_TEMPLATE + "namespaces/{namespace}/{type}/{id}{?q*}";
  var URL_NAMESPACED_OBJECT_SUBRESOURCE = URL_ROOT_TEMPLATE + "namespaces/{namespace}/{type}/{id}/{subresource}{?q*}";
  // TODO is there a better way to get this template instead of building it, introspection?
  var BUILD_HOOKS_URL = URL_ROOT_TEMPLATE + "{type}/{id}/{secret}/{hookType}{?q*}";

  // Set the api version the console is currently able to talk to
  API_CFG.openshift.version = "v1beta3";
  API_CFG.k8s.version = "v1beta3";

  // Set whether namespace is a path or query parameter
  API_CFG.openshift.namespacePath = true;
  API_CFG.k8s.namespacePath = true;

  // TODO this is not the ideal, issue open to discuss adding
  // an introspection endpoint that would give us this mapping
  // https://github.com/openshift/origin/issues/230
  var SERVER_TYPE_MAP = {
    builds:                    API_CFG.openshift,
    buildconfigs:              API_CFG.openshift,
    buildconfighooks:          API_CFG.openshift,
    deploymentconfigs:         API_CFG.openshift,
    imagestreams:              API_CFG.openshift,
    imagestreamimages:         API_CFG.openshift,
    imagestreamtags:           API_CFG.openshift,
    oauthaccesstokens:         API_CFG.openshift,
    oauthauthorizetokens:      API_CFG.openshift,
    oauthclients:              API_CFG.openshift,
    oauthclientauthorizations: API_CFG.openshift,
    policies:                  API_CFG.openshift,
    policybindings:            API_CFG.openshift,
    processedtemplates:        API_CFG.openshift,
    projects:                  API_CFG.openshift,
    projectrequests:           API_CFG.openshift,
    roles:                     API_CFG.openshift,
    rolebindings:              API_CFG.openshift,
    routes:                    API_CFG.openshift,
    templates:                 API_CFG.openshift,
    users:                     API_CFG.openshift,

    events:                    API_CFG.k8s,
    pods:                      API_CFG.k8s,
    replicationcontrollers:    API_CFG.k8s,
    services:                  API_CFG.k8s,
    resourcequotas:            API_CFG.k8s,
    limitranges:               API_CFG.k8s
  };

  DataService.prototype._urlForType = function(type, id, context, isWebsocket, params) {

    // Parse the type parameter for type itself and subresource. Example: 'buildconfigs/instantiate'
    if(type.indexOf('/') !== -1){
      var typeWithSubresource = type.split("/");
      var type = typeWithSubresource[0];
      var subresource = typeWithSubresource[1];
    }

    var protocol;
    params = params || {};
    if (isWebsocket) {
      protocol = window.location.protocol === "http:" ? "ws" : "wss";
    }
    else {
      protocol = window.location.protocol === "http:" ? "http" : "https";
    }

    if (context && context.namespace && !params.namespace) {
      params.namespace = context.namespace;
    }

    var namespaceInPath = params.namespace && SERVER_TYPE_MAP[type].namespacePath;
    var namespace = null;
    if (namespaceInPath) {
      namespace = params.namespace;
      params = angular.copy(params);
      delete params.namespace;
    }
    var template;
    var templateOptions = {
      protocol: protocol,
      serverUrl: SERVER_TYPE_MAP[type].hostPort,
      apiPrefix: SERVER_TYPE_MAP[type].prefix,
      apiVersion: SERVER_TYPE_MAP[type].version,
      type: type,
      id: id,
      namespace: namespace
    };
    if (isWebsocket) {
      template = namespaceInPath ? URL_NAMESPACED_WATCH_LIST : URL_WATCH_LIST;
    }
    else if (id) {
      if (type === "buildconfighooks") {
        templateOptions.secret = params.secret;
        templateOptions.hookType = params.hookType;
        params = angular.copy(params);
        delete params.secret;
        delete params.hookType;
        template = BUILD_HOOKS_URL;
      }
      else if (subresource) {
        templateOptions.subresource = subresource;
        template = namespaceInPath ? URL_NAMESPACED_OBJECT_SUBRESOURCE : URL_OBJECT_SUBRESOURCE;
      }
      else
      {
        template = namespaceInPath ? URL_NAMESPACED_GET_OBJECT : URL_GET_OBJECT;
      }
    }
    else {
      template = namespaceInPath ? URL_NAMESPACED_GET_LIST : URL_GET_LIST;
    }

    templateOptions.q = params;
    return URI.expand(template, templateOptions);
  };

  DataService.prototype.url = function(options) {
    if (options && options.type) {
      var opts = angular.copy(options);
      delete opts.type;
      delete opts.id;
      var type = normalizeType(options.type);
      return this._urlForType(type, options.id, null, false, opts).toString();
    }
    return null;
  };

  var OBJECT_KIND_MAP = {
    Build:                    "builds",
    BuildConfig:              "buildconfigs",
    DeploymentConfig:         "deploymentconfigs",
    ImageStream:              "imagestreams",
    OAuthAccessToken:         "oauthaccesstokens",
    OAuthAuthorizeToken:      "oauthauthorizetokens",
    OAuthClient:              "oauthclients",
    OAuthClientAuthorization: "oauthclientauthorizations",
    Policy:                   "policies",
    PolicyBinding:            "policybindings",
    Project:                  "projects",
    Role:                     "roles",
    RoleBinding:              "rolebindings",
    Route:                    "routes",
    User:                     "users",

    Pod:                      "pods",
    ReplicationController:    "replicationcontrollers",
    Service:                  "services",
    ResourceQuota:            "resourcequotas",
    LimitRange:               "limitranges"
  };

  DataService.prototype._objectType = function(kind) {
    return OBJECT_KIND_MAP[kind];
  };

  var CACHED_TYPE = {
    imageStreamImages: true
  };

  DataService.prototype._isTypeCached = function(type) {
    return !!CACHED_TYPE[type];
  };

  DataService.prototype._getNamespace = function(type, context, opts) {
    var deferred = $q.defer();
    if (opts.namespace) {
      deferred.resolve({namespace: opts.namespace});
    }
    else if (context.projectPromise && type !== "projects") {
      context.projectPromise.done(function(project) {
        deferred.resolve({namespace: project.metadata.name});
      });
    }
    else {
      deferred.resolve(null);
    }
    return deferred.promise;
  };

  return new DataService();
});
