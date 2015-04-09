"use strict";

angular.module("openshiftConsole")
  .directive("oscResourceNameValidator", function(){
    
    //github.com/GoogleCloudPlatform/kubernetes/pkg/util/validation.go
    //limiting to valid service names as LCD
    var maxNameLength = 24;
    var VALID_NAME_RE = /^[a-z]([-a-z0-9]*[a-z0-9])?/i;
    return {

      require: "ngModel",
      link: function(scope, elm, attrs, ctrl) {
        ctrl.$validators.oscResourceNameValidator = function(modelValue, viewValue){
          if(ctrl.$isEmpty(modelValue)){
            return false;
          }
          if(viewValue === null){
            return false;
          }
          if(ctrl.$isEmpty(viewValue.trim())){
            return false;
          }
          if(modelValue.length <= maxNameLength){
            
            if(VALID_NAME_RE.test(viewValue) && viewValue.indexOf(" ") === -1){
              return true;
            }
            
          }
          return false;
        };
      }
    };
  });