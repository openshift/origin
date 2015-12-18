'use strict';

angular.module('openshiftConsole')
    .controller('GitRepoController', function($filter, $http, $scope, $routeParams, ProjectsService, DataService, Git, GitApiStub) {
          $scope.projectName = $routeParams.project;
          $scope.renderOptions = {
            hideFilterWidget: true
          };

          ProjectsService
            .get($routeParams.project)
            .then(_.spread(function(project, context) {
              $scope.project = project;

              DataService
                .get("builds", $routeParams.build, context)
                .then(function(build) {
                  $scope.build = build;

                  if(!Git.uri.for.commits(build)) {
                    return;
                  };

                  Git                   // real API call
                  //GitApiStub          // stub, to avoid hitting github's rate limit
                    .get(Git.uri.for.commits(build))
                    .then(function(request) {
                      $scope.githubLinks = request.data;
                    },function() {
                      $scope.alerts['error'] = {
                        type: "error",
                        message: "Error",
                        details: "We're sorry, but something went wrong."
                      };
                    });
                });
            }));

        });
