'use strict';
/* jshint unused: false */

angular.module('openshiftConsole')
  .directive('catalogTemplate', function($location) {
    return {
      restrict: 'E',
      // Replace the catalog-template element so that the tiles are all equal height as flexbox items.
      // Otherwise, you have to add the CSS tile classes to catalog-template.
      replace: true,
      scope: {
        template: '=',
        project: '='
      },
      templateUrl: 'views/catalog/_template.html'
    };
  })
  .directive('catalogImage', function($location) {
    return {
      restrict: 'E',
      // Replace the catalog-template element so that the tiles are all equal height as flexbox items.
      // Otherwise, you have to add the CSS tile classes to catalog-template.
      replace: true,
      scope: {
        image: '=',
        imageRepo: '=',
        imageTag: '=',
        version: '=',
        project: '=',
        sourceUrl: '='
      },
      templateUrl: 'views/catalog/_image.html'
    };
  });
