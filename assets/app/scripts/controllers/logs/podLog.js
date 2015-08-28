'use strict';

angular.module('openshiftConsole')
  .controller('PodLog', [
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
                      .get('pods', $routeParams.pod, context),
                    DataService
                      .get('pods/log', $routeParams.pod, context)
                  ])
                  .then(_.spread(function(pod, log) {
                        angular.extend($scope, {
                          ready: true,
                          canDownload: logLinks.canDownload(),
                          makeDownload: logLinks.makeDownload,
                          scrollTo: logLinks.scrollTo,
                          goFull: logLinks.fullPageLink,
                          goChromeless: logLinks.chromelessLink,
                          goText: logLinks.textOnlyLink,
                          project: project,
                          pod: pod,
                          logName: pod.metadata.name,
                          log:  log ?
                                _.reduce(
                                  log.split('\n'),
                                  function(memo, next, i, list) {
                                    return (i < list.length) ?
                                              memo + _.padRight(i+1+'. ', 7) + next + '\n' :
                                              memo;
                                  },'') :
                                'Error retrieving pod log',
                          logList:  log ?
                                    _.map(
                                      log.split('\n'),
                                      function(text) {
                                        return {
                                          text: text
                                        }
                                      }) :
                                    [{text: 'Error retrieving pod log'}]
                        });

                        $timeout(function() {
                          $anchorScroll();
                        });
                  }));

        }))
        .catch(function(err) {
          angular.extend($scope, {
            log: err
          });
        });
    }
  ]);
