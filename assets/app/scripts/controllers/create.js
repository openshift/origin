'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CreateController', function ($scope, DataService, $filter, LabelFilter, $location, Logger) {
    $scope.projectTemplates = {};
    $scope.openshiftTemplates = {};

    $scope.templatesByTag = {};

    $scope.sourceURLPattern = /^((ftp|http|https|git):\/\/(\w+:{0,1}[^\s@]*@)|git@)?([^\s@]+)(:[0-9]+)?(\/|\/([\w#!:.?+=&%@!\-\/]))?$/;

    DataService.list("templates", $scope, function(templates) {
      $scope.projectTemplates = templates.by("metadata.name");
      templatesByTag();
      Logger.info("project templates", $scope.projectTemplates);
    });

    DataService.list("templates", {namespace: "openshift"}, function(templates) {
      $scope.openshiftTemplates = templates.by("metadata.name");
      templatesByTag();
      Logger.info("openshift templates", $scope.openshiftTemplates);
    });

    var templatesByTag = function() {
      $scope.templatesByTag = {};
      var fn = function(template) {
        if (template.metadata.annotations && template.metadata.annotations.tags) {
          var tags = template.metadata.annotations.tags.split(",");
          angular.forEach(tags, function(tag){
            tag = $.trim(tag);
            // not doing this as a map since we are dealing with things across namespaces that could have collisions on name
            $scope.templatesByTag[tag] = $scope.templatesByTag[tag] || [];
            $scope.templatesByTag[tag].push(template);
          });
        }
      };

      angular.forEach($scope.projectTemplates, fn);
      angular.forEach($scope.openshiftTemplates, fn);

      Logger.info("templatesByTag", $scope.templatesByTag);
    };

    $scope.createFromSource = function() {
      if($scope.from_source_form.$valid) {
        var createURI = URI.expand("/project/{project}/catalog/images{?q*}", {
          project: $scope.projectName,
          q: {
            builderfor: $scope.from_source_url
          }
        });
        $location.url(createURI.toString());
      }
    };
  });
