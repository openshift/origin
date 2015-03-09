'use strict';

angular.module('openshiftConsole')
  // Element directive to display tasks from TaskService
  .directive('tasks', function() {
    return {
      restrict: 'E',
      templateUrl: 'views/_tasks.html'
    };
  });
