'use strict';

angular.module('openshiftConsole')
.directive('oscGitLink', function() {
  return {
    restrict: 'E',
    scope: {
      uri: "=",
      commit: "="
    },
    transclude: true,
    template: '<a ng-href="{{uri | githubLink : commit}}" ng-transclude></a>'
  };
});
