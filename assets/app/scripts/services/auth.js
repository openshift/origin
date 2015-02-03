angular.module('openshiftConsole')
// In a config step, set the desired user store and login service. For example:
//   AuthServiceProvider.setUserStore('SessionUserStore')
//   AuthServiceProvider.setLoginService('RedirectLoginService')
//
// AuthService provides the following functions:
//   withUser()
//     returns a promise that resolves when there is a current user
//     starts a login if there is no current user
//   setUser(user, token)
//     sets the current user and token to use for authenticated requests
//     triggers onUserChanged callbacks if the new user is different than the current user
//   requestRequiresAuth(config)
//     returns true if the request is to a protected URL
//   addAuthToRequest(config)
//     adds auth info to the request, if available
//     if specified, uses config.auth.token as the token, otherwise uses the token store
//   startLogin()
//     returns a promise that is resolved when the login is complete
//   onLogin(callback)
//     the given callback is called whenever a login is completed
//   onUserChanged(callback)
//     the given callback is called whenever the current user changes
.provider('AuthService', function() {
  var debug = true;

  if (debug) { console.log('AuthServiceProvider()'); }

  var _userStore = "";
  this.UserStore = function(userStoreName) {
    if (userStoreName) {
      _userStore = userStoreName;
    }
    return _userStore;
  };
  var _loginService = "";
  this.LoginService = function(loginServiceName) {
    if (loginServiceName) {
      _loginService = loginServiceName;
    }
    return _loginService;
  };

  this.$get = function($q, $injector, $rootScope) {
    if (debug) { console.log('AuthServiceProvider.$get', arguments); }

    var _loginCallbacks = $.Callbacks();
    var _userChangedCallbacks = $.Callbacks();
  
    var _loginPromise = null;
  
    var userStore;
    if (!_userStore) {
      throw "AuthServiceProvider.UserStore() not set";
    } else if (angular.isString(_userStore)) {
      userStore = $injector.get(_userStore);
    } else {
      userStore = $injector.invoke(_userStore);
    }

    var loginService;
    if (!_loginService) {
      throw "AuthServiceProvider.LoginService() not set";
    } else if (angular.isString(_loginService)) {
      loginService = $injector.get(_loginService);
    } else {
      loginService = $injector.invoke(_loginService);
    }

    return {
      withUser: function() {
        var user = userStore.getUser();
        if (user) {
          $rootScope.user = user;
          if (debug) { console.log('AuthService.withUser()', user); }
          return $q.when(user);
        } else {
          if (debug) { console.log('AuthService.withUser(), calling startLogin()'); }
          return this.startLogin();
        }
      },
  
      setUser: function(user, token) {
        if (debug) { console.log('AuthService.setUser()', user, token); }
        var oldUser = userStore.getUser();
        userStore.setUser(user);
        userStore.setToken(token);

        $rootScope.user = user;

        var oldName = oldUser && oldUser.metadata && oldUser.metadata.name;
        var newName = user    && user.metadata    && user.metadata.name;
        if (oldName != newName) {
          if (debug) { console.log('AuthService.setUser(), user changed', oldUser, user); }
          _userChangedCallbacks.fire(user);
        }
      },
  
      requestRequiresAuth: function(config) {
        // TODO: replace with real implementation
        var requiresAuth = config.url.toString().indexOf("api/") > 0;
        if (debug && requiresAuth) { console.log('AuthService.requestRequiresAuth()', config.url.toString()); }
        return requiresAuth;
      },
      addAuthToRequest: function(config) {
        // Use the given token, if provided
        var token = "";
        if (config && config.auth && config.auth.token) {
          token = config.auth.token;
          if (debug) { console.log('AuthService.addAuthToRequest(), using token from request config', token); }
        } else {
          token = userStore.getToken();
          if (debug) { console.log('AuthService.addAuthToRequest(), using token from user store', token); }
        }
        if (!token) {
          if (debug) { console.log('AuthService.addAuthToRequest(), no token available'); }
          return false;
        }
  
        // Handle web socket requests with a parameter
        if (config.method == 'WATCH') {
          config.url = URI(config.url).addQuery({access_token: token}).toString();
          if (debug) { console.log('AuthService.addAuthToRequest(), added token param', config.url); }
        } else {
          config.headers["Authorization"] = "Bearer " + token;
          if (debug) { console.log('AuthService.addAuthToRequest(), added token header', config.headers["Authorization"]); }
        }
        return true;
      },
    
      startLogin: function() {
        if (_loginPromise) {
          if (debug) { console.log("Login already in progress"); }
          return _loginPromise;
        }
        var self = this;
        _loginPromise = loginService.login().then(function(user, token) {
          self.setUser(user, token);
          _loginCallbacks.fire(user);
        }).catch(function(err) {
          console.log(err);
        }).finally(function() {
          _loginPromise = null;
        });
        return _loginPromise;
      },
  
      // TODO: add a way to unregister once we start doing in-page logins
      onLogin: function(callback) {
        _loginCallbacks.add(callback);
      },
      // TODO: add a way to unregister once we start doing in-page user changes
      onUserChanged: function(callback) {
        _userChangedCallbacks.add(callback);
      }
    }
  };
})
// register the interceptor as a service
.factory('AuthInterceptor', ['$q', 'AuthService', function($q, AuthService) {
  var pendingRequestConfigs = [];
  // TODO: subscribe to user change events to empty the saved configs
  // TODO: subscribe to login events to retry the saved configs

  return {
    // If auth is not needed, or is already present, returns a config
    // If auth is needed and not present, starts a login flow and returns a promise of a config
    request: function(config) {
      // Requests that don't require auth can continue
      if (!AuthService.requestRequiresAuth(config)) {
        // console.log("No auth required", config.url);
        return config;
      }

      // If we could add auth info, we can continue
      if (AuthService.addAuthToRequest(config)) {
        // console.log("Auth added", config.url);
        return config;
      }

      // We should have added auth info, but couldn't

      // If we were specifically told not to trigger a login, return
      if (config.auth && config.auth.triggerLogin === false) {
        return config;
      }

      // 1. Set up a deferred and remember this config, so we can add auth info and resume once login is complete
      var deferred = $q.defer();
      pendingRequestConfigs.push([deferred, config, 'request']);
      // 2. Start the login flow
      AuthService.startLogin();
      // 3. Return the deferred's promise
      return deferred.promise;
    },

    responseError: function(rejection) {
      var authConfig = rejection.config.auth || {};

      // Requests that didn't require auth can continue
      if (!AuthService.requestRequiresAuth(rejection.config)) {
        // console.log("No auth required", rejection.config.url);
        return $q.reject(rejection);
      }

      // If we were specifically told not to trigger a login, return
      if (authConfig.triggerLogin === false) {
        return $q.reject(rejection);
      }

      // detect if this is an auth error (401) or other error we should trigger a login flow for
      var status = rejection.status;
      switch (status) {
        case 401:
        case 0:
          // console.log('responseError', status);
          // 1. Set up a deferred and remember this config, so we can add auth info and retry once login is complete
          var deferred = $q.defer();
          pendingRequestConfigs.push([deferred, rejection.config, 'responseError']);
          // 2. Start the login flow
          AuthService.startLogin();
          // 3. Return the deferred's promise
          return deferred.promise;
        default:
          return $q.reject(rejection);
      }
    }
  };
}])
