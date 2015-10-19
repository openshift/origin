'use strict';

angular.module('openshiftConsole')
// In a config step, set the desired user store and login service. For example:
//   AuthServiceProvider.setUserStore('LocalStorageUserStore')
//   AuthServiceProvider.setLoginService('RedirectLoginService')
//
// AuthService provides the following functions:
//   withUser()
//     returns a promise that resolves when there is a current user
//     starts a login if there is no current user
//   setUser(user, token[, ttl])
//     sets the current user and token to use for authenticated requests
//     if ttl is specified, it indicates how many seconds the user and token are valid
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
  var _logoutService = "";
  this.LogoutService = function(logoutServiceName) {
    if (logoutServiceName) {
      _logoutService = logoutServiceName;
    }
    return _logoutService;
  };

  var loadService = function(injector, name, setter) {
  	if (!name) {
  	  throw setter + " not set";
  	} else if (angular.isString(name)) {
  	  return injector.get(name);
  	} else {
  	  return injector.invoke(name);
  	}
  };

  this.$get = function($q, $injector, $log, $rootScope, Logger) {
    var authLogger = Logger.get("auth");
    authLogger.log('AuthServiceProvider.$get', arguments);

    var _loginCallbacks = $.Callbacks();
    var _logoutCallbacks = $.Callbacks();
    var _userChangedCallbacks = $.Callbacks();

    var _loginPromise = null;
    var _logoutPromise = null;

    var userStore = loadService($injector, _userStore, "AuthServiceProvider.UserStore()");
    if (!userStore.available()) {
      Logger.error("AuthServiceProvider.$get user store " + _userStore + " not available");
    }
    var loginService = loadService($injector, _loginService, "AuthServiceProvider.LoginService()");
    var logoutService = loadService($injector, _logoutService, "AuthServiceProvider.LogoutService()");

    return {

      // Returns the configured user store
      UserStore: function() {
        return userStore;
      },

      // Returns true if currently logged in.
      isLoggedIn: function() {
        return !!userStore.getUser();
      },

      // Returns a promise of a user, which is resolved with a logged in user. Triggers a login if needed.
      withUser: function() {
        var user = userStore.getUser();
        if (user) {
          $rootScope.user = user;
          authLogger.log('AuthService.withUser()', user);
          return $q.when(user);
        } else {
          authLogger.log('AuthService.withUser(), calling startLogin()');
          return this.startLogin();
        }
      },

      setUser: function(user, token, ttl) {
        authLogger.log('AuthService.setUser()', user, token, ttl);
        var oldUser = userStore.getUser();
        userStore.setUser(user, ttl);
        userStore.setToken(token, ttl);

        $rootScope.user = user;

        var oldName = oldUser && oldUser.metadata && oldUser.metadata.name;
        var newName = user    && user.metadata    && user.metadata.name;
        if (oldName !== newName) {
          authLogger.log('AuthService.setUser(), user changed', oldUser, user);
          _userChangedCallbacks.fire(user);
        }
      },

      requestRequiresAuth: function(config) {
        var requiresAuth = !!config.auth;
        authLogger.log('AuthService.requestRequiresAuth()', config.url.toString(), requiresAuth);
        return requiresAuth;
      },
      addAuthToRequest: function(config) {
        // Use the given token, if provided
        var token = "";
        if (config && config.auth && config.auth.token) {
          token = config.auth.token;
          authLogger.log('AuthService.addAuthToRequest(), using token from request config', token);
        } else {
          token = userStore.getToken();
          authLogger.log('AuthService.addAuthToRequest(), using token from user store', token);
        }
        if (!token) {
          authLogger.log('AuthService.addAuthToRequest(), no token available');
          return false;
        }

        // Handle web socket requests with a parameter
        if (config.method === 'WATCH') {
          config.url = URI(config.url).addQuery({access_token: token}).toString();
          authLogger.log('AuthService.addAuthToRequest(), added token param', config.url);
        } else {
          config.headers["Authorization"] = "Bearer " + token;
          authLogger.log('AuthService.addAuthToRequest(), added token header', config.headers["Authorization"]);
        }
        return true;
      },

      startLogin: function() {
        if (_loginPromise) {
          authLogger.log("Login already in progress");
          return _loginPromise;
        }
        var self = this;
        _loginPromise = loginService.login().then(function(result) {
          self.setUser(result.user, result.token, result.ttl);
          _loginCallbacks.fire(result.user);
        }).catch(function(err) {
          Logger.error(err);
        }).finally(function() {
          _loginPromise = null;
        });
        return _loginPromise;
      },

      startLogout: function() {
        if (_logoutPromise) {
          authLogger.log("Logout already in progress");
          return _logoutPromise;
        }
        var self = this;
        var user = userStore.getUser();
        var token = userStore.getToken();
        var wasLoggedIn = this.isLoggedIn();
        _logoutPromise = logoutService.logout(user, token).then(function() {
          authLogger.log("Logout service success");
        }).catch(function(err) {
          authLogger.error("Logout service error", err);
        }).finally(function() {
          // Clear the user and token
          self.setUser(null, null);
          // Make sure isLoggedIn() returns false before we fire logout callbacks
          var isLoggedIn = self.isLoggedIn();
          // Only fire logout callbacks if we transitioned from a logged in state to a logged out state
          if (wasLoggedIn && !isLoggedIn) {
            _logoutCallbacks.fire();
          }
          _logoutPromise = null;
        });
        return _logoutPromise;
      },

      // TODO: add a way to unregister once we start doing in-page logins
      onLogin: function(callback) {
        _loginCallbacks.add(callback);
      },
      // TODO: add a way to unregister once we start doing in-page logouts
      onLogout: function(callback) {
        _logoutCallbacks.add(callback);
      },
      // TODO: add a way to unregister once we start doing in-page user changes
      onUserChanged: function(callback) {
        _userChangedCallbacks.add(callback);
      }
    };
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
}]);
