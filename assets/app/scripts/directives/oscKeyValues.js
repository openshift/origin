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
        },
      path: function(modelValue, viewValue) {
        var ABS_PATH_REGEXP = /^\//;
        if (modelValue === undefined || modelValue === null || modelValue.trim().length === 0) {
          return true;
        }
        return ABS_PATH_REGEXP.test(viewValue);
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
   * ValueTitle:  The value to use as the value input's placeholder. Default: Value
   * editable:    true if the intention is to display values only otherwise false (default)
   * keyValidator: The validator to use for validating keys
   *   - always: Any value is allowed (Default).
   *   - env:    Validate as an ENV var /^[A-Za-z_][A-Za-z0-9_]*$/i
   *   - label:  Validate as a label
   *   - path:   Validate as an absolute path
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
        valueTitle: "@",
        entries: "=",
        delimiter: "@",
        editable: "@",
        keyValidator: "@",
        valueValidator: "@",
        deletePolicy: "@",
        readonlyKeys: "@",
        keyValidationTooltip: "@",
        valueValidationTooltip: "@",
        preventEmpty: "=?"
      },
      controller: function($scope){
        var focusElem;
        var added = {};
        var isUncommitted = function() {
          return (!!$scope.key) || (!!$scope.value);
        };
        var checkCommitted = function() {
          if(isUncommitted()) {
            $scope.showCommmitWarning = true;
          } else {
            $scope.showCommmitWarning = false;
          }
        };
        // checks if the key,value inputs have any text value.
        // if so, sets the form name="clean" to an 'invalid' state, which will
        // invalidate any parent form up the chain.  This should result in an
        // inability to submit that form until the user commits the new key-value pair.
        var isClean = _.debounce(function() {
          $scope.$applyAsync(function() {
            if(!!$scope.key) {
              $scope.clean.isClean.$setValidity('isClean', false);
            } else if(!!$scope.value) {
              $scope.clean.isClean.$setValidity('isClean', false);
            } else {
              $scope.clean.isClean.$setValidity('isClean', true);
            }
          });
        }, 200);

        // returns a new function bound to the set of DOM nodes provided as an array.
        // allows us to treat the osc-key-values directive as a single node on blur
        // events, though it is actually made up of a number of nodes.  If any of the
        // provided nodes is the document.activeElement, we know the osc-key-values
        // directive as a whole still has focus. When the document.activeElement no
        // longer matches a child node, we can test to see if there are uncommitted
        // values left in the osc-key-values directive & notify the user if so.
        var onBlur = function(nodes) {
          return function(evt) {
            $scope.$applyAsync(function() {
              if(!_.includes(nodes, document.activeElement)) {
                checkCommitted();
                isClean();
              }
            });
          };
        };
        $scope.isClean = isClean;

        $scope.clear = function() {
          $scope.key = '';
          $scope.value = '';
          checkCommitted();
          isClean();
        };

        $scope.allowDelete = function(value){
          if ($scope.preventEmpty && (Object.keys($scope.entries).length === 1)) {
            return false;
          }
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
            checkCommitted();
            isClean();
            focusElem.focus();
          }
        };

        $scope.deleteEntry = function(key) {
          if ($scope.entries[key]) {
            delete $scope.entries[key];
            delete added[key];
            $scope.form.$setDirty();
          }
        };

        $scope.setErrorText = function(keyTitle) {
          if (keyTitle === 'path') {
            return "absolute path";
          } else if (keyTitle === 'label') {
            return "label";
          } else {
            return "key";
          }
        };

        this.scope = $scope;

        this.init = function(keyInput, valInput, submitBtn) {
          var nodes = [keyInput[0], valInput[0], submitBtn[0]];
          var boundBlur = onBlur(nodes);
          focusElem = keyInput;
          keyInput.on('blur', boundBlur);
          valInput.on('blur', boundBlur);
          submitBtn.on('blur', boundBlur);

          $scope.$on('$destroy', function() {
            keyInput.off('blur', boundBlur);
            valInput.off('blur', boundBlur);
            submitBtn.off('blur', boundBlur);
          });
        };
      },
      templateUrl: "views/directives/osc-key-values.html",
      compile: function(element, attrs){
        if(!attrs.delimiter){attrs.delimiter = ":";}
        if(!attrs.keyTitle){attrs.keyTitle = "Name";}
        if(!attrs.valueTitle){attrs.valueTitle = "Value";}
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
        return {
          post: function($scope, $elem, $attrs, ctrl) {
            ctrl.init(
                $elem.find('input[name="key"]'),
                $elem.find('input[name="value"]'),
                $elem.find('a.add-key-value')
              );
          }
        };
      }
    };
  });
