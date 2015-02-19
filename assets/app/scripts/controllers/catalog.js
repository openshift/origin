'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CatalogController', function ($scope, DataService, $filter, LabelFilter) {
    $scope.projectTemplates = {};
    $scope.openshiftTemplates = {};

    $scope.templatesByTag = {};
    $scope.templates = [];

    $scope.instantApps = [];

    DataService.list("templates", $scope, function(templates) {
      $scope.projectTemplates = templates.by("metadata.name");
      allTemplates();
      templatesByTag();
      console.log("project templates", $scope.projectTemplates);
    });

    DataService.list("templates", {namespace: "openshift"}, function(templates) {
      $scope.openshiftTemplates = templates.by("metadata.name");
      allTemplates();
      templatesByTag();
      console.log("openshift templates", $scope.openshiftTemplates);
    });

    var allTemplates = function() {
      $scope.templates = [];
      angular.forEach($scope.projectTemplates, function(template) {
        $scope.templates.push(template);
      });
      angular.forEach($scope.openshiftTemplates, function(template) {
        $scope.templates.push(template);
      });
    };

    var templatesByTag = function() {
      $scope.templatesByTag = {};
      angular.forEach($scope.templates, function(template) {
        if (template.metadata.annotations && template.metadata.annotations.tags) {
          var tags = template.metadata.annotations.tags.split(",");
          angular.forEach(tags, function(tag){
            tag = $.trim(tag);
            // not doing this as a map since we are dealing with things across namespaces that could have collisions on name
            $scope.templatesByTag[tag] = $scope.templatesByTag[tag] || [];
            $scope.templatesByTag[tag].push(template);
          });
        }
      });

      console.log("templatesByTag", $scope.templatesByTag);
    };
  });
