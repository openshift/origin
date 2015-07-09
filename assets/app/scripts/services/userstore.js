'use strict';
/* jshint unused: false */

// UserStore objects able to remember user and tokens for the current user
angular.module('openshiftConsole')
.provider('MemoryUserStore', function() {
  this.$get = function(Logger){
    var authLogger = Logger.get("auth");
    var _user = null;
    var _token = null;
    return {
      available: function() {
        return true;
      },
      getUser: function(){
        authLogger.log("MemoryUserStore.getUser", _user);
        return _user;
      },
      setUser: function(user, ttl) {
        // TODO: honor ttl
        authLogger.log("MemoryUserStore.setUser", user);
        _user = user;
      },
      getToken: function() {
        authLogger.log("MemoryUserStore.getToken", _token);
        return _token;
      },
      setToken: function(token, ttl) {
        // TODO: honor ttl
        authLogger.log("MemoryUserStore.setToken", token);
        _token = token;
      }
    };
  };
})
.provider('SessionStorageUserStore', function() {
  this.$get = function(Logger){
    var authLogger = Logger.get("auth");
    var userkey = "SessionStorageUserStore.user";
    var tokenkey = "SessionStorageUserStore.token";
    return {
      available: function() {
        try {
          var x = String(new Date().getTime());
          sessionStorage['SessionStorageUserStore.test'] = x;
          var y = sessionStorage['SessionStorageUserStore.test'];
          sessionStorage.removeItem('SessionStorageUserStore.test');
          return x === y;
        } catch(e) {
          return false;
        }
      },
      getUser: function(){
        try {
          var user = JSON.parse(sessionStorage[userkey]);
          authLogger.log("SessionStorageUserStore.getUser", user);
          return user;
        } catch(e) {
          authLogger.error("SessionStorageUserStore.getUser", e);
          return null;
        }
      },
      setUser: function(user, ttl) {
        // TODO: honor ttl
        if (user) {
          authLogger.log("SessionStorageUserStore.setUser", user);
          sessionStorage[userkey] = JSON.stringify(user);
        } else {
          authLogger.log("SessionStorageUserStore.setUser", user, "deleting");
          sessionStorage.removeItem(userkey);
        }
      },
      getToken: function() {
        try {
          var token = sessionStorage[tokenkey];
          authLogger.log("SessionStorageUserStore.getToken", token);
          return token;
        } catch(e) {
          authLogger.error("SessionStorageUserStore.getToken", e);
          return null;
        }
      },
      setToken: function(token, ttl) {
        // TODO: honor ttl
        if (token) {
          authLogger.log("SessionStorageUserStore.setToken", token);
          sessionStorage[tokenkey] = token;
        } else {
          authLogger.log("SessionStorageUserStore.setToken", token, "deleting");
          sessionStorage.removeItem(tokenkey);
        }
      }
    };
  };
})
.provider('LocalStorageUserStore', function() {
  this.$get = function(Logger){
    var authLogger = Logger.get("auth");
    var userkey = "LocalStorageUserStore.user";
    var tokenkey = "LocalStorageUserStore.token";

    var ttlKey = function(key) {
      return key + ".ttl";
    };
    var setTTL = function(key, ttl) {
      if (ttl) {
        var expires = new Date().getTime() + ttl*1000;
        localStorage[ttlKey(key)] = expires;
        authLogger.log("LocalStorageUserStore.setTTL", key, ttl, new Date(expires).toString());
      } else {
        localStorage.removeItem(ttlKey(key));
        authLogger.log("LocalStorageUserStore.setTTL deleting", key);
      }
    };
    var isTTLExpired = function(key) {
      var ttl = localStorage[ttlKey(key)];
      if (!ttl) {
        return false;
      }
      var expired = parseInt(ttl) < new Date().getTime();
      authLogger.log("LocalStorageUserStore.isTTLExpired", key, expired);
      return expired;
    };

    return {
      available: function() {
        try {
          var x = String(new Date().getTime());
          localStorage['LocalStorageUserStore.test'] = x;
          var y = localStorage['LocalStorageUserStore.test'];
          localStorage.removeItem('LocalStorageUserStore.test');
          return x === y;
        } catch(e) {
          return false;
        }
      },
      getUser: function(){
        try {
          if (isTTLExpired(userkey)) {
            authLogger.log("LocalStorageUserStore.getUser expired");
            localStorage.removeItem(userkey);
            setTTL(userkey, null);
            return null;
          }
          var user = JSON.parse(localStorage[userkey]);
          authLogger.log("LocalStorageUserStore.getUser", user);
          return user;
        } catch(e) {
          authLogger.error("LocalStorageUserStore.getUser", e);
          return null;
        }
      },
      setUser: function(user, ttl) {
        if (user) {
          authLogger.log("LocalStorageUserStore.setUser", user, ttl);
          localStorage[userkey] = JSON.stringify(user);
          setTTL(userkey, ttl);
        } else {
          authLogger.log("LocalStorageUserStore.setUser", user, "deleting");
          localStorage.removeItem(userkey);
          setTTL(userkey, null);
        }
      },
      getToken: function() {
        try {
          if (isTTLExpired(tokenkey)) {
            authLogger.log("LocalStorageUserStore.getToken expired");
            localStorage.removeItem(tokenkey);
            setTTL(tokenkey, null);
            return null;
          }
          var token = localStorage[tokenkey];
          authLogger.log("LocalStorageUserStore.getToken", token);
          return token;
        } catch(e) {
          authLogger.error("LocalStorageUserStore.getToken", e);
          return null;
        }
      },
      setToken: function(token, ttl) {
        if (token) {
          authLogger.log("LocalStorageUserStore.setToken", token, ttl);
          localStorage[tokenkey] = token;
          setTTL(tokenkey, ttl);
        } else {
          authLogger.log("LocalStorageUserStore.setToken", token, ttl, "deleting");
          localStorage.removeItem(tokenkey);
          setTTL(tokenkey, null);
        }
      }
    };
  };
});
