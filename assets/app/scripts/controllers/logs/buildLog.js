'use strict';

angular.module('openshiftConsole')
  .controller('BuildLog', [
    '$anchorScroll',
    '$location',
    '$q',
    '$routeParams',
    '$scope',
    '$timeout',
    '$window',
    'AuthService',
    'DataService',
    'logLinks',
    function($anchorScroll, $location, $q, $routeParams, $scope, $timeout, $window, AuthService, DataService, logLinks) {

      var requestContext = {
        projectName: $routeParams.project,
        // TODO: possible to hide this away in the service?
        projectPromise: $.Deferred()
      };

      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        Logger.log("projects", $scope.projects);
      });

      AuthService
        .withUser()
        .then(function(user) {
          return DataService
                  .get('projects', requestContext.projectName, requestContext, {errorNotification: false})
                  .then(function(project) {
                    // though the above .get('projects') is really my project promise...
                    requestContext.projectPromise.resolve(project);
                    angular.extend(requestContext, {
                      project: project
                    });
                    return project;
                  }, function(e) {
                    requestContext.projectPromise.reject(e);
                  });
        })
        .then(function(project) {
          return DataService
                  .get('builds', $routeParams.build, requestContext)
                  .then(function(build) {
                    angular.extend($scope, {
                      project: project,
                      build: build,
                      logName: build.metadata.name
                    });
                    return build;
                  });
        })
        .then(function() {
          return DataService
                  .get('builds/log', $routeParams.build, requestContext)
                  .then(function(log) {
                    angular.extend($scope, {
                      ready: true,
                      canDownload: logLinks.canDownload(),
                      makeDownload: logLinks.makeDownload,
                      scrollTo: logLinks.scrollTo,
                      scrollTop: logLinks.scrollTop,
                      scrollBottom: logLinks.scrollBottom,
                      goFull: logLinks.fullPageLink,
                      goChromeless: logLinks.chromelessLink,
                      goText: logLinks.textOnlyLink,
                      // optionally as a text string or array.
                      // experimenting w/angular's ability to render...
                      log:  log ?
                            _.reduce(
                              log.split('\n'),
                              function(memo, next, i, list) {
                                return (i < list.length) ?
                                          memo + _.padRight(i+1+'. ', 7) + next + '\n' :
                                          memo;
                              },'') :
                            'Error retrieving build log',
                      // log list is an array of log lines. Angular struggles with this.
                      logList:  log ?
                                _.map(
                                  log.split('\n'),
                                  function(text) {
                                    return {
                                      text: text
                                    }
                                  }) :
                                [{text: 'Error retrieving build log'}]
                    });
                    // hack.
                    $timeout(function() {
                      $anchorScroll();
                    });
                    return log;
                  });
        })
        .catch(function(err) {
          angular.extend($scope, {
            // for the moment just passing the error response up to print
            log: err
          });
        });
    }
  ]);
