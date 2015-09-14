'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CreateController', function ($scope, DataService, AuthService, Upload, tagsFilter, uidFilter, createFromSourceURLFilter, LabelFilter, $location, Logger) {
    var projectTemplates;
    var openshiftTemplates;

    // for creating with auth
    AuthService.withUser();

    // alerts
    $scope.alerts = $scope.alerts || {};

    // Templates with the `instant-apps` tag.
    $scope.instantApps = undefined;

    // All templates from the shared or project namespace that aren't instant apps.
    // This is displayed in the "Other Templates" section.
    $scope.otherTemplates = undefined;

    // Set to true when shared templates and project templates have finished loading.
    $scope.templatesLoaded = false;

    $scope.sourceURLPattern = /^((ftp|http|https|git):\/\/(\w+:{0,1}[^\s@]*@)|git@)?([^\s@]+)(:[0-9]+)?(\/|\/([\w#!:.?+=&%@!\-\/]))?$/;

    function loadTemplates() {
      DataService.list("templates", $scope, function(templates) {
        projectTemplates = templates.by("metadata.name");
        updateTemplates();
        Logger.info("project templates", projectTemplates);
      });

      DataService.list("templates", {namespace: "openshift"}, function(templates) {
        openshiftTemplates = templates.by("metadata.name");
        updateTemplates();
        Logger.info("openshift templates", openshiftTemplates);
      });
    }

    loadTemplates();

    function isInstantApp(template) {
      var i, tags = tagsFilter(template);
      for (i = 0; i < tags.length; i++) {
        if (tags[i] === "instant-app") {
          return true;
        }
      }

      return false;
    }

    function updateTemplates() {
      // Check if we've loaded templates from both the openshift and project namespaces.
      $scope.templatesLoaded = openshiftTemplates && projectTemplates;
      $scope.instantApps = {};
      $scope.otherTemplates = {};

      // Categorize templates as instant apps or "other."
      var categorizeTemplates = function(template) {
        var uid = uidFilter(template);
        if (isInstantApp(template)) {
          $scope.instantApps[uid] = template;
        } else {
          $scope.otherTemplates[uid] = template;
        }
      };

      angular.forEach(projectTemplates, categorizeTemplates);
      angular.forEach(openshiftTemplates, categorizeTemplates);

      Logger.info("instantApps", $scope.instantApps);
      Logger.info("otherTemplates", $scope.otherTemplates);
    }

    $scope.createFromSource = function() {
      if($scope.from_source_form.$valid) {
        var createFromSourceURL = createFromSourceURLFilter($scope.projectName, $scope.from_source_url);
        $location.url(createFromSourceURL);
      }
    };

    // watch
    $scope.$on('refreshList', function() {
      loadTemplates();
    });
  });
