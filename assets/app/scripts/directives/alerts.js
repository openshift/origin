'use strict';
/* jshint expr: true */

angular.module('openshiftConsole')
  .directive('alerts', function(pubsub) {
    return {
      restrict: 'E',
      scope: true,
      controller: function($scope, $timeout) {
        $scope.alerts = [];
        var events = [
          pubsub.subscribe('alert', function(data) {
            var index = ($scope.alerts.push(data) -1);
            if(data.duration) {
              $timeout(function() {
                $scope.alerts.splice(index, 1);
              }, data.duration);
            }
          }),
          pubsub.subscribe('alerts.clear', function() {
            $scope.alerts = [];
          })
        ];

        // TODO: prob good to configure w/an attribute:
        // <alert clear-on-route-change> (shorter :)
        // would be useful if <alerts> was a DOM node persistent across pages.
        $scope.$on('$routeChangeStart', function() {
          $scope.alerts = [];
        });

        $scope.$on('$destroy', function() {
          _.each(events, function(evt) {
            evt && evt.unsubscribe && evt.unsubscribe();
          });
        });

      },
      templateUrl: 'views/_alerts.html'
    };
  });
