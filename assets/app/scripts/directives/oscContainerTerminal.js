'use strict';
// osc-container-terminal is a wrapper around kubernetes-container-terminal
// - it takes the same configuration options are kubernetes-container-terminal
// - if rows/cols attribs are not provided it will check browser window and
//   set these to something that should fit the window on initial page load.
//   note that we cannot resize the terminal at this time, there is no way to
//   send a SIGWINCH signal, and we don't want to restart the terminal.
angular
  .module('openshiftConsole')
  // note: could use BREAKPOINTS constant if we
  // want to standardize on a few terminal sizes
  .directive('oscContainerTerminal', function($compile, $sce, $timeout) {
    return {
      restrict: 'E',
      scope: {
        pod: '=',
        container: '=',
        prevent: '=',
        screenKeys: '=?',
        // optional, will set in link fn to something sensible.
        rows: '=?',
        cols: '=?'
      },
      templateUrl: 'views/directives/osc-container-terminal.html',
      link: function($scope, $elem) {
        // to test for # of cols
        var proxyDOM = $elem[0].find('.terminal-wrap');

        $scope.canRender = false;

        if(!$scope.rows) {
          $scope.rows = 24;
        }
        if(!$scope.screenKeys) {
          $scope.screenKeys = true;
        }

        $scope.$watch('prevent', function(prevent) {
          var maxWidth;
          if(!prevent) {
            $timeout(function() {
              if(!$scope.cols) {
                maxWidth = Math.floor(proxyDOM.clientWidth/6);
                $scope.cols = (maxWidth <= 80) ? maxWidth : 80;
                  $scope.canRender = true;
                }
            },1);
          }
        });
      }
    };
  });
