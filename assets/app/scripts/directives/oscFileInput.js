'use strict';

angular.module('openshiftConsole')
  .directive('oscFileInput', function(Logger) {
    return {
      restrict: 'E',
      scope: {
        model: "=",
        required: "=",
        disabled: "=ngDisabled",
        helpText: "@?"
      },
      templateUrl: 'views/directives/osc-file-input.html',
      link: function(scope, element){
        scope.helpID = _.uniqueId('help-');
        scope.supportsFileUpload = (window.File && window.FileReader && window.FileList && window.Blob);
        scope.uploadError = false;
        $(element).change(function(){
          var file = $('input[type=file]', this)[0].files[0];
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
            Logger.error("Could not read file", e);
          };
          reader.readAsText(file);
        });
      }
    };
  });
