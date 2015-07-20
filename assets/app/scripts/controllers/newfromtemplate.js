'use strict';
/* jshint unused: false */

/**
 * @ngdoc function
 * @name openshiftConsole.controller:NewFromTemplateController
 * @description
 * # NewFromTemplateController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('NewFromTemplateController', function ($scope, $http, $routeParams, DataService, $q, $location, TaskList, $parse, Navigate, $filter, imageObjectRefFilter, failureObjectNameFilter) {
    var displayNameFilter = $filter('displayName');

    var dcContainers = $parse('spec.template.spec.containers');
    var builderImage = $parse('spec.strategy.sourceStrategy.from || spec.strategy.dockerStrategy.from || spec.strategy.customStrategy.from');
    var outputImage = $parse('spec.output.to');

    function deploymentConfigImages(dc) {
      var images = [];
      var containers = dcContainers(dc);
      if (containers) {
        angular.forEach(containers, function(container) {
          images.push(container.image);
        });
      }
      return images;
    }

    function imageItems(data) {
      var images = [];
      var dcImages = [];
      var outputImages = {};
      angular.forEach(data.objects, function(item) {
        if (item.kind === "BuildConfig") {
          var builder = imageObjectRefFilter(builderImage(item), $scope.projectName);
          if(builder) {
            images.push({ name: builder });
          }
          var output = imageObjectRefFilter(outputImage(item), $scope.projectName);
          if (output) {
            outputImages[output] = true;
          }
        }
        if (item.kind === "DeploymentConfig") {
          dcImages = dcImages.concat(deploymentConfigImages(item));
        }
      });
      dcImages.forEach(function(image) {
        if (!outputImages[image]) {
          images.push({ name: image });
        }
      });
      return images;
    }

    function getHelpLinks(template) {
      var helpLinkName = /^helplink\.(.*)\.title$/;
      var helpLinkURL = /^helplink\.(.*)\.url$/;
      var helpLinks = {};
      for (var attr in template.annotations) {
        var match = attr.match(helpLinkName);
        var link;
        if (match) {
          link = helpLinks[match[1]] || {};
          link.title = template.annotations[attr];
          helpLinks[match[1]] = link;
        }
        else {
          match = attr.match(helpLinkURL);
          if (match) {
            link = helpLinks[match[1]] || {};
            link.url = template.annotations[attr];
            helpLinks[match[1]] = link;
          }
        }
      }
      return helpLinks;
    }

    $scope.projectDisplayName = function() {
      return displayNameFilter(this.project) || this.projectName;
    };

    $scope.templateDisplayName = function() {
      return displayNameFilter(this.template);
    };

    $scope.createFromTemplate = function() {
      DataService.create("processedtemplates", null, $scope.template, $scope).then(
        function(config) { // success
          var titles = {
            started: "Creating " + $scope.templateDisplayName() + " in project " + $scope.projectDisplayName(),
            success: "Created " + $scope.templateDisplayName() + " in project " + $scope.projectDisplayName(),
            failure: "Failed to create " + $scope.templateDisplayName() + " in project " + $scope.projectDisplayName()
          };

          var helpLinks = getHelpLinks($scope.template);
          TaskList.add(titles, helpLinks, function() {
            var d = $q.defer();
            DataService.createList(config.objects, $scope).then(
              function(result) {
                var alerts = [];
                var hasErrors = false;
                if (result.failure.length > 0) {
                  result.failure.forEach(
                    function(failure) {
                      var objectName = failureObjectNameFilter(failure) || "object";
                      alerts.push({
                        type: "error",
                        message: "Cannot create " + objectName + ". ",
                        details: failure.data.message
                      });
                      hasErrors = true;
                    }
                  );
                } else {
                  alerts.push({ type: "success", message: "All items in template " + $scope.templateDisplayName() +
                    " were created successfully."});
                }
                d.resolve({alerts: alerts, hasErrors: hasErrors});
              }
            );
            return d.promise;
          });
          Navigate.toProjectOverview($scope.projectName);
        },
        function(result) { // failure
          var details;
          if (result.data && result.data.message) {
            details = result.data.message;
          }
          $scope.alerts = [
            {
              type: "error",
              message: "An error occurred processing the template.",
              details: details
            }
          ];
        }
      );
    };

    $scope.toggleOptionsExpanded = function() {
      $scope.optionsExpanded = !$scope.optionsExpanded;
    };

    var name = $routeParams.name;
    var namespace = $routeParams.namespace;

    if (!name) {
      Navigate.toErrorPage("Cannot create from template: a template name was not specified.");
      return;
    }

    $scope.emptyMessage = "Loading...";
    $scope.alerts = [];
    $scope.projectName = $routeParams.project;
    $scope.projectPromise = $.Deferred();
    DataService.get("projects", $scope.projectName, $scope).then(function(project) {
      $scope.project = project;
      $scope.projectPromise.resolve(project);
    });

    DataService.get("templates", name, $scope, {namespace: namespace}).then(
      function(template) {
        $scope.template = template;
        $scope.templateImages = imageItems(template);
        $scope.hasParameters = $scope.template.parameters && $scope.template.parameters.length > 0;
        $scope.optionsExpanded = false;
        $scope.templateUrl = template.metadata.selfLink;
        template.labels = template.labels || {};
      },
      function() {
        Navigate.toErrorPage("Cannot create from template: the specified template could not be retrieved.");
      }
    );
  });
