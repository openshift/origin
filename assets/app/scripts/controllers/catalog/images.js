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
    $scope.builders = [];
    $scope.nonBuilders = [];
    $scope.sourceURL = $routeParams.builderfor;
    $scope.loaded = false;

    // Image repositories for this project.
    var projectImageRepos;

    // Image repositories in the openshift namespace.
    var openshiftImageRepos;

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
      var tagAnnotationFilter = $filter('imageStreamTagAnnotation');
      angular.forEach(imageRepos, function(imageRepo) {
        if (imageRepo.status) {
          angular.forEach(imageRepo.status.tags, function(tag) {
            var imageRepoTag = tag.tag;
            var image = {
              imageRepo: imageRepo,
              imageRepoTag: imageRepoTag,
              name: imageRepo.metadata.name + ":" + imageRepoTag,
              version: tagAnnotationFilter(imageRepo, 'version', imageRepoTag)
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

    var checkIfLoaded = function() {
      // Set loaded to true only if both openshift image repos and project
      // image repos have been loaded.
      $scope.loaded = openshiftImageRepos && projectImageRepos;
    };

    DataService.list("imagestreams", $scope, function(imageRepos) {
      projectImageRepos = imageRepos.by("metadata.name");
      imagesForRepos(projectImageRepos, $scope);
      checkIfLoaded();

      Logger.info("project image repos", projectImageRepos);
    });

    DataService.list("imagestreams", {namespace: "openshift"}, function(imageRepos) {
      openshiftImageRepos = imageRepos.by("metadata.name");
      imagesForRepos(openshiftImageRepos, {namespace: "openshift"});
      checkIfLoaded();

      Logger.info("openshift image repos", openshiftImageRepos);
    });
  });
