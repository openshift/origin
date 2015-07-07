'use strict';

angular.module('openshiftConsole')
  .directive('oscFileInput', function(Logger) {
    return {
      restrict: 'E',
      scope: {
        name: "@",
        model: "=",
        required: "="
      },
      templateUrl: 'views/directives/osc-file-input.html',
      link: function(scope, element){
        scope.supportsFileUpload = (window.File && window.FileReader && window.FileList && window.Blob);
        scope.uploadError = false;
        $(element).change(function(){
          var file = $('input[type=file]',this)[0].files[0];
          var reader = new FileReader();
          reader.onloadend = function(){
            scope.$apply(function(){
              scope.fileName = file.name;
              scope.model = reader.result;
            });
          };
          reader.onerror = function(e){
            scope.supportsFileUpload = false;
            scope.uploadError = true;
            Logger.error(e);
          };
//          reader.readAsBinaryString(file);
          reader.onerror();
        });
      }
    };
  });
