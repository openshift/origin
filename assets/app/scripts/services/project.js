'use strict';

angular.module('openshiftConsole')
  .factory('project', [
    '$q',
    '$routeParams',
    'AuthService',
    'DataService',
    function($q, $routeParams, AuthService, DataService) {
      var context = {
        // TODO: swap $.Deferred() for $q.defer()
        projectPromise: $.Deferred()
      };

      return {
        get: function(projectName) {
          return  AuthService
                    .withUser()
                    .then(function() {
                      context.projectName = projectName;
                      return DataService
                              .get('projects', context.projectName, context, {errorNotification: false})
                              .then(function(project) {
                                context.projectPromise.resolve(project);
                                // TODO: ideally would just return project, but DataService expects
                                // context.projectPromise as a separate Deferred at this point.
                                return [project, context];
                              }, function(e) {
                                context.projectPromise.reject(e);
                              });
                    });
          }
        };
    }
  ]);





