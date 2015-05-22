'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodsController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CatalogImagesController', function ($scope, DataService, $filter, LabelFilter, imageEnvFilter, $routeParams, Logger) {
    $scope.projectImageRepos = {};
    $scope.openshiftImageRepos = {};
    $scope.builders = [];
    $scope.nonBuilders = [];
    $scope.sourceURL = $routeParams.builderfor;

    var isBuilder = function(imageRepo, tag) {
      if (!imageRepo.spec || !imageRepo.spec.tags) {
        return false;
      }

      var i, imageTagSpec, categoryTags;
      for (i = 0; i < imageRepo.spec.tags.length; i++) {
        imageTagSpec = imageRepo.spec.tags[i];
        if (imageTagSpec.name === tag && imageTagSpec.annotations && imageTagSpec.annotations.tags) {
          categoryTags = imageTagSpec.annotations.tags.split(/\s*,\s*/);
          if (categoryTags.indexOf("builder") >= 0) {
            return true;
          }
        }
      }
    };

    var imagesForRepos = function(imageRepos, scope) {
      angular.forEach(imageRepos, function(imageRepo) {
        if (imageRepo.status) {
          angular.forEach(imageRepo.status.tags, function(tag) {
            var imageRepoTag = tag.tag;
            var image = {
              imageRepo: imageRepo,
              imageRepoTag: imageRepoTag,
              name: imageRepo.metadata.name + ":" + imageRepoTag
            };

            if (isBuilder(imageRepo, imageRepoTag)) {
              $scope.builders.push(image);
            } else {
              $scope.nonBuilders.push(image);
            }
          });
        }
      });
      Logger.info("builders", $scope.builders);
    };

    DataService.list("imageStreams", $scope, function(imageRepos) {
      $scope.projectImageRepos = imageRepos.by("metadata.name");
      imagesForRepos($scope.projectImageRepos, $scope);

      Logger.info("project image repos", $scope.projectImageRepos);
    });

    DataService.list("imageStreams", {namespace: "openshift"}, function(imageRepos) {
      $scope.openshiftImageRepos = imageRepos.by("metadata.name");
      imagesForRepos($scope.openshiftImageRepos, {namespace: "openshift"});

      Logger.info("openshift image repos", $scope.openshiftImageRepos);
    });


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

      Logger.info("templatesByTag", $scope.templatesByTag);
    };
  });
