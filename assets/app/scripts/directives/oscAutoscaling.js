"use strict";

angular.module("openshiftConsole")
  /**
   * Widget for entering autoscaling information
   */
  .directive("oscAutoscaling", function(HPAService, LimitRangesService) {
    return {
      restrict: 'E',
      scope: {
        autoscaling: "=model",
        // Needed to determine if limit/request overrides are set. Required.
        project: "=",
        showNameInput: "=?",
        nameReadOnly: "=?"
      },
      templateUrl: 'views/directives/osc-autoscaling.html',
      link: function(scope) {
        // Wait for project to be set.
        scope.$watch('project', function() {
          if (!scope.project) {
            return;
          }

          // The HPA targetCPU percentage always is against CPU request. If
          // request/limit overrides are in place, we ask for the percentage
          // against limit, however.
          scope.isRequestCalculated = LimitRangesService.isRequestCalculated('cpu', scope.project);

          // Set a default value in the model to include if the HPA if the field is empty.
          var defaultTargetCPU = window.OPENSHIFT_CONSTANTS.DEFAULT_HPA_CPU_TARGET_PERCENT;
          if (scope.isRequestCalculated) {
            // Convert to percent of request to set in the HPA resource.
            defaultTargetCPU = HPAService.convertLimitPercentToRequest(defaultTargetCPU, scope.project);
          }
          _.set(scope, 'autoscaling.defaultTargetCPU', defaultTargetCPU);

          // Default percent for display in the view as a placeholder and in help
          // text. Don't convert this value since the field will prompt for
          // limit instead when a request/limit override is in place and we want
          // to show default percent of limit.
          scope.defaultTargetCPUDisplayValue = window.OPENSHIFT_CONSTANTS.DEFAULT_HPA_CPU_TARGET_PERCENT;

          // Keep the input value and model value separate in the scope so we can
          // convert between them on changes.
          var inputValueChanged = false;
          var updateTargetCPUInput = function(targetCPU) {
            if (inputValueChanged) {
              // Don't update the input in response to the user typing. Only
              // update the input value when the target CPU changes outside the
              // directive.
              inputValueChanged = false;
              return;
            }

            if (targetCPU && scope.isRequestCalculated) {
              // Convert this to a limit value for the target CPU input.
              targetCPU = HPAService.convertRequestPercentToLimit(targetCPU, scope.project);
            }
            _.set(scope, 'targetCPUInput.percent', targetCPU);
          };
          scope.$watch('autoscaling.targetCPU', updateTargetCPUInput);

          // Update the model with the target CPU request percentage when the input value changes.
          var updateTargetCPUModel = function(targetCPU) {
            if (targetCPU && scope.isRequestCalculated) {
              targetCPU = HPAService.convertLimitPercentToRequest(targetCPU, scope.project);
            }

            inputValueChanged = true;
            _.set(scope, 'autoscaling.targetCPU', targetCPU);
          };

          // Watch changes to the target CPU input to set values back in the model.
          scope.$watch('targetCPUInput.percent', function(newValue, oldValue) {
            if (newValue === oldValue) {
              return;
            }

            updateTargetCPUModel(newValue);
          });
        });
      }
    };
  });
