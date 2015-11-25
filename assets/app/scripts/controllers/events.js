'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:EventsController
 * @description
 * # EventsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('EventsController', function ($routeParams, $scope, DataService, ProjectsService, Logger) {
    $scope.projectName = $routeParams.project;
    $scope.emptyMessage = "Loading...";
    $scope.events = {};
    $scope.renderOptions = {
      hideFilterWidget: true
    };
    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        watches.push(DataService.watch("events", context, function(events) {
          $scope.events = events.by("metadata.name");
          $scope.emptyMessage = "No events to show";
          Logger.log("events (subscribe)", $scope.events);
        }));

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });
      }));
  });

