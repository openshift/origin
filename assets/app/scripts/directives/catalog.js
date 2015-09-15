'use strict';

angular.module('openshiftConsole')
  .directive("catalogCategory", function () {
    return {
      restrict: "E",
      scope: {
        categoryLabel: "@",
        builders: "=",
        templates: "=",
        project: "@",
        itemLimit: "@",
        filterTag: "="
      },
      templateUrl: "views/catalog/_catalog-category.html",
      controller: function($scope) {
        $scope.builderID = function(builder) {
          return builder.imageStream.metadata.uid + ":" + builder.imageStreamTag;
        };
      }
    };
  })
  .directive('catalogTemplate', function() {
    return {
      restrict: 'E',
      // Replace the catalog-template element so that the tiles are all equal height as flexbox items.
      // Otherwise, you have to add the CSS tile classes to catalog-template.
      replace: true,
      scope: {
        template: '=',
        project: '@',
        filterTag: "="
      },
      templateUrl: 'views/catalog/_template.html'
    };
  })
  .directive('catalogImage', function() {
    return {
      restrict: 'E',
      // Replace the catalog-template element so that the tiles are all equal height as flexbox items.
      // Otherwise, you have to add the CSS tile classes to catalog-template.
      replace: true,
      scope: {
        image: '=',
        imageStream: '=',
        imageTag: '=',
        version: '=',
        project: '@',
        filterTag: "=",
        isBuilder: "=?"
      },
      templateUrl: 'views/catalog/_image.html'
    };
  });
