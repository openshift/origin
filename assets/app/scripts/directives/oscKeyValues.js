"use strict";
/* jshint unused: false */

angular.module("openshiftConsole")
  .controller("KeyValuesEntryController", function($scope){
    $scope.editing = false;
    $scope.edit = function(){
      $scope.originalValue = $scope.value;
      $scope.editing = true;
    };
    $scope.cancel= function(){
      $scope.value = $scope.originalValue;
      $scope.editing = false;
    };
    $scope.update = function(key, value, entries){
      if(value){
        entries[key] = value;
        $scope.editing = false;
      }
    };
  })
  .controller("KeyValuesController", function($scope){
    var added = {};
    $scope.allowDelete = function(value){
      if($scope.deletePolicy === "never") {
        return false;
      }
      if($scope.deletePolicy === "added"){
        return added[value] !== undefined;
      }
      return true;
    };
    $scope.addEntry = function() {
      if($scope.key && $scope.value){
       var readonly = $scope.readonlyKeys.split(",");
       if(readonly.indexOf($scope.key) !== -1){
         return;
       }
       added[$scope.key] = "";
       $scope.entries[$scope.key] = $scope.value;
       $scope.key = null;
       $scope.value = null;
       $scope.form.$setPristine();
       $scope.form.$setUntouched();
       $scope.form.$setValidity();
      }
     };
     $scope.deleteEntry = function(key) {
       if ($scope.entries[key]) {
         delete $scope.entries[key];
         delete added[key];
       }
     };
  })
  .directive("oscInputValidator", function(){

    var validators = {
      always: function(modelValue, viewValue){
        return true;
      },
      env: function(modelValue, viewValue){
        var C_IDENTIFIER_RE = /^[A-Za-z_][A-Za-z0-9_]*$/i;
        if(modelValue === undefined || modelValue === null || modelValue.trim().length === 0) {
          return true;
        }
        return C_IDENTIFIER_RE.test(viewValue);
      },
      label: function(modelValue, viewValue) {
          var LABEL_REGEXP = /^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$/;
          var LABEL_MAXLENGTH = 63;
          var SUBDOMAIN_REGEXP = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;
          var SUBDOMAIN_MAXLENGTH = 253;

          function validateSubdomain(str) {
            if (str.length > SUBDOMAIN_MAXLENGTH) { return false; }
            return SUBDOMAIN_REGEXP.test(str);
          }

          function validateLabel(str) {
            if (str.length > LABEL_MAXLENGTH) { return false; }
            return LABEL_REGEXP.test(str);
          }

          if (modelValue === undefined || modelValue === null || modelValue.trim().length === 0) {
            return true;
          }
          var parts = viewValue.split("/");
          switch(parts.length) {
            case 1:
              return validateLabel(parts[0]);
            case 2:
              return validateSubdomain(parts[0]) && validateLabel(parts[1]);
          }
          return false;
        }
    };
    return {
      require: ["ngModel", "^oscKeyValues"],
      restrict: "A",
      link: function(scope, elm, attrs, controllers) {
        var ctrl = controllers[0];
        var oscKeyValues = controllers[1];
        if(attrs.oscInputValidator === 'key'){
          ctrl.$validators.oscKeyValid = validators[oscKeyValues.scope.keyValidator];
        }else if(attrs.oscInputValidator === 'value'){
          ctrl.$validators.oscValueValid = validators[oscKeyValues.scope.valueValidator];
        }
      }
    };
  })
  /**
   * A Directive for displaying key/value pairs.  Configuration options
   * via attributes:
   * delimiter:   the value to use to separate key/value pairs when displaying
   *              (e.g. foo:bar).  Default: ":"
   * keyTitle:    The value to use as the key input's placeholder. Default: Name
   * editable:    true if the intention is to display values only otherwise false (default)
   * keyValidaor: The validator to use for validating keys
   *   - always: Any value is allowed (Default).
   *   - env:    Validate as an ENV var /^[A-Za-z_][A-Za-z0-9_]*$/i
   *   - label:  Validate as a label
   * deletePolicy:
   *  - always: allow any key/value pair (Default)
   *  - added:  allow any added not originally in entries
   *  - never:  disallow any entries being deleted
   * readonlyKeys:  A comma delimted list of keys that are readonly
   * keyValidationTooltip: The tool tip to display when the key validation message is visible
   */
  .directive("oscKeyValues", function() {
    return {
      restrict: "E",
      scope: {
        keyTitle: "@",
        entries: "=",
        delimiter: "@",
        editable: "@",
        keyValidator: "@",
        valueValidator: "@",
        deletePolicy: "@",
        readonlyKeys: "@",
        keyValidationTooltip: "@",
        valueValidationTooltip: "@"
      },
      controller: function($scope){
        this.scope = $scope;
      },
      templateUrl: "views/directives/osc-key-values.html",
      compile: function(element, attrs){
        if(!attrs.delimiter){attrs.delimiter = ":";}
        if(!attrs.keyTitle){attrs.keyTitle = "Name";}
        if(!attrs.editable || attrs.editable === "true"){
          attrs.editable = true;
        }else{
          attrs.editable = false;
        }
        if(!attrs.keyValidator){attrs.keyValidator = "always";}
        if(!attrs.valueValidator){attrs.valueValidator = "always";}
        if(["always", "added", "none"].indexOf(attrs.deletePolicy) === -1){
          attrs.deletePolicy = "always";
        }
        if(!attrs.readonlyKeys){
          attrs.readonlyKeys = "";
        }
      }
    };
  });
