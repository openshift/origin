'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ImageController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ImageController', function ($scope, $routeParams, DataService, project, $filter) {
    $scope.imageStream = null;
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};    
    $scope.renderOptions.hideFilterWidget = true;    
    $scope.breadcrumbs = [
      {
        title: "Image Streams",
        link: "project/" + $routeParams.project + "/browse/images"
      },
      {
        title: $routeParams.image
      }
    ];

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("imagestreams", $routeParams.image, $scope).then(
        // success
        function(imageStream) {
          $scope.loaded = true;
          $scope.imageStream = imageStream;

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("imagestreams", $routeParams.image, $scope, function(imageStream, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This image stream has been deleted."
              }; 
            }
            $scope.imageStream = imageStream;
          }));          
        },
        // failure
        function(e) {
          $scope.loaded = true;
          $scope.alerts["load"] = {
            type: "error",
            message: "The image stream details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    }); 
  });
