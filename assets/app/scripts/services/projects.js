'use strict';

angular.module('openshiftConsole')
  .factory('ProjectsService', [
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
                      // basic compatibility w/previous impl with a controller
                      var context = {
                        // TODO: swap $.Deferred() for $q.defer()
                        projectPromise: $.Deferred(),
                        projectName: projectName,
                        project: undefined
                      };
                      return DataService
                              .get('projects', projectName, context, {errorNotification: false})
                              .then(function(project) {
                                context.project = project;
                                // backwards compat
                                context.projectPromise.resolve(project);
                                // TODO: ideally would just return project, but DataService expects
                                // context.projectPromise as a separate Deferred at this point
                                // and ties mutliple requests together via this obj
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




