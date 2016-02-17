'use strict';

angular.module('openshiftConsole')
  .directive('events', function($routeParams, $filter, DataService, ProjectsService, Logger) {
    return {
      restrict: 'E',
      scope: {
        resourceKind: "@",
        resourceName: "@",
        projectContext: "="
      },
      templateUrl: 'views/directives/events.html',
      controller: function($scope){

        var filterEvent = function(event) {
          return (event.involvedObject.kind === $scope.resourceKind) && (event.involvedObject.name === $scope.resourceName);
        };

        var watches = [];
        watches.push(DataService.watch("events", $scope.projectContext, function(events) {
          $scope.emptyMessage = "No events to show";
          // $scope.eventsArray = $filter('toArray')(events.by("metadata.name"));
          $scope.filteredEvents = _.filter(events.by("metadata.name"), filterEvent);
          Logger.log("events (subscribe)", $scope.filteredEvents);
        }));

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });

      },
    };
  });
