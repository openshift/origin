'use strict';

angular.module('openshiftConsole')
  .directive('labels', function() {
    return {
      restrict: 'E',
      templateUrl: 'views/_labels.html',
      scope: {
        labels: '='
      },
    };
  })
  .directive('labelValidator', function() {
    return {
      restrict: 'A',
      require: 'ngModel',
      link: function(scope, elm, attrs, ctrl) {
        ctrl.$validators.label = function(modelValue, viewValue) {
          var LABEL_REGEXP = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;
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

          if (ctrl.$isEmpty(modelValue)) {
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
        };
      }
    };
  });
