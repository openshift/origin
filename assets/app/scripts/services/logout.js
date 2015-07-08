'use strict';

// Logout strategies
angular.module('openshiftConsole')
.provider('DeleteTokenLogoutService', function() {

  this.$get = function($q, $injector, Logger) {
    var authLogger = Logger.get("auth");

    return {
      logout: function(user, token) {
        authLogger.log("DeleteTokenLogoutService.logout()", user, token);

        // If we don't have a token, we're done
        if (!token) {
          authLogger.log("DeleteTokenLogoutService, no token, returning immediately");
          return $q.when({});
        }

        // Lazily get the data service. Can't explicitly depend on it or we get circular dependencies.
        var DataService = $injector.get('DataService');
        // Use the token to delete the token
        // Never trigger a login when deleting our token
        var opts = {http: {auth: {token: token, triggerLogin: false}}};
        // TODO: Change this to return a promise that "succeeds" even if the token delete fails?
        return DataService.delete("oauthaccesstokens", token, {}, opts);
      },
    };
  };
});
