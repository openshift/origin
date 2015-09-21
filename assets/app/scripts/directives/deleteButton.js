'use strict';

angular.module("openshiftConsole")
  .directive("deleteButton", function ($modal, $location, hashSizeFilter, DataService, AlertMessageService, Logger) {
    return {
      restrict: "E",
      scope: {
        resourceType: "@",
        resourceName: "@",
        alerts: "=alerts",
        displayName: "@"
      },
      templateUrl: "views/directives/delete-button.html",
      link: function(scope, element, attrs) {
        // make resource types available in the modal
        scope.resourceType = attrs.resourceType;
        scope.resourceName = attrs.resourceName;
        scope.displayName = attrs.displayName;

        if (attrs.resourceType === 'project') {
          scope.isProject = true;
          scope.biggerButton = true;
        }

        // make the resource types show up nicely in the modal
        scope.nameFormatMap = {
          'imagestream': 'Image Stream',
          'pod': 'Pod',
          'service': 'Service',
          'buildconfig': 'Build Config',
          'deploymentconfig': 'Deployment Config',
          'project': 'Project'
        };

        scope.openDeleteModal = function() {
          // get error details
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
          // opening the modal with settings scope as parent
          var modalInstance = $modal.open({
            animation: true,
            templateUrl: 'views/modals/delete-resource.html',
            controller: 'DeleteModalController',
            scope: scope
          });

          modalInstance.result.then(function() {
          // upon clicking delete button, delete resource and send alert
            var resourceType = attrs.resourceType;
            var resourceName = attrs.resourceName;
            var formattedResource = scope.nameFormatMap[resourceType] + ' ' + "\'"  + (attrs.displayName ? attrs.displayName : resourceName) + "\'";

            DataService.delete(resourceType + 's', resourceName, scope.$parent)
            .then(function() {
              scope.alerts[resourceName] = {
                type: "success",
                message: formattedResource + " was marked for deletion."
              };
            })
            .catch(function(err) {
              // called if failure to delete
              scope.alerts[resourceName] = {
                type: "error",
                message: formattedResource + "\'" + " could not be deleted.",
                details: getErrorDetails(err)
              };
              Logger.error(formattedResource + " could not be deleted.", err);
            })
            .then(function() {
              // for deleting routes associated with a service
              if (resourceType === 'project') {
                if ($location.path() === '/') {
                  scope.$emit('deleteProject');
                }
                else if ($location.path().indexOf('settings') > '-1') {
                  var redirect = URI('/');
                  AlertMessageService.addAlert({
                    name: resourceName,
                    data: {
                      type: "success",
                      message: formattedResource + " was marked for deletion."
                    }
                  });
                  $location.url(redirect);
                }
              }
            });
          });
        };
      }
    };
  });