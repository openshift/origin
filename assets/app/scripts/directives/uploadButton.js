'use strict';

angular.module("openshiftConsole")
  .directive("uploadButton", function ($filter, DataService, Logger) {
    return {
      restrict: "E",
      scope: {
        resourceType: "@",
        alerts: "=alerts"
      },
      templateUrl: "views/directives/upload-button.html",
      link: function(scope, element, attrs) {
        scope.uploadFile = function(file) {
          scope.uploading = true;
          scope.created = false;
          DataService.createFromUpload(attrs.resourceType + 's', file, scope.$parent)
          .then(function() {
            scope.$emit('refreshList');
            scope.alerts[file.name] = {
              type: "success",
              message: "Successfully created " + attrs.resourceType + " from '" + file.name + "'!"
            };
          }).catch(function (err) {
            scope.alerts[file.name] = {
              type: "error",
              message: "Could not create " + attrs.resourceType + " from '" + file.name + "'.",
              details: $filter('getErrorDetails')(err)
            };
            Logger.error("Could not create " + attrs.resourceType + " from '" + file.name + "'.", err);
          }).finally(function() {
            scope.uploading = false;
          });
        };
      }
    };
  });