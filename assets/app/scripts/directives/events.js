'use strict';

angular.module('openshiftConsole')
  .directive('events', function($routeParams, $filter, DataService, ProjectsService, Logger) {
    return {
      restrict: 'E',
      scope: {
        resourceKind: "@?",
        resourceName: "@?",
        projectContext: "="
      },
      templateUrl: 'views/directives/events.html',
      controller: function($scope){
        $scope.filter = {
          text: ''
        };

        var filterForResource = function(events) {
          if (!$scope.resourceKind || !$scope.resourceName) {
            return events;
          }

          return _.filter(events, function(event) {
            return event.involvedObject.kind === $scope.resourceKind &&
                   event.involvedObject.name === $scope.resourceName;
          });
        };

        var sortedEvents = [];
        var sortEvents = function() {
          // TODO: currentField is renamed in angular-patternfly 3.0
          var sortID = _.get($scope, 'sortConfig.currentField.id', 'lastTimestamp'),
              order = $scope.sortConfig.isAscending ? 'asc' : 'desc';
          sortedEvents = _.sortByOrder($scope.events, [sortID], [order]);
        };

        var keywords = [];
        var updateKeywords = function() {
          if (!$scope.filter.text) {
            keywords = [];
            return;
          }

          keywords = _.uniq($scope.filter.text.split(/\s+/));
          // Sort the longest keyword first.
          keywords.sort(function(a, b){
            return b.length - a.length;
          });
        };

        var applyFilter = $filter('filter');
        var filterForKeyword = function() {
          $scope.filteredEvents = sortedEvents;
          if (!keywords.length) {
            return;
          }

          // Find events that match all keywords, removing duplicates.
          angular.forEach(keywords, function(keyword) {
            $scope.filteredEvents = applyFilter($scope.filteredEvents, keyword);
          });
        };

        $scope.$watch('filter.text', _.debounce(function() {
          updateKeywords();
          $scope.$apply(filterForKeyword);
        }, 50, { maxWait: 250 }));

        var update = function() {
          // Sort first so we can update the filter as the user types without resorting.
          sortEvents();
          filterForKeyword();
        };

        // Invoke update when first called, debouncing subsequent calls.
        var debounceUpdate = _.debounce(function() {
          $scope.$evalAsync(update);
        }, 250, {
          leading: true,
          trailing: false,
          maxWait: 1000
        });

        // Set up the sort configuration for `pf-simple-sort`.
        $scope.sortConfig = {
          fields: [{
            id: 'lastTimestamp',
            title: 'Time',
            sortType: 'alpha'
          }, {
            id: 'type',
            title: 'Severity',
            sortType: 'alpha'
          }, {
            id: 'reason',
            title: 'Reason',
            sortType: 'alpha'
          }, {
            id: 'message',
            title: 'Message',
            sortType: 'alpha'
          }, {
            id: 'count',
            title: 'Count',
            sortType: 'numeric'
          }],
          isAscending: false,
          onSortChange: update
        };

        // Conditionally add kind and name to sort fields if not passed to the directive.
        if (!$scope.resourceKind || !$scope.resourceName) {
          $scope.sortConfig.fields.splice(1, 0, {
            id: 'involvedObject.name',
            title: 'Name',
            sortType: 'alpha'
          }, {
            id: 'involvedObject.kind',
            title: 'Kind',
            sortType: 'alpha'
          });
        }

        var watches = [];
        watches.push(DataService.watch("events", $scope.projectContext, function(events) {
          $scope.events = filterForResource(events.by('metadata.name'));
          debounceUpdate();
          Logger.log("events (subscribe)", $scope.filteredEvents);
        }));

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });

      },
    };
  });
