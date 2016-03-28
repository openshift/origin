'use strict';

angular.module('openshiftConsole')
  .factory('ProjectsService',
    function($location, $q, $routeParams, AuthService, DataService, annotationNameFilter) {


      var cleanEditableAnnotations = function(resource) {
        var paths = [
              annotationNameFilter('description'),
              annotationNameFilter('displayName')
            ];
        _.each(paths, function(path) {
          if(!resource.metadata.annotations[path]) {
            delete resource.metadata.annotations[path];
          }
        });
        return resource;
      };

      return {
        get: function(projectName) {
          return  AuthService
                    .withUser()
                    .then(function() {
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
                                context.projectPromise.resolve(project);
                                // TODO: fix need to return context & projectPromise
                                return [project, context];
                              }, function(e) {
                                context.projectPromise.reject(e);
                                var description = 'The project could not be loaded.';
                                var type = 'error';
                                if(e.status === 403) {
                                  description = 'The project ' + context.projectName + ' does not exist or you are not authorized to view it.';
                                  type = 'access_denied';
                                } else if (e.status === 404) {
                                  description = 'The project ' + context.projectName + ' does not exist.';
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
          },
          update: function(projectName, data) {
            return DataService
                    .update('projects', projectName, cleanEditableAnnotations(data), {projectName: projectName}, {errorNotification: false});
          }
        };
    });
