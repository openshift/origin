// UserStore objects able to remember user and tokens for the current user
angular.module('openshiftConsole')
.provider('MemoryUserStore', function() {
  this.$get = function(){
    var debug = false;
    var _user = null;
    var _token = null;
    return {
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
.provider('SessionUserStore', function() {
  this.$get = function(){
    var debug = false;
    var userkey = "user";
    var tokenkey = "token";
    return {
      getUser: function(){
        try {
          var user = JSON.parse(sessionStorage[userkey]);
          if (debug) { console.log("SessionUserStore.getUser", user); }
          return user;
        } catch(e) {
          if (debug) { console.log("SessionUserStore.getUser", e); }
          return null;
        }
      },
      setUser: function(user) {
        if (user) {
          if (debug) { console.log("SessionUserStore.setUser", user); }
          sessionStorage[userkey] = JSON.stringify(user);
        } else {
          if (debug) { console.log("SessionUserStore.setUser", user, "deleting"); }
          sessionStorage.removeItem(userkey);
        }
      },
      getToken: function() {
        try {
          var token = sessionStorage[tokenkey];
          if (debug) { console.log("SessionUserStore.getToken", token); }
          return token;
        } catch(e) {
          if (debug) { console.log("SessionUserStore.getToken", e); }
          return null;
        }
      },
      setToken: function(token) {
        if (token) {
          if (debug) { console.log("SessionUserStore.setToken", token); }
          sessionStorage[tokenkey] = token;
        } else {
          if (debug) { console.log("SessionUserStore.setToken", token, "deleting"); }
          sessionStorage.removeItem(tokenkey);
        }
      }
    }
  };
});
