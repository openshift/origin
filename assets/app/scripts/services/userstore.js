// UserStore objects able to remember user and tokens for the current user
angular.module('openshiftConsole')
.provider('MemoryUserStore', function() {
  this.$get = function(){
    var debug = false;
    var _user = null;
    var _token = null;
    return {
      available: function() {
        return true;
      },
      getUser: function(){
        if (debug) { console.log("MemoryUserStore.getUser", _user); }
        return _user;
      },
      setUser: function(user) {
        if (debug) { console.log("MemoryUserStore.setUser", user); }
        _user = user;
      },
      getToken: function() {
        if (debug) { console.log("MemoryUserStore.getToken", _token); }
        return _token;
      },
      setToken: function(token) {
        if (debug) { console.log("MemoryUserStore.setToken", token); }
        _token = token;
      }
    }
  };
})
.provider('SessionStorageUserStore', function() {
  this.$get = function(){
    var debug = false;
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
          if (debug) { console.log("SessionStorageUserStore.getUser", user); }
          return user;
        } catch(e) {
          if (debug) { console.log("SessionStorageUserStore.getUser", e); }
          return null;
        }
      },
      setUser: function(user) {
        if (user) {
          if (debug) { console.log("SessionStorageUserStore.setUser", user); }
          sessionStorage[userkey] = JSON.stringify(user);
        } else {
          if (debug) { console.log("SessionStorageUserStore.setUser", user, "deleting"); }
          sessionStorage.removeItem(userkey);
        }
      },
      getToken: function() {
        try {
          var token = sessionStorage[tokenkey];
          if (debug) { console.log("SessionStorageUserStore.getToken", token); }
          return token;
        } catch(e) {
          if (debug) { console.log("SessionStorageUserStore.getToken", e); }
          return null;
        }
      },
      setToken: function(token) {
        if (token) {
          if (debug) { console.log("SessionStorageUserStore.setToken", token); }
          sessionStorage[tokenkey] = token;
        } else {
          if (debug) { console.log("SessionStorageUserStore.setToken", token, "deleting"); }
          sessionStorage.removeItem(tokenkey);
        }
      }
    }
  };
})
.provider('LocalStorageUserStore', function() {
  this.$get = function(){
    var debug = false;
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
          if (debug) { console.log("LocalStorageUserStore.getUser", user); }
          return user;
        } catch(e) {
          if (debug) { console.log("LocalStorageUserStore.getUser", e); }
          return null;
        }
      },
      setUser: function(user) {
        if (user) {
          if (debug) { console.log("LocalStorageUserStore.setUser", user); }
          localStorage[userkey] = JSON.stringify(user);
        } else {
          if (debug) { console.log("LocalStorageUserStore.setUser", user, "deleting"); }
          localStorage.removeItem(userkey);
        }
      },
      getToken: function() {
        try {
          var token = localStorage[tokenkey];
          if (debug) { console.log("LocalStorageUserStore.getToken", token); }
          return token;
        } catch(e) {
          if (debug) { console.log("LocalStorageUserStore.getToken", e); }
          return null;
        }
      },
      setToken: function(token) {
        if (token) {
          if (debug) { console.log("LocalStorageUserStore.setToken", token); }
          localStorage[tokenkey] = token;
        } else {
          if (debug) { console.log("LocalStorageUserStore.setToken", token, "deleting"); }
          localStorage.removeItem(tokenkey);
        }
      }
    }
  };
});
