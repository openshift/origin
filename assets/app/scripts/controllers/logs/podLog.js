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
    'AuthService',
    'DataService',
    'logLinks',
    function($anchorScroll, $location, $q, $routeParams, $scope, $timeout, $window, AuthService, DataService, logLinks) {

      var requestContext = {
        projectName: $routeParams.project,
        projectPromise: $.Deferred()
      };

      AuthService
        .withUser()
        .then(function(user) {
          return DataService
                  .get('projects', requestContext.projectName, requestContext, {errorNotification: false})
                  .then(function(project) {
                    requestContext.projectPromise.resolve(project);
                    angular.extend(requestContext, {
                      project: project
                    });
                    return project;
                  }, function(e) {
                    requestContext.projectPromise.reject(e);
                  });
        })
        .then(function() {
          return DataService
                  .get('pods', $routeParams.pod, requestContext)
                  .then(function(pod) {
                    angular.extend($scope, {
                      pod: pod,
                      logName: pod.metadata.name
                    });
                    return pod;
                  });
        })
        .then(function() {
          return DataService
                  .get('pods/log', $routeParams.pod, requestContext)
                  .then(function(log) {
                    angular.extend($scope, {
                      ready: true,
                      makeDownload: logLinks.makeDownload,
                      scrollTo: logLinks.scrollTo,
                      goFull: logLinks.fullPageLink,
                      goChromeless: logLinks.chromelessLink,
                      goText: logLinks.textOnlyLink,
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
                    return log;
                  });
        })
        .catch(function(err) {
          angular.extend($scope, {
            log: err
          });
        });
    }
  ]);
