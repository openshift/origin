'use strict';
/* jshint eqeqeq: false, unused: false, expr: true */

angular.module('openshiftConsole')
.factory('DataService', function($http, $ws, $rootScope, $q, APIService,  Notification, Logger, $timeout) {

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
    var resourcePath = APIService.qualifyResource(resource).resource;
    var callbacks = this._listCallbacks(resourcePath, context);
    callbacks.add(callback);

    if (this._watchInFlight(resourcePath, context) && this._resourceVersion(resourcePath, context)) {
      // A watch operation is running, and we've already received the
      // initial set of data for this resource
      callbacks.fire(this._data(resourcePath, context));
      callbacks.empty();
    }
    else if (this._listInFlight(resourcePath, context)) {
      // no-op, our callback will get called when listOperation completes
    }
    else {
      this._startListOp(resourcePath, context);
    }
  };

// resource:  API resource (e.g. "pods")
// name:      API name, the unique name for the object
// context:   API context (e.g. {project: "..."})
// opts:      http - options to pass to the inner $http call
// Returns a promise resolved with response data or rejected with {data:..., status:..., headers:..., config:...} when the delete call completes.
  DataService.prototype.delete = function(resource, name, context, opts) {
    // resource = APIService.normalizeResource(resource);
    var resourcePath = APIService.qualifyResource(resource).resource;
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(resourcePath, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'DELETE',
        auth: {},
        url: APIService.urlForResource(resource, name, null, context, false, ns)
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
    // resource = APIService.normalizeResource(resource);
    var resourcePath = APIService.qualifyResource(resource).resource;
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(resourcePath, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'PUT',
        auth: {},
        data: object,
        url: APIService.urlForResource(resource, name, object.apiVersion, context, false, ns)
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
    // resource = APIService.normalizeResource(resource);
    var resourcePath = APIService.qualifyResource(resource).resource;
    opts = opts || {};
    var deferred = $q.defer();
    var self = this;
    this._getNamespace(resourcePath, context, opts).then(function(ns){
      $http(angular.extend({
        method: 'POST',
        auth: {},
        data: object,
        url: APIService.urlForResource(resource, name, object.apiVersion, context, false, ns)
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
      var resourcePath = APIService.kindToResource(object.kind);
      var derived = APIService.deriveResource(object);

      if (!resourcePath) {
        failureResults.push({
          data: {message: "Unrecognized kind " + object.kind}
        });
        remaining--;
        _checkDone();
        return;
      }

      if (!APIService.apiExistsFor(derived, object.apiVersion)) {

        failureResults.push({
          data: {message: 'Kind ' + object.kind + ' is invalid for API version ' + object.apiVersion},
        });
        remaining--;
        _checkDone();
        return;
      }

      self.create(derived, null, object, context, opts).then(
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
    // resource = APIService.normalizeResource(resource);
    var resourcePath = APIService.qualifyResource(resource).resource;
    opts = opts || {};

    var force = !!opts.force;
    delete opts.force;

    var deferred = $q.defer();

    var existingData = this._data(resourcePath, context);

    // If this is a cached resource (immutable resources only), ignore the force parameter
    if (this._isResourceCached(resourcePath) && existingData && existingData.by('metadata.name')[name]) {
      $timeout(function() {
        deferred.resolve(existingData.by('metadata.name')[name]);
      }, 0);
    }
    else if (!force && this._watchInFlight(resourcePath, context) && this._resourceVersion(resourcePath, context)) {
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
      this._getNamespace(resourcePath, context, opts).then(function(ns){
        $http(angular.extend({
          method: 'GET',
          auth: {},
          url: APIService.urlForResource(resource, name, null, context, false, ns)
        }, opts.http || {}))
        .success(function(data, status, headerFunc, config, statusText) {
          if (self._isResourceCached(resourcePath)) {
            if (!existingData) {
              self._data(resourcePath, context, [data]);
            }
            else {
              existingData.update(data, "ADDED");
            }
          }
          deferred.resolve(data);
        })
        .error(function(data, status, headers, config) {
          if (opts.errorNotification !== false) {
            var msg = "Failed to get " + resourcePath + "/" + name;
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
DataService.prototype.createStream = function(kind, name, context, opts, isRaw) {
  var getNamespace = this._getNamespace.bind(this);
  var resourcePath = APIService.normalizeResource(kind);
  var protocols = isRaw ? 'binary.k8s.io' : 'base64.binary.k8s.io';
  var identifier = 'stream_';
  var openQueue = {};
  var messageQueue = {};
  var closeQueue = {};
  var errorQueue = {};

  var stream;
  var makeStream = function() {
     return getNamespace(resourcePath, context, {})
                .then(function(params) {
                  var cumulativeBytes = 0;
                  return  $ws({
                            url: APIService.urlForResource(resourcePath, name, null, context, true, _.extend(params, opts)),
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

                              var message;
                              if(!isRaw) {
                                message = b64_to_utf8(evt.data);
                                // Count bytes for log streams, which will stop when limitBytes is reached.
                                // There's no other way to detect we've reach the limit currently.
                                cumulativeBytes += message.length;
                              }

                              _.each(messageQueue, function(fn) {
                                if(isRaw) {
                                  fn(evt.data);
                                } else {
                                  fn(message, evt.data, cumulativeBytes);
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
    // resource = APIService.normalizeResource(resource);
    var resourcePath = APIService.qualifyResource(resource).resource;
    opts = opts || {};

    if (callback) {
      // If we were given a callback, add it
      this._watchCallbacks(resourcePath, context).add(callback);
    }
    else if (!this._watchCallbacks(resourcePath, context).has()) {
      // We can be called with no callback in order to re-run a list/watch sequence for existing callbacks
      // If there are no existing callbacks, return
      return {};
    }

    var existingWatchOpts = this._watchOptions(resourcePath, context);
    if (existingWatchOpts) {
      // Check any options for compatibility with existing watch
      if (existingWatchOpts.poll != opts.poll) {
        throw "A watch already exists for " + resourcePath + " with a different polling option.";
      }
    }
    else {
      this._watchOptions(resourcePath, context, opts);
    }

    var self = this;

    if (this._watchInFlight(resourcePath, context) && this._resourceVersion(resourcePath, context)) {
      if (callback) {
        $timeout(function() {
          callback(self._data(resourcePath, context));
        }, 0);
      }
    }
    else {
      if (callback) {
        var existingData = this._data(resourcePath, context);
        if (existingData) {
          $timeout(function() {
            callback(existingData);
          }, 0);
        }
      }
      if (!this._listInFlight(resourcePath, context)) {
        this._startListOp(resource, context);
      }
    }

    // returned handle needs resource, context, and callback in order to unwatch
    return {
      resource: resourcePath,
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
    // resource = APIService.normalizeResource(resource);
    var resourcePath = APIService.qualifyResource(resource).resource;
    opts = opts || {};

    var wrapperCallback;
    if (callback) {
      // If we were given a callback, add it
      this._watchObjectCallbacks(resourcePath, name, context).add(callback);
      var self = this;
      wrapperCallback = function(items, event, item) {
        // If we got an event for a single item, only fire the callback if its the item we care about
        if (item && item.metadata.name === name) {
          self._watchObjectCallbacks(resourcePath, name, context).fire(item, event);
        }
        else if (!item) {
          // Otherwise its an initial listing, see if we can find the item we care about in the list
          var itemsByName = items.by("metadata.name");
          if (itemsByName[name]) {
            self._watchObjectCallbacks(resourcePath, name, context).fire(itemsByName[name]);
          }
        }
      };
    }
    else if (!this._watchObjectCallbacks(resourcePath, name, context).has()) {
      // This block may not be needed yet, don't expect this would get called without a callback currently...
      return {};
    }

    // For now just watch the type, eventually we may want to do something more complicated
    // and watch just the object if the type is not already being watched
    var handle = this.watch(resourcePath, context, wrapperCallback, opts);
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

  DataService.prototype._watchCallbacks = function(resourcePath, context) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!this._watchCallbacksMap[key]) {
      this._watchCallbacksMap[key] = $.Callbacks();
    }
    return this._watchCallbacksMap[key];
  };

  DataService.prototype._watchObjectCallbacks = function(resourcePath, name, context) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context) + "/" + name;
    if (!this._watchObjectCallbacksMap[key]) {
      this._watchObjectCallbacksMap[key] = $.Callbacks();
    }
    return this._watchObjectCallbacksMap[key];
  };

  DataService.prototype._listCallbacks = function(resourcePath, context) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!this._listCallbacksMap[key]) {
      this._listCallbacksMap[key] = $.Callbacks();
    }
    return this._listCallbacksMap[key];
  };

  // maybe change these
  DataService.prototype._watchInFlight = function(resourcePath, context, op) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!op && op !== false) {
      return this._watchOperationMap[key];
    }
    else {
      this._watchOperationMap[key] = op;
    }
  };

  DataService.prototype._listInFlight = function(resourcePath, context, op) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!op && op !== false) {
      return this._listOperationMap[key];
    }
    else {
      this._listOperationMap[key] = op;
    }
  };

  DataService.prototype._resourceVersion = function(resourcePath, context, rv) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!rv) {
      return this._resourceVersionMap[key];
    }
    else {
      this._resourceVersionMap[key] = rv;
    }
  };

  DataService.prototype._data = function(resourcePath, context, data) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!data) {
      return this._dataMap[key];
    }
    else {
      this._dataMap[key] = new Data(data);
    }
  };

  DataService.prototype._watchOptions = function(resourcePath, context, opts) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (opts === undefined) {
      return this._watchOptionsMap[key];
    }
    else {
      this._watchOptionsMap[key] = opts;
    }
  };

  DataService.prototype._watchPollTimeouts = function(resourcePath, context, timeout) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!timeout) {
      return this._watchPollTimeoutsMap[key];
    }
    else {
      this._watchPollTimeoutsMap[key] = timeout;
    }
  };

  DataService.prototype._watchWebsockets = function(resourcePath, context, timeout) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    if (!timeout) {
      return this._watchWebsocketsMap[key];
    }
    else {
      this._watchWebsocketsMap[key] = timeout;
    }
  };

  // Maximum number of websocket events to track per resource/context in _websocketEventsMap.
  var maxWebsocketEvents = 10;

  DataService.prototype._addWebsocketEvent = function(resourcePath, context, eventType) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
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

  DataService.prototype._isTooManyWebsocketRetries = function(resourcePath, context) {
    var key = this._uniqueKeyForResourceContext(resourcePath, context);
    var events = this._websocketEventsMap[key];
    if (!events) {
      return false;
    }

    if (isTooManyRecentEvents(events)) {
      Logger.log("Too many websocket open or close events for resource/context in a short period", resourcePath, context, events);
      return true;
    }

    if (isTooManyConsecutiveCloses(events)) {
      Logger.log("Too many consecutive websocket close events for resource/context", resourcePath, context, events);
      return true;
    }

    return false;
  };

  DataService.prototype._uniqueKeyForResourceContext = function(resourcePath, context) {
    // Note: when we start handling selecting multiple projects this
    // will change to include all relevant scope
    if (resourcePath === "projects" || resourcePath === "projectrequests") { // when we are loading non-namespaced resources we don't need additional context
      return resourcePath;
    }
    if(!context) {
      return resourcePath;
    }
    else if (context.namespace) {
      return resourcePath + "/" + context.namespace;
    }
    else if (context.project && context.project.metadata) {
      return resourcePath + "/" + context.project.metadata.name;
    }
    else if (context.projectName) {
      return resourcePath + "/" + context.projectName;
    }
    else {
      return resourcePath;
    }
  };

  DataService.prototype._startListOp = function(resource, context) {
    var resourcePath = APIService.qualifyResource(resource).resource;
    // mark the operation as in progress
    this._listInFlight(resourcePath, context, true);

    var self = this;
    if (context.projectPromise && resource !== "projects") {
      context.projectPromise.done(function(project) {
        $http({
          method: 'GET',
          auth: {},
          url: APIService.urlForResource(resource, null, null, context, false, {namespace: project.metadata.name})
        }).success(function(data, status, headerFunc, config, statusText) {
          self._listOpComplete(resource, context, data);
        }).error(function(data, status, headers, config) {
          var msg = "Failed to list " + resourcePath;
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
        url: APIService.urlForResource(resource, null, null, context),
      }).success(function(data, status, headerFunc, config, statusText) {
        self._listOpComplete(resource, context, data);
      }).error(function(data, status, headers, config) {
        var msg = "Failed to list " + resourcePath;
        if (status !== 0) {
          msg += " (" + status + ")";
        }
        // TODO would like to make this optional with an errorNotification option, see get for an example
        Notification.error(msg);
      });
    }
  };

  DataService.prototype._listOpComplete = function(resource, context, data) {
    var resourcePath = APIService.qualifyResource(resource).resource;
    // Here we normalize all items to have a kind property.
    // One of the warts in the kubernetes REST API is that items retrieved
    // via GET on a list resource won't have a kind property set.
    // See: https://github.com/kubernetes/kubernetes/issues/3030
    if (data.kind && data.kind.indexOf("List") === data.kind.length - 4) {
      angular.forEach(data.items, function(item) {
        if (!item.kind) {
          item.kind = data.kind.slice(0, -4);
        }
        if (!item.apiVersion) {
          item.apiVersion = data.apiVersion;
        }
      });
    }

    this._resourceVersion(resourcePath, context, data.resourceVersion || data.metadata.resourceVersion);
    this._data(resourcePath, context, data.items);
    this._listCallbacks(resourcePath, context).fire(this._data(resourcePath, context));
    this._listCallbacks(resourcePath, context).empty();
    this._watchCallbacks(resourcePath, context).fire(this._data(resourcePath, context));

    // mark list op as complete
    this._listInFlight(resourcePath, context, false);

    if (this._watchCallbacks(resourcePath, context).has()) {
      var watchOpts = this._watchOptions(resourcePath, context) || {};
      if (watchOpts.poll) {
        this._watchInFlight(resourcePath, context, true);
        this._watchPollTimeouts(resourcePath, context, setTimeout($.proxy(this, "_startListOp", resourcePath, context), watchOpts.pollInterval || 5000));
      }
      else if (!this._watchInFlight(resourcePath, context)) {
        this._startWatchOp(resource, context, this._resourceVersion(resourcePath, context));
      }
    }
  };

  DataService.prototype._startWatchOp = function(resource, context, resourceVersion) {
    var resourcePath = APIService.qualifyResource(resource).resource;
    this._watchInFlight(resourcePath, context, true);
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
      if (context.projectPromise && resourcePath !== "projects") {
        context.projectPromise.done(function(project) {
          params.namespace = project.metadata.name;
          $ws({
            method: "WATCH",
            url: APIService.urlForResource(resource, null, null, context, true, params),
            auth:      {},
            onclose:   $.proxy(self, "_watchOpOnClose",   resourcePath, context),
            onmessage: $.proxy(self, "_watchOpOnMessage", resourcePath, context),
            onopen:    $.proxy(self, "_watchOpOnOpen",    resourcePath, context)
          }).then(function(ws) {
            Logger.log("Watching", ws);
            self._watchWebsockets(resourcePath, context, ws);
          });
        });
      }
      else {
        $ws({
          method: "WATCH",
          url: APIService.urlForResource(resource, null, null, context, true, params),
          auth:      {},
          onclose:   $.proxy(self, "_watchOpOnClose",   resourcePath, context),
          onmessage: $.proxy(self, "_watchOpOnMessage", resourcePath, context),
          onopen:    $.proxy(self, "_watchOpOnOpen",    resourcePath, context)
        }).then(function(ws){
          Logger.log("Watching", ws);
          self._watchWebsockets(resourcePath, context, ws);
        });
      }
    }
  };

  DataService.prototype._watchOpOnOpen = function(resourcePath, context, event) {
    Logger.log('Websocket opened for resource/context', resourcePath, context);
    this._addWebsocketEvent(resourcePath, context, 'open');
  };

  DataService.prototype._watchOpOnMessage = function(resourcePath, context, event) {
    try {
      var eventData = $.parseJSON(event.data);

      if (eventData.type == "ERROR") {
        Logger.log("Watch window expired for resource/context", resourcePath, context);
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
        this._resourceVersion(resourcePath, context, eventData.object.resourceVersion || eventData.object.metadata.resourceVersion);
      }
      // TODO do we reset all the by() indices, or simply update them, since we should know what keys are there?
      // TODO let the data object handle its own update
      this._data(resourcePath, context).update(eventData.object, eventData.type);
      var self = this;
      // Wrap in a $timeout which will trigger an $apply to mirror $http callback behavior
      // without timeout this is triggering a repeated digest loop
      $timeout(function() {
        self._watchCallbacks(resourcePath, context).fire(self._data(resourcePath, context), eventData.type, eventData.object);
      }, 0);
    }
    catch (e) {
      // TODO: surface in the UI?
      Logger.error("Error processing message", resourcePath, event.data);
    }
  };

  DataService.prototype._watchOpOnClose = function(resourcePath, context, event) {
    var eventWS = event.target;
    if (!eventWS) {
      Logger.log("Skipping reopen, no eventWS in event", event);
      return;
    }

    var registeredWS = this._watchWebsockets(resourcePath, context);
    if (!registeredWS) {
      Logger.log("Skipping reopen, no registeredWS for resource/context", resourcePath, context);
      return;
    }

    // Don't reopen a web socket that is no longer registered for this resource/context
    if (eventWS !== registeredWS) {
      Logger.log("Skipping reopen, eventWS does not match registeredWS", eventWS, registeredWS);
      return;
    }

    // We are the registered web socket for this resource/context, and we are no longer in flight
    // Unlock this resource/context in case we decide not to reopen
    this._watchInFlight(resourcePath, context, false);

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
    if (!this._watchCallbacks(resourcePath, context).has()) {
      Logger.log("Skipping reopen, no listeners registered for resource/context", resourcePath, context);
      return;
    }

    // Don't reopen if we've failed this resource/context too many times
    if (this._isTooManyWebsocketRetries(resourcePath, context)) {
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
    this._addWebsocketEvent(resourcePath, context, 'close');

    // If our watch window expired, we have to relist to get a new resource version to watch from
    if (eventWS.shouldRelist) {
      Logger.log("Relisting for resource/context", resourcePath, context);
      // Restart a watch() from the beginning, which triggers a list/watch sequence
      // The watch() call is responsible for setting _watchInFlight back to true
      // Add a short delay to avoid a scenario where we make non-stop requests
      // When the timeout fires, if no callbacks are registered for this
      //   resource/context, or if a watch is already in flight, `watch()` is a no-op
      var self = this;
      setTimeout(function() {
        self.watch(resourcePath, context);
      }, 2000);
      return;
    }

    // Attempt to re-establish the connection after a two-second back-off
    // Re-mark ourselves as in-flight to prevent other callers from jumping in in the meantime
    Logger.log("Rewatching for resource/context", resourcePath, context);
    this._watchInFlight(resourcePath, context, true);
    setTimeout(
      $.proxy(this, "_startWatchOp", resourcePath, context, this._resourceVersion(resourcePath, context)),
      2000
    );
  };

  var CACHED_RESOURCE = {
    imagestreamimages: true
  };

  DataService.prototype._isResourceCached = function(resourcePath) {
    return !!CACHED_RESOURCE[resourcePath];
  };

  DataService.prototype._getNamespace = function(resourcePath, context, opts) {
    var deferred = $q.defer();
    if (opts.namespace) {
      deferred.resolve({namespace: opts.namespace});
    }
    else if (context.projectPromise && resourcePath !== "projects") {
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
