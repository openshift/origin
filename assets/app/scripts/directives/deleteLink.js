'use strict';

angular.module("openshiftConsole")
  .directive("deleteLink", function ($uibModal, $location, $filter, hashSizeFilter, DataService, AlertMessageService, Navigate, Logger) {
    return {
      restrict: "E",
      scope: {
        resourceType: "@",
        // Optional display name for resourceType. If not specified, humanizeResourceTypeFilter is used.
        typeDisplayName: "@?",
        resourceName: "@",
        projectName: "@",
        alerts: "=",
        displayName: "@",
        // Set to true to disable the delete button.
        disableDelete: "=?",
        // Only show the button and no text.
        buttonOnly: "@"
      },
      templateUrl: function(elem, attr) {
        if (angular.isDefined(attr.buttonOnly)) {
          return "views/directives/delete-button.html";
        }

        return "views/directives/delete-link.html";
      },
      // Replace so ".dropdown-menu > li > a" styles are applied.
      replace: true,
      link: function(scope, element, attrs) {

        if (attrs.resourceType === 'project') {
          scope.isProject = true;
          // scope.biggerButton = true;
        }

        scope.openDeleteModal = function() {
          if (scope.disableDelete) {
            return;
          }

          // opening the modal with settings scope as parent
          var modalInstance = $uibModal.open({
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
            var typeDisplayName = scope.typeDisplayName || $filter('humanizeResourceType')(resourceType);
            var formattedResource = typeDisplayName + ' ' + "\'"  + (scope.displayName ? scope.displayName : resourceName) + "\'";
            var context = (scope.resourceType === 'project') ? {} : {namespace: scope.projectName};

            DataService.delete(resourceType + 's', resourceName, context)
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

