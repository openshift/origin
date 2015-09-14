'use strict';

angular.module("openshiftConsole")
  .directive("uploadButton", function (DataService, Logger) {
    return {
      restrict: "E",
      scope: {
        resourceType: "@",
        alerts: "=alerts"
      },
      templateUrl: "views/directives/upload-button.html",
      link: function(scope, element, attrs) {
        var getErrorDetails = function(result) {
          var error = result.data || {};
          if (error.message) {
            return error.message;
          }

          var status = result.status || error.status;
          if (status) {
            return "Status: " + status;
          }

          return "";
        };

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
              details: getErrorDetails(err)
            };
            Logger.error("Could not create " + attrs.resourceType + " from '" + file.name + "'.", err);
          }).finally(function() {
            scope.uploading = false;
          });
        };
      }
    };
  });