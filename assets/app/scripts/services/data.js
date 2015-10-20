'use strict';
/* jshint eqeqeq: false, unused: false, expr: true */

angular.module('openshiftConsole')
.factory('DataService', function($http, $ws, $rootScope, $q, API_CFG, Notification, Logger, $timeout) {

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

  var normalizeResource = function(resource) {
     if (!resource) {
      return resource;
     }

     // only lowercase the first segment, leaving subresources as-is (some are case-sensitive)
     var segments = resource.split("/");
     segments[0] = segments[0].toLowerCase();
     var normalized = segments.join("/");
     if (resource !== normalized) {
       Logger.warn('Non-lower case resource "' + resource + '"');
     }

     return normalized;
  };

  function DataService() {
    this._listCallbacksMap = {};
    this._watchCallbacksMap = {};
    this._watchObjectCallbacksMap = {};
    this._watchOperationMap = {};
    this._listOperationMap = {};
    this._resourceVersionMap = {};
    this._dataMap = {};
    this._watchOptionsMap = {};
    this._watchWebsocketsMap = {};
    this._watchPollTimeoutsMap = {};
    this._websocketEventsMap = {};

    var self = this;
    $rootScope.$on( "$routeChangeStart", function(event, next, current) {
      self._websocketEventsMap = {};
    });

    this.oApiVersion = "v1";
    this.k8sApiVersion = "v1";

  }

// resource:  API resource (e.g. "pods")
// context:   API context (e.g. {project: "..."})
// callback:  function to be called with the list of the requested resource and context,
//            parameters passed to the callback:
//            Data:   a Data object containing the (context-qualified) results
//                    which includes a helper method for returning a map indexed
//                    by attribute (e.g. data.by('metadata.name'))
// opts:      options (currently none, placeholder)
  DataService.prototype.list = function(resource, context, callback, opts) {
    resource = normalizeResource(resource);
    var callbacks = this._listCallbacks(resource, context);
    callbacks.add(callback);

    if (this._watchInFlight(resource, context) && this._resourceVersion(resource, context)) {
      // A watch operation is running, and we've already received the
      // initial set of data for this resource
      callbacks.fire(this._data(resource, context));
      callbacks.empty();
    }
    else if (this._listInFlight(resource, context)) {
      // no-op, our callback will get called when listOperation completes
    }
    else {
      this._startListOp(resource, context);
    }
  };

// resource:  API resource (e.g. "pods")
// name:      API name, the unique name for the object
// context:   API context (e.g. {project: "..."})
// opts:      http - options to pass to the inner $http call
// Returns a promise resolved with response data or rejected with {data:..., status:..., headers:..., config:...} when the delete call completes.
  DataService.prototype.delete = function(resource, name, context, opts) {
    resource = normalizeResource(resource);
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(resource, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'DELETE',
        auth: {},
        url: self._urlForResource(resource, name, null, context, false, ns)
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

// resource:  API resource (e.g. "pods")
// name:      API name, the unique name for the object
// object:    API object data(eg. { kind: "Build", parameters: { ... } } )
// context:   API context (e.g. {project: "..."})
// opts:      http - options to pass to the inner $http call
// Returns a promise resolved with response data or rejected with {data:..., status:..., headers:..., config:...} when the delete call completes.
  DataService.prototype.update = function(resource, name, object, context, opts) {
    resource = normalizeResource(resource);
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(resource, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'PUT',
        auth: {},
        data: object,
        url: self._urlForResource(resource, name, object.apiVersion, context, false, ns)
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

// resource:  API resource (e.g. "pods")
// name:      API name, the unique name for the object.
//            In case the name of the Object is provided, expected format of 'resource' parameter is 'resource/subresource', eg: 'buildconfigs/instantiate'.
// object:    API object data(eg. { kind: "Build", parameters: { ... } } )
// context:   API context (e.g. {project: "..."})
// opts:      http - options to pass to the inner $http call
// Returns a promise resolved with response data or rejected with {data:..., status:..., headers:..., config:...} when the delete call completes.
  DataService.prototype.create = function(resource, name, object, context, opts) {
    resource = normalizeResource(resource);
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(resource, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'POST',
        auth: {},
        data: object,
        url: self._urlForResource(resource, name, object.apiVersion, context, false, ns)
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
      var resource = self.kindToResource(object.kind);
      if (!resource) {
        failureResults.push({
          data: {message: "Unrecognized kind " + object.kind}
        });
        remaining--;
        _checkDone();
        return;
      }

      var resourceInfo = self.resourceInfo(resource, object.apiVersion);
      if (!resourceInfo) {
        failureResults.push({
          data: {message: "Unknown API version "+object.apiVersion+" for kind " + object.kind}
        });
        remaining--;
        _checkDone();
        return;
      }

      self.create(resource, null, object, context, opts).then(
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

// resource:  API resource (e.g. "pods")
// name:      API name, the unique name for the object
// context:   API context (e.g. {project: "..."})
// opts:      force - always request (default is false)
//            http - options to pass to the inner $http call
//            errorNotification - will popup an error notification if the API request fails (default true)
  DataService.prototype.get = function(resource, name, context, opts) {
    resource = normalizeResource(resource);
    opts = opts || {};

    var force = !!opts.force;
    delete opts.force;

    var deferred = $q.defer();

    var existingData = this._data(resource, context);

    // If this is a cached resource (immutable resources only), ignore the force parameter
    if (this._isResourceCached(resource) && existingData && existingData.by('metadata.name')[name]) {
      $timeout(function() {
        deferred.resolve(existingData.by('metadata.name')[name]);
      }, 0);
    }
    else if (!force && this._watchInFlight(resource, context) && this._resourceVersion(resource, context)) {
      var obj = existingData.by('metadata.name')[name];
      if (obj) {
        $timeout(function() {
          deferred.resolve(obj);
        }, 0);
      }
      else {
        $timeout(function() {
          // simulation of API object not found
          deferred.reject({
            data: {},
            status: 404,
            headers: function() { return null; },
            config: {}
          });
        }, 0);
      }
    }
    else {
      var self = this;
      this._getNamespace(resource, context, opts).then(function(ns){
        $http(angular.extend({
          method: 'GET',
          auth: {},
          url: self._urlForResource(resource, name, null, context, false, ns)
        }, opts.http || {}))
        .success(function(data, status, headerFunc, config, statusText) {
          if (self._isResourceCached(resource)) {
            if (!existingData) {
              self._data(resource, context, [data]);
            }
            else {
              existingData.update(data, "ADDED");
            }
          }
          deferred.resolve(data);
        })
        .error(function(data, status, headers, config) {
          if (opts.errorNotification !== false) {
            var msg = "Failed to get " + resource + "/" + name;
            if (status !== 0) {
              msg += " (" + status + ")";
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

// https://developer.mozilla.org/en-US/docs/Web/API/WindowBase64/btoa
function utf8_to_b64( str ) {
    return window.btoa(window.unescape(encodeURIComponent( str )));
}
function b64_to_utf8( str ) {
    return decodeURIComponent(window.escape(window.atob( str )));
}

// TODO (bpeterse): Create a new Streamer service & get this out of DataService.
DataService.prototype.createStream = function(kind, name, context, isRaw) {
  var getNamespace = this._getNamespace.bind(this);
  var urlForResource = this._urlForResource.bind(this);
  kind = this.kindToResource(kind) ?
              this.kindToResource(kind) :
              normalizeResource(kind);

  var protocols = isRaw ? 'binary.k8s.io' : 'base64.binary.k8s.io';
  var identifier = 'stream_';
  var openQueue = {};
  var messageQueue = {};
  var closeQueue = {};
  var errorQueue = {};

  var stream;
  var makeStream = function() {
     return getNamespace(kind, context, {})
                .then(function(params) {
                  return  $ws({
                            url: urlForResource(kind, name, null, context, true, _.extend(params, {follow: true})),
                            auth: {},
                            onopen: function(evt) {
                              _.each(openQueue, function(fn) {
                                fn(evt);
                              });
                            },
                            onmessage: function(evt) {
                              if(!_.isString(evt.data)) {
                                Logger.log('log stream response is not a string', evt.data);
                                return;
                              }
                              _.each(messageQueue, function(fn) {
                                if(isRaw) {
                                  fn(evt.data);
                                } else {
                                  fn(b64_to_utf8(evt.data), evt.data);
                                }
                              });
                            },
                            onclose: function(evt) {
                              _.each(closeQueue, function(fn) {
                                fn(evt);
                              });
                            },
                            onerror: function(evt) {
                              _.each(errorQueue, function(fn) {
                                fn(evt);
                              });
                            },
                            protocols: protocols
                          }).then(function(ws) {
                            Logger.log("Streaming pod log", ws);
                            return ws;
                          });
                });
  };
  return {
    onOpen: function(fn) {
      if(!_.isFunction(fn)) {
        return;
      }
      var id = _.uniqueId(identifier);
      openQueue[id] = fn;
      return id;
    },
    onMessage: function(fn) {
      if(!_.isFunction(fn)) {
        return;
      }
      var id = _.uniqueId(identifier);
      messageQueue[id] = fn;
      return id;
    },
    onClose: function(fn) {
      if(!_.isFunction(fn)) {
        return;
      }
      var id = _.uniqueId(identifier);
      closeQueue[id] = fn;
      return id;
    },
    onError: function(fn) {
      if(!_.isFunction(fn)) {
        return;
      }
      var id = _.uniqueId(identifier);
      errorQueue[id] = fn;
      return id;
    },
    // can remove any callback from open, message, close or error
    remove: function(id) {
      if (openQueue[id]) { delete openQueue[id]; }
      if (messageQueue[id]) { delete messageQueue[id]; }
      if (closeQueue[id]) { delete closeQueue[id]; }
      if (errorQueue[id]) { delete errorQueue[id]; }
    },
    start: function() {
      stream = makeStream();
      return stream;
    },
    stop: function() {
      stream.then(function(ws) {
        ws.close();
      });
    }
  };
};


// resource:  API resource (e.g. "pods")
// context:   API context (e.g. {project: "..."})
// callback:  optional function to be called with the initial list of the requested resource,
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
//        var handle = DataService.watch(resource,context,callback[,opts])
//        DataService.unwatch(handle)
  DataService.prototype.watch = function(resource, context, callback, opts) {
    resource = normalizeResource(resource);
    opts = opts || {};

    if (callback) {
      // If we were given a callback, add it
      this._watchCallbacks(resource, context).add(callback);
    }
    else if (!this._watchCallbacks(resource, context).has()) {
      // We can be called with no callback in order to re-run a list/watch sequence for existing callbacks
      // If there are no existing callbacks, return
      return {};
    }

    var existingWatchOpts = this._watchOptions(resource, context);
    if (existingWatchOpts) {
      // Check any options for compatibility with existing watch
      if (existingWatchOpts.poll != opts.poll) {
        throw "A watch already exists for " + resource + " with a different polling option.";
      }
    }
    else {
      this._watchOptions(resource, context, opts);
    }

    var self = this;

    if (this._watchInFlight(resource, context) && this._resourceVersion(resource, context)) {
      if (callback) {
        $timeout(function() {
          callback(self._data(resource, context));
        }, 0);
      }
    }
    else {
      if (callback) {
        var existingData = this._data(resource, context);
        if (existingData) {
          $timeout(function() {
            callback(existingData);
          }, 0);
        }
      }
      if (!this._listInFlight(resource, context)) {
        this._startListOp(resource, context);
      }
    }

    // returned handle needs resource, context, and callback in order to unwatch
    return {
      resource: resource,
      context: context,
      callback: callback,
      opts: opts
    };
  };



// resource:  API resource (e.g. "pods")
// name:      API name, the unique name for the object
// context:   API context (e.g. {project: "..."})
// callback:  optional function to be called with the initial list of the requested resource,
//            and when updates are received, parameters passed to the callback:
//            obj:    the requested object
//            event:  specific event that caused this call ("ADDED", "MODIFIED",
//                    "DELETED", or null) callbacks can optionally use this to
//                    more efficiently process updates
// opts:      options
//            poll:   true | false - whether to poll the server instead of opening
//                    a websocket. Default is false.
//            pollInterval: in milliseconds, how long to wait between polling the server
//                    only applies if poll=true.  Default is 5000.
//
// returns handle to the watch, needed to unwatch e.g.
//        var handle = DataService.watch(resource,context,callback[,opts])
//        DataService.unwatch(handle)
  DataService.prototype.watchObject = function(resource, name, context, callback, opts) {
    resource = normalizeResource(resource);
    opts = opts || {};

    var wrapperCallback;
    if (callback) {
      // If we were given a callback, add it
      this._watchObjectCallbacks(resource, name, context).add(callback);
      var self = this;
      wrapperCallback = function(items, event, item) {
        // If we got an event for a single item, only fire the callback if its the item we care about
        if (item && item.metadata.name === name) {
          self._watchObjectCallbacks(resource, name, context).fire(item, event);
        }
        else {
          // Otherwise see if we can find the item we care about in the list
          var itemsByName = items.by("metadata.name");
          if (itemsByName[name]) {
            self._watchObjectCallbacks(resource, name, context).fire(itemsByName[name], event);
          }
        }
      };
    }
    else if (!this._watchObjectCallbacks(resource, name, context).has()) {
      // This block may not be needed yet, don't expect this would get called without a callback currently...
      return {};
    }

    // For now just watch the type, eventually we may want to do something more complicated
    // and watch just the object if the type is not already being watched
    var handle = this.watch(resource, context, wrapperCallback, opts);
    handle.objectCallback = callback;
    handle.objectName = name;

    return handle;
  };

  DataService.prototype.unwatch = function(handle) {
    var resource = handle.resource;
    var objectName = handle.objectName;
    var context = handle.context;
    var callback = handle.callback;
    var objectCallback = handle.objectCallback;
    var opts = handle.opts;

    if (objectCallback && objectName) {
      var objCallbacks = this._watchObjectCallbacks(resource, objectName, context);
      objCallbacks.remove(objectCallback);
    }

    var callbacks = this._watchCallbacks(resource, context);
    if (callback) {
      callbacks.remove(callback);
    }
    if (!callbacks.has()) {
      if (opts && opts.poll) {
        clearTimeout(this._watchPollTimeouts(resource, context));
        this._watchPollTimeouts(resource, context, null);
      }
      else if (this._watchWebsockets(resource, context)){
        // watchWebsockets may not have been set up yet if the projectPromise never resolves
        var ws = this._watchWebsockets(resource, context);
        // Make sure the onclose listener doesn't reopen this websocket.
        ws.shouldClose = true;
        ws.close();
        this._watchWebsockets(resource, context, null);
      }

      this._watchInFlight(resource, context, false);
      this._watchOptions(resource, context, null);
    }
  };

  // Takes an array of watch handles and unwatches them
  DataService.prototype.unwatchAll = function(handles) {
    for (var i = 0; i < handles.length; i++) {
      this.unwatch(handles[i]);
    }
  };

  DataService.prototype._watchCallbacks = function(resource, context) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!this._watchCallbacksMap[key]) {
      this._watchCallbacksMap[key] = $.Callbacks();
    }
    return this._watchCallbacksMap[key];
  };

  DataService.prototype._watchObjectCallbacks = function(resource, name, context) {
    var key = this._uniqueKeyForResourceContext(resource, context) + "/" + name;
    if (!this._watchObjectCallbacksMap[key]) {
      this._watchObjectCallbacksMap[key] = $.Callbacks();
    }
    return this._watchObjectCallbacksMap[key];
  };

  DataService.prototype._listCallbacks = function(resource, context) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!this._listCallbacksMap[key]) {
      this._listCallbacksMap[key] = $.Callbacks();
    }
    return this._listCallbacksMap[key];
  };

  // maybe change these
  DataService.prototype._watchInFlight = function(resource, context, op) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!op && op !== false) {
      return this._watchOperationMap[key];
    }
    else {
      this._watchOperationMap[key] = op;
    }
  };

  DataService.prototype._listInFlight = function(resource, context, op) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!op && op !== false) {
      return this._listOperationMap[key];
    }
    else {
      this._listOperationMap[key] = op;
    }
  };

  DataService.prototype._resourceVersion = function(resource, context, rv) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!rv) {
      return this._resourceVersionMap[key];
    }
    else {
      this._resourceVersionMap[key] = rv;
    }
  };

  DataService.prototype._data = function(resource, context, data) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!data) {
      return this._dataMap[key];
    }
    else {
      this._dataMap[key] = new Data(data);
    }
  };

  DataService.prototype._watchOptions = function(resource, context, opts) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (opts === undefined) {
      return this._watchOptionsMap[key];
    }
    else {
      this._watchOptionsMap[key] = opts;
    }
  };

  DataService.prototype._watchPollTimeouts = function(resource, context, timeout) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!timeout) {
      return this._watchPollTimeoutsMap[key];
    }
    else {
      this._watchPollTimeoutsMap[key] = timeout;
    }
  };

  DataService.prototype._watchWebsockets = function(resource, context, timeout) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    if (!timeout) {
      return this._watchWebsocketsMap[key];
    }
    else {
      this._watchWebsocketsMap[key] = timeout;
    }
  };

  // Maximum number of websocket events to track per resource/context in _websocketEventsMap.
  var maxWebsocketEvents = 10;

  DataService.prototype._addWebsocketEvent = function(resource, context, eventType) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    var events = this._websocketEventsMap[key];
    if (!events) {
      events = this._websocketEventsMap[key] = [];
    }

    // Add the event to the end of the array with the time in millis.
    events.push({
      type: eventType,
      time: Date.now()
    });

    // Only keep 10 events. Shift the array to make room for the new event.
    while (events.length > maxWebsocketEvents) { events.shift(); }
  };

  function isTooManyRecentEvents(events) {
    // If we've had more than 10 events in 30 seconds, stop.
    // The oldest event is at index 0.
    var recentDuration = 1000 * 30;
    return events.length >= maxWebsocketEvents && (Date.now() - events[0].time) < recentDuration;
  }

  function isTooManyConsecutiveCloses(events) {
    var maxConsecutiveCloseEvents = 5;
    if (events.length < maxConsecutiveCloseEvents) {
      return false;
    }

    // Make sure the last 5 events were not close events, which means the
    // connection is not succeeding. This check is necessary if connection
    // timeouts take longer than 6 seconds.
    for (var i = events.length - maxConsecutiveCloseEvents; i < events.length; i++) {
      if (events[i].type !== 'close') {
        return false;
      }
    }

    return true;
  }

  DataService.prototype._isTooManyWebsocketRetries = function(resource, context) {
    var key = this._uniqueKeyForResourceContext(resource, context);
    var events = this._websocketEventsMap[key];
    if (!events) {
      return false;
    }

    if (isTooManyRecentEvents(events)) {
      Logger.log("Too many websocket open or close events for resource/context in a short period", resource, context, events);
      return true;
    }

    if (isTooManyConsecutiveCloses(events)) {
      Logger.log("Too many consecutive websocket close events for resource/context", resource, context, events);
      return true;
    }

    return false;
  };

  DataService.prototype._uniqueKeyForResourceContext = function(resource, context) {
    // Note: when we start handling selecting multiple projects this
    // will change to include all relevant scope
    if (resource === "projects" || resource === "projectrequests") { // when we are loading non-namespaced resources we don't need additional context
      return resource;
    }
    else if (context.namespace) {
      return resource + "/" + context.namespace;
    }
    else if (context.project && context.project.metadata) {
      return resource + "/" + context.project.metadata.name;
    }
    else if (context.projectName) {
      return resource + "/" + context.projectName;
    }
    else {
      return resource;
    }
  };

  DataService.prototype._startListOp = function(resource, context) {
    // mark the operation as in progress
    this._listInFlight(resource, context, true);

    var self = this;
    if (context.projectPromise && resource !== "projects") {
      context.projectPromise.done(function(project) {
        $http({
          method: 'GET',
          auth: {},
          url: self._urlForResource(resource, null, null, context, false, {namespace: project.metadata.name})
        }).success(function(data, status, headerFunc, config, statusText) {
          self._listOpComplete(resource, context, data);
        }).error(function(data, status, headers, config) {
          var msg = "Failed to list " + resource;
          if (status !== 0) {
            msg += " (" + status + ")";
          }
          // TODO would like to make this optional with an errorNotification option, see get for an example
          Notification.error(msg);
        });
      });
    }
    else {
      $http({
        method: 'GET',
        auth: {},
        url: this._urlForResource(resource, null, null, context),
      }).success(function(data, status, headerFunc, config, statusText) {
        self._listOpComplete(resource, context, data);
      }).error(function(data, status, headers, config) {
        var msg = "Failed to list " + resource;
        if (status !== 0) {
          msg += " (" + status + ")";
        }
        // TODO would like to make this optional with an errorNotification option, see get for an example
        Notification.error(msg);
      });
    }
  };

  DataService.prototype._listOpComplete = function(resource, context, data) {
    // Here we normalize all items to have a kind property.
    // One of the warts in the kubernetes REST API is that items retrieved
    // via GET on a list resource won't have a kind property set.
    // See: https://github.com/kubernetes/kubernetes/issues/3030
    if (data.kind && data.kind.indexOf("List") === data.kind.length - 4) {
      angular.forEach(data.items, function(item) {
        if (!item.kind) {
          item.kind = data.kind.slice(0, -4);
        }
      });
    }

    this._resourceVersion(resource, context, data.resourceVersion || data.metadata.resourceVersion);
    this._data(resource, context, data.items);
    this._listCallbacks(resource, context).fire(this._data(resource, context));
    this._listCallbacks(resource, context).empty();
    this._watchCallbacks(resource, context).fire(this._data(resource, context));

    // mark list op as complete
    this._listInFlight(resource, context, false);

    if (this._watchCallbacks(resource, context).has()) {
      var watchOpts = this._watchOptions(resource, context) || {};
      if (watchOpts.poll) {
        this._watchInFlight(resource, context, true);
        this._watchPollTimeouts(resource, context, setTimeout($.proxy(this, "_startListOp", resource, context), watchOpts.pollInterval || 5000));
      }
      else if (!this._watchInFlight(resource, context)) {
        this._startWatchOp(resource, context, this._resourceVersion(resource, context));
      }
    }
  };

  DataService.prototype._startWatchOp = function(resource, context, resourceVersion) {
    this._watchInFlight(resource, context, true);
    // Note: current impl uses one websocket per resource
    // eventually want a single websocket connection that we
    // send a subscription request to for each resource

    // Only listen for updates if websockets are available
    if ($ws.available()) {
      var self = this;
      var params = {};
      params.watch = true;
      if (resourceVersion) {
        params.resourceVersion = resourceVersion;
      }
      if (context.projectPromise && resource !== "projects") {
        context.projectPromise.done(function(project) {
          params.namespace = project.metadata.name;
          $ws({
            method: "WATCH",
            url: self._urlForResource(resource, null, null, context, true, params),
            auth:      {},
            onclose:   $.proxy(self, "_watchOpOnClose",   resource, context),
            onmessage: $.proxy(self, "_watchOpOnMessage", resource, context),
            onopen:    $.proxy(self, "_watchOpOnOpen",    resource, context)
          }).then(function(ws) {
            Logger.log("Watching", ws);
            self._watchWebsockets(resource, context, ws);
          });
        });
      }
      else {
        $ws({
          method: "WATCH",
          url: self._urlForResource(resource, null, null, context, true, params),
          auth:      {},
          onclose:   $.proxy(self, "_watchOpOnClose",   resource, context),
          onmessage: $.proxy(self, "_watchOpOnMessage", resource, context),
          onopen:    $.proxy(self, "_watchOpOnOpen",    resource, context)
        }).then(function(ws){
          Logger.log("Watching", ws);
          self._watchWebsockets(resource, context, ws);
        });
      }
    }
  };

  DataService.prototype._watchOpOnOpen = function(resource, context, event) {
    Logger.log('Websocket opened for resource/context', resource, context);
    this._addWebsocketEvent(resource, context, 'open');
  };

  DataService.prototype._watchOpOnMessage = function(resource, context, event) {
    try {
      var eventData = $.parseJSON(event.data);

      if (eventData.type == "ERROR") {
        Logger.log("Watch window expired for resource/context", resource, context);
        if (event.target) {
          event.target.shouldRelist = true;
        }
        return;
      }
      else if (eventData.type === "DELETED") {
        // Add this ourselves since the API doesn't add anything
        // this way the views can use it to trigger special behaviors
        if (eventData.object && eventData.object.metadata && !eventData.object.metadata.deletionTimestamp) {
          eventData.object.metadata.deletionTimestamp = (new Date()).toISOString();
        }
      }

      if (eventData.object) {
        this._resourceVersion(resource, context, eventData.object.resourceVersion || eventData.object.metadata.resourceVersion);
      }
      // TODO do we reset all the by() indices, or simply update them, since we should know what keys are there?
      // TODO let the data object handle its own update
      this._data(resource, context).update(eventData.object, eventData.type);
      var self = this;
      // Wrap in a $timeout which will trigger an $apply to mirror $http callback behavior
      // without timeout this is triggering a repeated digest loop
      $timeout(function() {
        self._watchCallbacks(resource, context).fire(self._data(resource, context), eventData.type, eventData.object);
      }, 0);
    }
    catch (e) {
      // TODO: surface in the UI?
      Logger.error("Error processing message", resource, event.data);
    }
  };

  DataService.prototype._watchOpOnClose = function(resource, context, event) {
    var eventWS = event.target;
    if (!eventWS) {
      Logger.log("Skipping reopen, no eventWS in event", event);
      return;
    }

    var registeredWS = this._watchWebsockets(resource, context);
    if (!registeredWS) {
      Logger.log("Skipping reopen, no registeredWS for resource/context", resource, context);
      return;
    }

    // Don't reopen a web socket that is no longer registered for this resource/context
    if (eventWS !== registeredWS) {
      Logger.log("Skipping reopen, eventWS does not match registeredWS", eventWS, registeredWS);
      return;
    }

    // We are the registered web socket for this resource/context, and we are no longer in flight
    // Unlock this resource/context in case we decide not to reopen
    this._watchInFlight(resource, context, false);

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
    if (!this._watchCallbacks(resource, context).has()) {
      Logger.log("Skipping reopen, no listeners registered for resource/context", resource, context);
      return;
    }

    // Don't reopen if we've failed this resource/context too many times
    if (this._isTooManyWebsocketRetries(resource, context)) {
      Notification.error("Server connection interrupted.", {
        id: "websocket_retry_halted",
        mustDismiss: true,
        actions: {
          refresh: {label: "Refresh", action: function() { window.location.reload(); }}
        }
      });
      return;
    }

    // Keep track of this event.
    this._addWebsocketEvent(resource, context, 'close');

    // If our watch window expired, we have to relist to get a new resource version to watch from
    if (eventWS.shouldRelist) {
      Logger.log("Relisting for resource/context", resource, context);
      // Restart a watch() from the beginning, which triggers a list/watch sequence
      // The watch() call is responsible for setting _watchInFlight back to true
      // Add a short delay to avoid a scenario where we make non-stop requests
      // When the timeout fires, if no callbacks are registered for this
      //   resource/context, or if a watch is already in flight, `watch()` is a no-op
      var self = this;
      setTimeout(function() {
        self.watch(resource, context);
      }, 2000);
      return;
    }

    // Attempt to re-establish the connection after a two-second back-off
    // Re-mark ourselves as in-flight to prevent other callers from jumping in in the meantime
    Logger.log("Rewatching for resource/context", resource, context);
    this._watchInFlight(resource, context, true);
    setTimeout(
      $.proxy(this, "_startWatchOp", resource, context, this._resourceVersion(resource, context)),
      2000
    );
  };

  var URL_ROOT_TEMPLATE         = "{protocol}://{+serverUrl}{+apiPrefix}/{apiVersion}/";
  var URL_GET_LIST              = URL_ROOT_TEMPLATE + "{resource}{?q*}";
  var URL_OBJECT                = URL_ROOT_TEMPLATE + "{resource}/{name}{/subresource*}{?q*}";
  var URL_NAMESPACED_GET_LIST   = URL_ROOT_TEMPLATE + "namespaces/{namespace}/{resource}{?q*}";
  var URL_NAMESPACED_OBJECT     = URL_ROOT_TEMPLATE + "namespaces/{namespace}/{resource}/{name}{/subresource*}{?q*}";

  // Set the default api versions the console will use if otherwise unspecified
  API_CFG.openshift.defaultVersion = "v1";
  API_CFG.k8s.defaultVersion = "v1";

  DataService.prototype._urlForResource = function(resource, name, apiVersion, context, isWebsocket, params) {

    var resourceWithSubresource;
    var subresource;
    // Parse the resource parameter for resource itself and subresource. Examples:
    //    buildconfigs/instantiate
    //    buildconfigs/webhooks/mysecret/github
    if(resource.indexOf('/') !== -1){
      resourceWithSubresource = resource.split("/");
      // first segment is the resource
      resource = resourceWithSubresource.shift();
      // all remaining segments are the subresource
      subresource = resourceWithSubresource;
    }

    var resourceInfo = this.resourceInfo(resource, apiVersion);
    if (!resourceInfo) {
      Logger.error("_urlForResource called with unknown resource", resource, arguments);
      return null;
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

    var namespaceInPath = params.namespace;
    var namespace = null;
    if (namespaceInPath) {
      namespace = params.namespace;
      params = angular.copy(params);
      delete params.namespace;
    }
    var template;
    var templateOptions = {
      protocol: protocol,
      serverUrl: resourceInfo.hostPort,
      apiPrefix: resourceInfo.prefix,
      apiVersion: resourceInfo.apiVersion,
      resource: resource,
      subresource: subresource,
      name: name,
      namespace: namespace,
      q: params
    };
    if (name) {
      template = namespaceInPath ? URL_NAMESPACED_OBJECT : URL_OBJECT;
    }
    else {
      template = namespaceInPath ? URL_NAMESPACED_GET_LIST : URL_GET_LIST;
    }
    return URI.expand(template, templateOptions);
  };

  DataService.prototype.url = function(options) {
    if (options && options.resource) {
      var opts = angular.copy(options);
      delete opts.resource;
      delete opts.name;
      delete opts.apiVersion;
      delete opts.isWebsocket;
      var resource = normalizeResource(options.resource);
      var u = this._urlForResource(resource, options.name, options.apiVersion, null, !!options.isWebsocket, opts);
      if (u) {
        return u.toString();
      }
    }
    return null;
  };

  DataService.prototype.openshiftAPIBaseUrl = function() {
    var protocol = window.location.protocol === "http:" ? "http" : "https";
    var hostPort = API_CFG.openshift.hostPort;
    return new URI({protocol: protocol, hostname: hostPort}).toString();
  };

  DataService.prototype.resourceInfo = function(resource, preferredAPIVersion) {
    var api, apiVersion, prefix;
    for (var apiName in API_CFG) {
      api = API_CFG[apiName];
      if (!api.resources[resource] && !api.resources['*']) {
        continue;
      }
      apiVersion = preferredAPIVersion || api.defaultVersion;
      prefix = api.prefixes[apiVersion] || api.prefixes['*'];
      if (!prefix) {
        continue;
      }
      return {
      	hostPort:   api.hostPort,
      	prefix:     prefix,
      	apiVersion: apiVersion
      };
    }
    return undefined;
  };

  // port of restmapper.go#kindToResource
  DataService.prototype.kindToResource = function(kind) {
    if (!kind) {
      return "";
    }
    var resource = String(kind).toLowerCase();
    if (resource.endsWith('status')) {
      resource = resource + 'es';
    }
    else if (resource.endsWith('s')) {
      // no-op
    }
    else if (resource.endsWith('y')) {
      resource = resource.substring(0, resource.length-1) + 'ies';
    }
    else {
      resource = resource + 's';
    }

    // make sure it is a known resource
    if (!this.resourceInfo(resource)) {
      Logger.warn('Unknown resource "' + resource + '"');
      return undefined;
    }
    return resource;
  };

  var CACHED_RESOURCE = {
    imagestreamimages: true
  };

  DataService.prototype._isResourceCached = function(resource) {
    return !!CACHED_RESOURCE[resource];
  };

  DataService.prototype._getNamespace = function(resource, context, opts) {
    var deferred = $q.defer();
    if (opts.namespace) {
      deferred.resolve({namespace: opts.namespace});
    }
    else if (context.projectPromise && resource !== "projects") {
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
