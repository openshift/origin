// Login strategies
angular.module('openshiftConsole')
.provider('RedirectLoginService', function() {
  
  var debug = true;

  var _oauth_client_id = "";
  var _oauth_authorize_uri = "";
  var _oauth_redirect_uri = "";

  this.OAuthClientID = function(id) {
  	if (id) {
  	  _oauth_client_id = id;
  	}
  	return _oauth_client_id;
  };
  this.OAuthAuthorizeURI = function(uri) {
  	if (uri) {
  	  _oauth_authorize_uri = uri;
  	}
  	return _oauth_authorize_uri;
  };
  this.OAuthRedirectURI = function(uri) {
  	if (uri) {
  	  _oauth_redirect_uri = uri;
  	}
  	return _oauth_redirect_uri;
  };

  this.$get = function($location, $q) {

    return {
      login: function() {
      	if (_oauth_client_id == "") {
      		return $q.reject("RedirectLoginServiceProvider.OAuthClientID() not set"); 
      	}
      	if (_oauth_authorize_uri == "") {
      		return $q.reject("RedirectLoginServiceProvider.OAuthAuthorizeURI() not set"); 
      	}
      	if (_oauth_redirect_uri == "") {
      		return $q.reject("RedirectLoginServiceProvider.OAuthRedirectURI not set"); 
      	}

        var deferred = $q.defer();
        var uri = new URI(_oauth_authorize_uri);
        uri.query({
          client_id: _oauth_client_id,
          response_type: 'token',
          state: $location.url(), // TODO: get state working
          redirect_uri: _oauth_redirect_uri,
        });
        if (debug) { console.log("RedirectLoginService.login(), redirecting", uri.toString()); }
        window.location.href = uri.toString();
        return deferred.promise;
      },

      // Parses oauth callback parameters from window.location
      // Returns a promise that resolves with {token:'...',then:'...'}, or rejects with {error:'...',error_description:'...',error_uri:'...'}
      // If no token and no error is present, resolves with {}
      // Example error codes: https://tools.ietf.org/html/rfc6749#section-5.2
      finish: function() {
      	// Get url
      	var u = new URI(window.location.href);

      	// Read params
      	var queryParams = u.query(true);
      	var fragmentParams = new URI("?" + u.fragment()).query(true); 
      	if (debug) { console.log("RedirectLoginService.finish()", queryParams, fragmentParams); }

     	// Error codes can come in query params or fragment params
     	// Handle an error response from the OAuth server
      	var error = queryParams.error || fragmentParams.error;
     	if (error) {
     	  var error_description = queryParams.error_description || fragmentParams.error_description;
     	  var error_uri = queryParams.error_uri || fragmentParams.error_uri;
      	  if (debug) { console.log("RedirectLoginService.finish(), error", error, error_description, error_uri); }
      	  return $q.reject({
      	    error: error,
      	    error_description: error_description,
      	    error_uri: error_uri
      	  });
      	}

      	// Handle an access_token response
      	if (fragmentParams.access_token && fragmentParams.token_type == "bearer") {
      	  var deferred = $q.defer();
      	  deferred.resolve({
      	    token: fragmentParams.access_token,
      	    then: fragmentParams.state
      	  });
      	  return deferred.promise;
      	}

      	return $q.when({});
      }
    };
  };
});
