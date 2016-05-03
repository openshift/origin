'use strict';

angular.module("openshiftConsole")
  .directive("fromFile", function($q,
                                  $uibModal,
                                  $location,
                                  $filter,
                                  CachedTemplateService,
                                  AlertMessageService,
                                  Navigate,
                                  TaskList,
                                  DataService,
                                  APIService) {
    return {
      restrict: "E",
      scope: false,
      templateUrl: "views/directives/from-file.html",
      controller: function($scope) {
        var aceEditorSession;
        var humanizeKind = $filter('humanizeKind');
        TaskList.clear();

        $scope.aceLoaded = function(editor) {
          aceEditorSession = editor.getSession();
          aceEditorSession.setOption('tabSize', 2);
          aceEditorSession.setOption('useSoftTabs', true);
          editor.setDragDelay = 0;
          editor.$blockScrolling = Infinity;
        };

        var checkErrorAnnotations = function() {
          var editorAnnotations = aceEditorSession.getAnnotations();
          $scope.editorErrorAnnotation = _.some(editorAnnotations, { type: 'error' });
        };


        // Determine whats the input format (JSON/YAML) and set appropriate view mode
        var updateEditorMode = _.debounce(function(){
          try {
            JSON.parse($scope.editorContent);
            aceEditorSession.setMode("ace/mode/json");
          } catch (e) {
            try {
              YAML.parse($scope.editorContent);
              aceEditorSession.setMode("ace/mode/yaml");
            } catch (e) {}
          }
          $scope.$apply(checkErrorAnnotations);
        }, 300);

        // Check if the editor isn't empty to disable the 'Add' button. Also check in what format the input is in (JSON/YAML) and change
        // the editor accordingly.
        $scope.aceChanged = updateEditorMode;

        $scope.create = function() {
          delete $scope.alerts['create'];
          delete $scope.error;
          var resource;

          // Trying to auto-detect what format the input is in. Since parsing JSON throws only SyntexError
          // exception if the string to parse is not valid JSON, it is tried first and then the YAML parser
          // is trying to parse the string. If that fails it will print the reason. In case the real reason
          // is JSON related the printed reason will be "Reason: Unable to parse", in case of YAML related
          // reason the true reason will be printed, since YAML parser throws an error object with needed
          // data.
          try {
            resource = JSON.parse($scope.editorContent);
          } catch (e) {
            try {
              resource = YAML.parse($scope.editorContent);
            } catch (e) {
              $scope.error = e;
              return;
            }
          }

          // Top level resource field check. 
          if (!validateFields(resource)) {
            return;
          }

          $scope.resourceKind = resource.kind;
          
          if ($scope.resourceKind.endsWith("List")) {
            $scope.isList = true;
            $scope.resourceList = resource.items;
            $scope.resourceName = '';
          } else {
            $scope.resourceList = [resource];
            $scope.resourceName = resource.metadata.name;
            if ($scope.resourceKind === "Template") {
              $scope.templateOptions = {
                process: true,
                add: false
              };
            }
          }

          $scope.updateResources = [];
          $scope.createResources = [];

          var resourceCheckPromises = [];
          $scope.errorOccured = false;
          _.forEach($scope.resourceList, function(item) {
            if (!validateFields(item)) {
              $scope.errorOccured = true;
              return false;
            }
            resourceCheckPromises.push(checkIfExists(item));
          });

          if ($scope.errorOccured) {
            return;
          }

          $q.all(resourceCheckPromises).then(function() {
            // If resource if Template and it doesn't exist in the project
            if ($scope.createResources.length === 1 && $scope.resourceList[0].kind === "Template") {
              openTemplateProcessModal();
            // Else if any resources already exist
            } else if (!_.isEmpty($scope.updateResources)) {
              confirmReplace();
            } else {
              createAndUpdate();
            }
          });
        };

        // Takes item that will be inspect for kind, metadata fields and if the item is meant to be created in current namespace 
        function validateFields(item) {
          if (!item.kind) {
            $scope.error = {
              message: "Resource is missing kind field."
            };
            return false;
          }
          if (!item.metadata) {
            $scope.error = {
              message: "Resource is missing metadata field."
            };
            return false;
          }
          if (!item.metadata.name) {
            $scope.error = {
              message: "Resource name is missing in metadata field."
            };
            return false;
          }
          if (item.metadata.namespace && item.metadata.namespace !== $scope.projectName) {
            $scope.error = {
              message: item.kind + " " + item.metadata.name + " can't be created in project " + item.metadata.namespace + ". Can't create resource in different projects."
            };
            return false;
          }
          return true;
        }

        function openTemplateProcessModal() {
          var modalInstance = $uibModal.open({
            animation: true,
            templateUrl: 'views/modals/process-template.html',
            controller: 'ProcessTemplateModalController',
            scope: $scope
          });
          modalInstance.result.then(function() {
            if ($scope.templateOptions.add) {
              createAndUpdate();
            } else {
              CachedTemplateService.setTemplate($scope.resourceList[0]);
              redirect();
            }
          });      
        }

        function confirmReplace() {
          var modalInstance = $uibModal.open({
            animation: true,
            templateUrl: 'views/modals/confirm-replace.html',
            controller: 'ConfirmReplaceModalController',
            scope: $scope
          });
          modalInstance.result.then(function() {
            createAndUpdate();
          });
        }

        function createAndUpdate() {
          var createResourcesSum = $scope.createResources.length,
            updateResourcesSum = $scope.updateResources.length;
            
          if (!$scope.resourceKind.endsWith("List")) {
            creatUpdateSingleResource();
          } else {
            var createUpdatePromises = [];
            if (updateResourcesSum > 0) {
              createUpdatePromises.push(updateResourceList());
            }
            if (createResourcesSum > 0) {
              createUpdatePromises.push(createResourceList());
            }
            $q.all(createUpdatePromises).then(redirect);
          }
        }

        // Redirect to newFromTemplate page in case the resource type is Template and user wants to process it.
        // When redirecting to newFromTemplate page, use the cached Template if user doesn't adds it into the
        // namespace by the create process or if the template is being updated.
        function redirect() {
          var path;
          if ($scope.resourceKind === "Template" && $scope.templateOptions.process && !$scope.errorOccured) {
            var namespace = ($scope.templateOptions.add || $scope.updateResources.length > 0) ? $scope.projectName : "";
            path = Navigate.fromTemplateURL($scope.projectName, $scope.resourceName, namespace);
          } else {
            path = Navigate.projectOverviewURL($scope.projectName);
          }
          $location.url(path);
        }

        function checkIfExists(item) {
          // Check if the resource already exists. If it does, replace it spec with the new one.
          return DataService.get(APIService.kindToResource(item.kind), item.metadata.name, $scope.context, {errorNotification: false}).then(
            // resource does exist
            function(resource) {
              if (item.kind === "Template") {
                resource.metadata.annotations = item.metadata.annotations;
              } else {
                resource.spec = item.spec;
              }
              $scope.updateResources.push(resource);
            },
            // resource doesn't exist with RC 404 or catch other RC 
            function(response) {
              if (response.status === 404) {
                $scope.createResources.push(item);
              } else {
                $scope.alerts["check"] = {
                  type: "error",
                  message: "An error occurred checking if the " + humanizeKind(item.kind) + " " + item.metadata.name + " already exists.",
                  details: "Reason: " + $filter('getErrorDetails')(response)
                };
                $scope.errorOccured = true;
              }
          });
        }

        // creatUpdateSingleResource function will create/update just a single resource on a none-List resource kind. 
        function creatUpdateSingleResource() {
          var resource;
          if (!_.isEmpty($scope.createResources)) {
            resource = _.head($scope.createResources);
            DataService.create(APIService.kindToResource(resource.kind), null, resource, {namespace: $scope.projectName}).then(
              // create resource success
              function() {
                AlertMessageService.addAlert({
                  name: resource.metadata.name,
                  data: {
                    type: "success",
                    message: resource.kind + " " + resource.metadata.name + " was successfully created."
                  }
                });
                redirect();
              },
              // create resource failure
              function(result) {
                $scope.error = {
                  message: $filter('getErrorDetails')(result)
                };
              });
          } else {
            resource = _.head($scope.updateResources);
            DataService.update(APIService.kindToResource(resource.kind), resource.metadata.name, resource, {namespace: $scope.projectName}).then(
              // update resource success
              function() {
                AlertMessageService.addAlert({
                  name: resource.metadata.name,
                  data: {
                    type: "success",
                    message: resource.kind + " " + resource.metadata.name + " was successfully updated."
                  }
                });
                redirect();
              },
              // update resource failure
              function(result) {
                $scope.error = {
                  message: $filter('getErrorDetails')(result)
                };
              });
          }

        }

        function createResourceList(){
          var titles = {
            started: "Creating resources in project " + $scope.projectName,
            success: "Creating resources in project " + $scope.projectName,
            failure: "Failed to create some resources in project " + $scope.projectName
          };
          var helpLinks = {};
          TaskList.add(titles, helpLinks, function() {
            var d = $q.defer();

            DataService.batch($scope.createResources, $scope.context, "create").then(
              function(result) {
                var alerts = [];
                var hasErrors = false;
                if (result.failure.length > 0) {
                  hasErrors = true;
                  $scope.errorOccured = true;
                  result.failure.forEach(
                    function(failure) {
                      alerts.push({
                        type: "error",
                        message: "Cannot create " + humanizeKind(failure.object.kind) + " \"" + failure.object.metadata.name + "\". ",
                        details: failure.data.message
                      });
                    }
                  );
                  result.success.forEach(
                    function(success) {
                      alerts.push({
                        type: "success",
                        message: "Created " + humanizeKind(success.kind) + " \"" + success.metadata.name + "\" successfully. "
                      });
                    }
                  );
                } else {
                  var alertMsg;
                  if ($scope.isList) {
                    alertMsg = "All items in list were created successfully.";
                  } else {
                    alertMsg = humanizeKind($scope.resourceKind) + " " + $scope.resourceName + " was successfully created.";
                  }
                  alerts.push({ type: "success", message: alertMsg});
                }
                d.resolve({alerts: alerts, hasErrors: hasErrors});
              }
            );
            return d.promise;
          });
        }


        function updateResourceList(){
          var titles = {
            started: "Updating resources in project " + $scope.projectName,
            success: "Updated resources in project " + $scope.projectName,
            failure: "Failed to update some resources in project " + $scope.projectName
          };
          var helpLinks = {};
          TaskList.add(titles, helpLinks, function() {
            var d = $q.defer();

            DataService.batch($scope.updateResources, $scope.context, "update").then(
              function(result) {
                var alerts = [];
                var hasErrors = false;
                if (result.failure.length > 0) {
                  hasErrors = true;
                  $scope.errorOccured = true;
                  result.failure.forEach(
                    function(failure) {
                      alerts.push({
                        type: "error",
                        message: "Cannot update " + humanizeKind(failure.object.kind) + " \"" + failure.object.metadata.name + "\". ",
                        details: failure.data.message
                      });
                    }
                  );
                  result.success.forEach(
                    function(success) {
                      alerts.push({
                        type: "success",
                        message: "Updated " + humanizeKind(success.kind) + " \"" + success.metadata.name + "\" successfully. "
                      });
                    }
                  );
                } else {
                  var alertMsg;
                  if ($scope.isList) {
                    alertMsg = "All items in list were updated successfully.";
                  } else {
                    alertMsg = humanizeKind($scope.resourceKind) + " " + $scope.resourceName + " was successfully updated.";
                  }
                  alerts.push({ type: "success", message: alertMsg});
                }
                d.resolve({alerts: alerts, hasErrors: hasErrors});
              },
              function(result) {
                var alerts = [];
                alerts.push({
                    type: "error",
                    message: "An error occurred updating the resources.",
                    details: "Status: " + result.status + ". " + result.data
                  });
                d.resolve({alerts: alerts});
              }
            );
            return d.promise;
          });
        }
      }
    };
  });
