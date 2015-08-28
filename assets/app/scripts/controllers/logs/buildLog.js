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
    'DataService',
    'project',
    'logLinks',
    function($anchorScroll, $location, $q, $routeParams, $scope, $timeout, $window, DataService, project, logLinks) {

      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        Logger.log("projects", $scope.projects);
      });

      project
        .get($routeParams.project)
        .then(_.spread(function(project, context) {
          return $q.all([
                    DataService
                      .get('builds', $routeParams.build, context),
                    DataService
                      .get('builds/log', $routeParams.build, context)
                  ])
                  .then(_.spread(function(build, log) {
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
                      project: project,
                      build: build,
                      logName: build.metadata.name,
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
                  }));
        }))
        .catch(function(err) {
          angular.extend($scope, {
            log: err
          });
        });
    }
  ]);
