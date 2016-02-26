'use strict';

angular.module('openshiftConsole')
  .directive("relativeTimestamp", function() {
    return {
      restrict: 'E',
      scope: {
        timestamp: '=',
        dropSuffix: '=?'
      },
      template: '<span data-timestamp="{{timestamp}}" data-drop-suffix="{{dropSuffix}}" class="timestamp" title="{{timestamp | date : \'short\'}}">{{timestamp | dateRelative : dropSuffix}}</span>'
    };
  })
  .directive("durationUntilNow", function() {
    return {
      restrict: 'E',
      scope: {
        timestamp: '=',
        omitSingle: '=?',
        precision: '=?'
      },
      template: '<span data-timestamp="{{timestamp}}" data-omit-single="{{omitSingle}}" data-precision="{{precision}}" class="duration">{{timestamp | duration : null : omitSingle : precision}}</span>'
    };
  });
