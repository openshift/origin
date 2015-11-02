'use strict';

angular.module('openshiftConsole')
  .factory('project', [
    '$location',
    '$q',
    '$routeParams',
    'AuthService',
    'DataService',
    function($location, $q, $routeParams, AuthService, DataService) {
      return {
        get: function(projectName) {
          return  AuthService
                    .withUser()
                    .then(function() {
                      var context = {
                        // TODO: swap $.Deferred() for $q.defer()
                        projectName: projectName,
                        projectPromise: $.Deferred()
                      };
                      return DataService
                              .get('projects', context.projectName, context, {errorNotification: false})
                              .then(function(project) {
                                context.projectPromise.resolve(project);
                                // TODO: ideally would just return project, but DataService expects
                                // context.projectPromise as a separate Deferred at this point.
                                return [project, context];
                              }, function(e) {
                                context.projectPromise.reject(e);
                                var description = 'The project could not be loaded.';
                                var type = 'error';
                                if(e.status === 403) {
                                  description = 'The project ' + context.projectName + ' does not exist or you are not authorized to view it.';
                                  type = 'access_denied';
                                } else if (e.status === 404) {
                                  description = 'The project " + context.projectName + " does not exist.';
                                  type = 'not_found';
                                }
                                $location
                                  .url(
                                    URI('error')
                                      .query({
                                        "error" : type,
                                        "error_description": description
                                      })
                                      .toString());
                              });
                    });
          }
        };
    }
  ]);





