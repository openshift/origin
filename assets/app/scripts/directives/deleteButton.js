'use strict';

angular.module("openshiftConsole")
  .directive("deleteButton", function ($modal, $location, $filter, hashSizeFilter, DataService, AlertMessageService, Navigate, Logger) {
    return {
      restrict: "E",
      scope: {
        resourceType: "@",
        resourceName: "@",
        projectName: "@",
        alerts: "=",
        displayName: "@"
      },
      templateUrl: "views/directives/delete-button.html",
      link: function(scope, element, attrs) {
        // make resource types available in the modal
        scope.resourceType = attrs.resourceType;
        scope.resourceName = attrs.resourceName;
        scope.projectName = attrs.projectName;
        scope.displayName = attrs.displayName;

        if (attrs.resourceType === 'project') {
          scope.isProject = true;
          // scope.biggerButton = true;
        }

        scope.openDeleteModal = function() {
          // opening the modal with settings scope as parent
          var modalInstance = $modal.open({
            animation: true,
            templateUrl: 'views/modals/delete-resource.html',
            controller: 'DeleteModalController',
            scope: scope
          });

          modalInstance.result.then(function() {
          // upon clicking delete button, delete resource and send alert
            var resourceType = scope.resourceType;
            var resourceName = scope.resourceName;
            var projectName = scope.projectName;
            var formattedResource = $filter('humanizeResourceType')(resourceType) + ' ' + "\'"  + (scope.displayName ? scope.displayName : resourceName) + "\'";

            DataService.delete(resourceType + 's', resourceName, scope.$parent)
            .then(function() {
              if (resourceType !== 'project') {
                AlertMessageService.addAlert({
                  name: resourceName,
                  data: {
                    type: "success",
                    message: formattedResource + " was marked for deletion."
                  }
                });
                Navigate.toResourceList(resourceType, projectName);
              }
              else {
                if ($location.path() === '/') {
                  scope.$emit('deleteProject');
                }
                else if ($location.path().indexOf('settings') > '-1') {
                  var homeRedirect = URI('/');
                  AlertMessageService.addAlert({
                    name: resourceName,
                    data: {
                      type: "success",
                      message: formattedResource + " was marked for deletion."
                    }
                  });
                  $location.url(homeRedirect);
                }
              }
            })
            .catch(function(err) {
              // called if failure to delete
              scope.alerts[resourceName] = {
                type: "error",
                message: formattedResource + "\'" + " could not be deleted.",
                details: $filter('getErrorDetails')(err)
              };
              Logger.error(formattedResource + " could not be deleted.", err);
            });
          });
        };
      }
    };
  });
