'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectsController
 * @description
 * # ProjectsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectsController', function ($scope, $location, DataService) {   
    $scope.projects = [];

    var callback = function(projects) {
      $scope.$apply(function(){
        $scope.projects = projects.items;
      });
    };

    DataService.getList("projects", callback, $scope);

    $scope.tileClickHandler = function(evt) {
      var t = $(evt.target);
      if (t && t.is('a')){
        return;
      }
      var tile = t.closest(".tile");
      if (tile) {
        var a = $('a.tile-target', tile)[0];
        if (a) {
          if (evt.which === 2 || evt.ctrlKey || evt.shiftKey) {
            window.open(a.href);
          }
          else {
            // Must use getAttribute or the browser will make the URL absolute before returning it
            var href = a.getAttribute("href");
            if (URI(href).is("absolute")) {
              window.location = href;
            }
            else {
              $location.url(href);
            }
          }
        }
      }
    };
  });
