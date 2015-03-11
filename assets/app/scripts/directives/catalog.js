'use strict';

angular.module('openshiftConsole')
  .directive('catalogTemplate', function($location) {
    return {
      restrict: 'E',
      scope: {
        template: '=',
        project: '='
      },
      templateUrl: 'views/catalog/_template.html',
      link: function(scope, elem, attrs) {
        $(".select-template", elem).click(function() {
          // Must trigger off of the modal's hidden event to guarantee modal has finished closing before switching screens
          $(".modal", elem).on('hidden.bs.modal', function () {
            scope.$apply(function() {
              var createURI = URI.expand("/project/{project}/create/fromtemplate{?q*}", {
                project: scope.project,
                q: {
                  name: scope.template.metadata.name,
                  namespace: scope.template.metadata.namespace
                }
              });
              $location.url(createURI.toString());
            });
          })
          .modal('hide');

        });
      }
    };
  });
