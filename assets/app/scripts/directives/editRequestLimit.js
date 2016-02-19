"use strict";

angular.module('openshiftConsole')
  .directive('computeResource', function($filter) {
    return {
      restrict: 'E',
      require: 'ngModel',
      scope: {
        label: '@',
        type: '@',
        description: '@',
        defaultValue: '=',
        limitRangeMin: '=',
        limitRangeMax: '=',
        maxLimitRequestRatio: '=',
        // If this is a limit, the value of the corresponding request to
        // validate limit >= request.
        request: '='
      },
      templateUrl: 'views/_compute-resource.html',
      link: function(scope, elem, attrs, ngModel) {
        var usageValue = $filter('usageValue');
        var amountAndUnit = $filter('amountAndUnit');
        var humanizeUnit = $filter('humanizeUnit');

        // Create a unique ID for `label for` and `aria-describedby` attributes.
        scope.id = _.uniqueId('compute-resource-');

        // If unit is not already in options, add it.
        var addUnitOption = function(unit) {
          if (!_.some(scope.units, { value: unit })) {
            scope.units.push({
              value: unit,
              label: humanizeUnit(unit, scope.type)
            });
          }
        };

        scope.$watch('defaultValue', function(defaultValue) {
          // Set unit default based on default value or millicores/MiB if no default value.
          // The amount input will have a placeholder using on limit range default.
          var setDefault = _.spread(function(defaultAmount, defaultUnit) {
            scope.placeholder = defaultAmount;
            addUnitOption(defaultUnit);
            // Only change selected unit if no value is set.
            if (!scope.amount) {
              scope.unit = defaultUnit;
            }
          });
          if (defaultValue) {
            setDefault(amountAndUnit(defaultValue, scope.type));
          }
        });

        // Set unit options and based on type.
        switch (scope.type) {
        case 'cpu':
          scope.unit = 'm';
          scope.units = [{
            value: "m",
            label: "millicores"
          }, {
            value: "",
            label: "cores"
          }];
          break;
        case 'memory':
          scope.unit = 'Mi';
          scope.units = [{
            value: "M",
            label: "MB"
          }, {
            value: "G",
            label: "GB"
          }, {
            value: "Mi",
            label: "MiB"
          }, {
            value: "Gi",
            label: "GiB"
          }];
          break;
        }

        var validateLimitRange = function() {
          // Use usageValue filter to normalize units for comparison.
          var value = scope.amount && usageValue(scope.amount + scope.unit),
              min = scope.limitRangeMin && usageValue(scope.limitRangeMin),
              max = scope.limitRangeMax && usageValue(scope.limitRangeMax),
              minValid = true,
              maxValid = true;

          // Test against limit range min if defined.
          if (value && min) {
            minValid = value >= min;
          }

          // Test against limit range max if defined.
          if (value && max) {
            maxValid = value <= max;
          }

          scope.form.amount.$setValidity('limitRangeMin', minValid);
          scope.form.amount.$setValidity('limitRangeMax', maxValid);
        };

        var validateLimitAgainstRequest = function() {
          // Use usageValue filter to normalize units for comparison.
          var limit,
              request = scope.request && usageValue(scope.request),
              limitLargerThanRequest = true,
              limitWithinRatio = true;

          // Limit is either the value set by the user or the default value if limit is unset.
          if (scope.amount) {
            limit = usageValue(scope.amount + scope.unit);
          } else if (scope.defaultValue) {
            limit = usageValue(scope.defaultValue);
          }

          if (request && limit) {
            // Limit must be greater than or equal to request.
            limitLargerThanRequest = limit >= request;

            // Limit must be within the max limit/request ratio if defined.
            if (scope.maxLimitRequestRatio) {
              limitWithinRatio = (limit / request) <= scope.maxLimitRequestRatio;
            }
          }

          if (request && !limit && scope.maxLimitRequestRatio) {
            limitWithinRatio = false;
          }

          scope.form.amount.$setValidity('limitLargerThanRequest', limitLargerThanRequest);
          scope.form.amount.$setValidity('limitWithinRatio', limitWithinRatio);
        };

        // Update view from model.
        ngModel.$render = function() {
          var update = _.spread(function(amount, unit) {
            if (amount) {
              scope.amount = Number(amount);
              scope.unit = unit;
              // If the unit already set in the resource isn't in the list, add it.
              addUnitOption(unit);
            } else {
              scope.amount = null;
            }
          });
          update(amountAndUnit(ngModel.$viewValue, scope.type));
        };

        // Update model from view.
        scope.$watchGroup(['amount', 'unit'], function() {
          validateLimitRange();
          validateLimitAgainstRequest();
          if (!scope.amount) {
            ngModel.$setViewValue(undefined);
          } else {
            ngModel.$setViewValue(scope.amount + scope.unit);
          }
        });

        scope.$watchGroup(['limitRangeMin', 'limitRangeMax'], validateLimitRange);
        scope.$watch('request', validateLimitAgainstRequest);
      }
    };
  })
  .directive('editRequestLimit', function($filter, LimitRangesService) {
    return {
      restrict: 'E',
      scope: {
        resources: '=',
        // 'cpu' or 'memory'
        type: '@',
        limitRanges: '=',
        requestCalculated: '=',
        limitCalculated: '=?'
      },
      templateUrl: 'views/_edit-request-limit.html',
      link: function(scope) {
        scope.$watch('limitRanges', function() {
          scope.limits = LimitRangesService.getEffectiveLimitRange(scope.limitRanges, scope.type, 'Container');
        }, true);
      }
    };
  });
