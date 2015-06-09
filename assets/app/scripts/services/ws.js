// Provide a websocket implementation that behaves like $http
// Methods:
//   $ws({
//     url: "...", // required
//     method: "...", // defaults to WATCH
//   })
//   returns a promise to the opened WebSocket
// 
//   $ws.available()
//   returns true if WebSockets are available to use
angular.module('openshiftConsole')
.provider('$ws', function($httpProvider) {

  var debug = false;

  // $get method is called to build the $ws service
  this.$get = function($q, $injector, Logger) {
    var authLogger = Logger.get("auth");
    authLogger.log("$wsProvider.$get", arguments);

    // Build list of interceptors from $httpProvider when constructing the $ws service
    // Build in reverse-order, so the last interceptor added gets to handle the request first
    var _interceptors = [];
    angular.forEach($httpProvider.interceptors, function(interceptorFactory) {
      if (angular.isString(interceptorFactory)) {
      	_interceptors.unshift($injector.get(interceptorFactory));
      } else {
      	_interceptors.unshift($injector.invoke(interceptorFactory));
      }
    });

    // Implement $ws()
    var $ws = function(config) {
      config.method = angular.uppercase(config.method || "WATCH");

      authLogger.log("$ws (pre-intercept)", config.url.toString());
      var serverRequest = function(config) {
        authLogger.log("$ws (post-intercept)", config.url.toString());
        var ws = new WebSocket(config.url);
        if (config.onclose)   { ws.onclose   = config.onclose;   }
        if (config.onmessage) { ws.onmessage = config.onmessage; }
        if (config.onopen)    { ws.onopen    = config.onopen;    }
        return ws;
      };

      // Apply interceptors to request config
      var chain = [serverRequest, undefined];
      var promise = $q.when(config);
      angular.forEach(_interceptors, function(interceptor) {
        if (interceptor.request || interceptor.requestError) {
          chain.unshift(interceptor.request, interceptor.requestError);
        }
        // TODO: figure out how to get interceptors to handle response errors from web sockets
        // if (interceptor.response || interceptor.responseError) {
        //   chain.push(interceptor.response, interceptor.responseError);
        // }
      });
      while (chain.length) {
        var thenFn = chain.shift();
        var rejectFn = chain.shift();
        promise = promise.then(thenFn, rejectFn);
      }
      return promise;
    };

    // Implement $ws.available()
    $ws.available = function() {
      try {
        return !!WebSocket;
      }
      catch(e) {
        return false;
      }
    };

    return $ws;
  };
})
