'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ProjectsController
 * @description
 * # ProjectsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ProjectsController', function ($scope, $location, DataService, AuthService) {   
    $scope.projects = {};
    $scope.alerts = $scope.alerts || {};
    $scope.emptyMessage = "Loading...";

    AuthService.withUser().then(function() {
      DataService.list("projects", $scope, function(projects) {
        $scope.projects = projects.by("metadata.name");
        $scope.emptyMessage = "You have no projects. For an <a href='https://github.com/openshift/origin/tree/master/examples/sample-app'>example</a>, run <code>openshift cli create -f https://raw.githubusercontent.com/openshift/origin/master/examples/sample-app/project.json</code>";
      });
    });
  
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
