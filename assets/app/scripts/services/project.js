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
        // NOTE: Currently this has to be a jQuery Deferred() because
        // dataService is  expecting a particular kind of promise
        //  that has a .done() method. Angular's $q.defer() will not work.
        projectPromise: $.Deferred()
      };

      return {
        get: function(projectName) {
          return  AuthService
                    .withUser()
                    .then(function(user) {
                      context.projectName = projectName;
                      return DataService
                              .get('projects', context.projectName, context, {errorNotification: false})
                              .then(function(project) {
                                context.projectPromise.resolve(project);
                                // TODO:
                                // this should only need to return the project,
                                // but has to return the projectPromise
                                // because DataService is using it as a
                                // proxy before performing various tasks, keeping
                                // additional requests from resolving.
                                return [project, context];
                              }, function(e) {
                                context.projectPromise.reject(e);
                              });
                    });
          }
        }
    }
  ]);





