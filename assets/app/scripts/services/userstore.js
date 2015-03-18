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
      setUser: function(user) {
        authLogger.log("MemoryUserStore.setUser", user);
        _user = user;
      },
      getToken: function() {
        authLogger.log("MemoryUserStore.getToken", _token);
        return _token;
      },
      setToken: function(token) {
        authLogger.log("MemoryUserStore.setToken", token);
        _token = token;
      }
    }
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
          var x = new Date().getTime();
          sessionStorage['SessionStorageUserStore.test'] = x;
          var y = sessionStorage['SessionStorageUserStore.test'];
          sessionStorage.removeItem('SessionStorageUserStore.test');
          return x == y;
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
      setUser: function(user) {
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
      setToken: function(token) {
        if (token) {
          authLogger.log("SessionStorageUserStore.setToken", token);
          sessionStorage[tokenkey] = token;
        } else {
          authLogger.log("SessionStorageUserStore.setToken", token, "deleting");
          sessionStorage.removeItem(tokenkey);
        }
      }
    }
  };
})
.provider('LocalStorageUserStore', function() {
  this.$get = function(Logger){
    var authLogger = Logger.get("auth");
    var userkey = "LocalStorageUserStore.user";
    var tokenkey = "LocalStorageUserStore.token";
    return {
      available: function() {
        try {
          var x = new Date().getTime();
          localStorage['LocalStorageUserStore.test'] = x;
          var y = localStorage['LocalStorageUserStore.test'];
          localStorage.removeItem('LocalStorageUserStore.test');
          return x == y;
        } catch(e) {
          return false;
        }
      },
      getUser: function(){
        try {
          var user = JSON.parse(localStorage[userkey]);
          authLogger.log("LocalStorageUserStore.getUser", user);
          return user;
        } catch(e) {
          authLogger.error("LocalStorageUserStore.getUser", e);
          return null;
        }
      },
      setUser: function(user) {
        if (user) {
          authLogger.log("LocalStorageUserStore.setUser", user);
          localStorage[userkey] = JSON.stringify(user);
        } else {
          authLogger.log("LocalStorageUserStore.setUser", user, "deleting");
          localStorage.removeItem(userkey);
        }
      },
      getToken: function() {
        try {
          var token = localStorage[tokenkey];
          authLogger.log("LocalStorageUserStore.getToken", token);
          return token;
        } catch(e) {
          authLogger.error("LocalStorageUserStore.getToken", e);
          return null;
        }
      },
      setToken: function(token) {
        if (token) {
          authLogger.log("LocalStorageUserStore.setToken", token);
          localStorage[tokenkey] = token;
        } else {
          authLogger.log("LocalStorageUserStore.setToken", token, "deleting");
          localStorage.removeItem(tokenkey);
        }
      }
    }
  };
});
