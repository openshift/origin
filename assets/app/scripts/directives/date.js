angular.module('openshiftConsole')
  .directive("relativeTimestamp", function() {
    return {
      restrict: 'E',
      scope: {
        timestamp: '='
      },
      template: '<span data-timestamp="{{timestamp}}" class="timestamp" title="{{timestamp | date : \'short\'}}">{{timestamp | dateRelative}}</span>'
    };
  });