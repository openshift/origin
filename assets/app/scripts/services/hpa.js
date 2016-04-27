'use strict';

angular.module("openshiftConsole")
  .factory("HPAService", function($filter, $q, LimitRangesService, MetricsService, Logger) {
    var getCPURequestToLimitPercent = function(project) {
      return LimitRangesService.getRequestToLimitPercent('cpu', project);
    };

    // Converts a percentage of request to a percentage of the limit based on
    // the request-to-limit ratio.
    // If CPU request percent is 120% and CPU request-to-limit percent is 50%, returns 60%
    var convertRequestPercentToLimit = function(requestPercent, project) {
      var cpuRequestToLimitPercent = getCPURequestToLimitPercent(project);
      if (!cpuRequestToLimitPercent) {
        Logger.warn('convertRequestPercentToLimit called, but no request/limit ratio defined.');
        return NaN;
      }

      if (!requestPercent) {
        return requestPercent;
      }

      var limitPercent = (cpuRequestToLimitPercent / 100) * requestPercent;
      return Math.round(limitPercent);
    };

    // Converts a percentage of limit to a percentage of the request based on
    // the request-to-limit ratio.
    // If CPU limit percent is 60% and CPU request-to-limit percent is 50%, returns 120%
    var convertLimitPercentToRequest = function(limitPercent, project) {
      var cpuRequestToLimitPercent = getCPURequestToLimitPercent(project);
      if (!cpuRequestToLimitPercent) {
        Logger.warn('convertLimitPercentToRequest called, but no request/limit ratio defined.');
        return NaN;
      }

      if (!limitPercent) {
        return limitPercent;
      }

      var requestPercent = limitPercent / (cpuRequestToLimitPercent / 100);
      return Math.round(requestPercent);
    };

    // Checks if all containers have a value set for the compute resource request or limit.
    //
    // computeResource  - 'cpu' or 'memory'
    // requestsOrLimits - 'requests' or 'limits'
    // containers       - array of containters from a deployment config or replication controller
    var hasRequestOrLimit = function(computeResource, requestsOrLimits, containers) {
      return _.every(containers, function(container) {
        return _.get(container, ['resources', requestsOrLimits, computeResource]);
      });
    };

    var hasRequestSet = function(computeResource, containers) {
      return hasRequestOrLimit(computeResource, 'requests', containers);
    };

    var hasLimitSet = function(computeResource, containers) {
      return hasRequestOrLimit(computeResource, 'limits', containers);
    };

    // Checks if there's a default for the compute resource request or limit in any LimitRange.
    //
    // computeResource  - 'cpu' or 'memory'
    // defaultType      - 'defaultRequest' or 'defaultLimit'
    // limitRanges     - collection of LimitRange objects (hash or array)
    var hasDefault = function(computeResource, defaultType, limitRanges) {
      var effectiveLimits = LimitRangesService.getEffectiveLimitRange(limitRanges, computeResource, 'Container');
      return !!effectiveLimits[defaultType];
    };

    var hasDefaultRequest = function(computeResource, limitRanges) {
      return hasDefault(computeResource, 'defaultRequest', limitRanges);
    };

    var hasDefaultLimit = function(computeResource, limitRanges) {
      return hasDefault(computeResource, 'defaultLimit', limitRanges);
    };

    // Is the corresponding limit value set to calculate a request?
    var canCalculateCPURequest = function(containers, limitRanges, project) {
      var limitComputeResource;
      if (LimitRangesService.isLimitCalculated('cpu', project)) {
        // If CPU limit is calculated from a memory limit, we need to check if memory limit is set.
        limitComputeResource = 'memory';
      } else {
        limitComputeResource = 'cpu';
      }

      // Check if the corresponding limit is set or defaulted.
      return hasLimitSet(limitComputeResource, containers) ||
             hasDefaultLimit(limitComputeResource, limitRanges);
    };

    // Checks if a CPU request is currently set or will be defaulted for any
    // container. A CPU request is required for autoscaling.
    //
    // containers       - array of containters from a deployment config or replication controller
    // limitRanges      - collection of LimitRange objects (hash or array)
    // project          - the project to determine if a request/limit ratio is set
    var hasCPURequest = function(containers, limitRanges, project) {
      if (hasRequestSet('cpu', containers)) {
        return true;
      }

      if (hasDefaultRequest('cpu', limitRanges)) {
        return true;
      }

      return LimitRangesService.isRequestCalculated('cpu', project) &&
             canCalculateCPURequest(containers, limitRanges, project);
    };

    // Filters the HPAs for those referencing kind/name.
    var filterHPA = function(hpaResources, kind, name) {
      return _.filter(hpaResources, function(hpa) {
        return hpa.spec.scaleRef.kind === kind && hpa.spec.scaleRef.name === name;
      });
    };

    // Filters the HPAs to those for a deployment config.
    // hpaResources  - map of HPA by name
    // dcName        - deployment config name
    var hpaForDC = function(hpaResources, dcName) {
      return filterHPA(hpaResources, "DeploymentConfig", dcName);
    };

    // Filters the HPAs to those for a replication controller.
    // hpaResources  - map of HPA by name
    // rcName        - replication controller name
    var hpaForRC = function(hpaResources, rcName) {
      return filterHPA(hpaResources, "ReplicationController", rcName);
    };

    var humanizeKind = $filter('humanizeKind');
    var isDeployment = $filter('isDeployment');

    // Gets HPA warnings.
    //
    // scaleTarget      - the object being scaled (DC or RC)
    // hpaResources     - collection of HPA resources (already filtered to this object)
    // limitRanges      - collection of LimitRange objects (hash or array)
    // project          - the project to determine if a request/limit ratio is set
    //
    // Returns an array of warnings, each an object with `message` and `reason` properties.
    var getHPAWarnings = function(scaleTarget, hpaResources, limitRanges, project) {
      if (!scaleTarget || _.isEmpty(hpaResources)) {
        return $q.when([]);
      }

      return MetricsService.isAvailable().then(function(metricsAvailable) {
        var warnings = [];
        if (!metricsAvailable) {
          warnings.push({
            message: 'Metrics might not be configured by your cluster administrator. ' +
                     'Metrics are required for autoscaling.',
            reason: 'MetricsNotAvailable'
          });
        }

        var containers = _.get(scaleTarget, 'spec.template.spec.containers', []);
        var kind, cpuRequestMessage;
        if (!hasCPURequest(containers, limitRanges, project)) {
          kind = humanizeKind(scaleTarget.kind);
          if (LimitRangesService.isRequestCalculated('cpu', project)) {
            cpuRequestMessage = 'This ' + kind + ' does not have any containers with a CPU limit set. ' +
                      'Autoscaling will not work without a CPU limit.';
            if (LimitRangesService.isLimitCalculated('cpu', project)) {
              cpuRequestMessage += ' The CPU limit will be automatically calculated from the container memory limit.';
            }
          } else {
            cpuRequestMessage = 'This ' + kind + ' does not have any containers with a CPU request set. ' +
                      'Autoscaling will not work without a CPU request.';
          }

          warnings.push({
            message: cpuRequestMessage,
            reason: 'NoCPURequest'
          });
        }

        if (_.size(hpaResources) > 1) {
          warnings.push({
            message: 'More than one autoscaler is scaling this resource. ' +
                     'This is not recommended because they might compete with each other. ' +
                     'Consider removing all but one autoscaler.',
            reason: 'MultipleHPA'
          });
        }

        // Warn about replication controllers that have both an HPA and DC, but
        // make sure an HPA targets the replication controller directly and
        // not its parent DC.
        var targetsRC = function() {
          return _.some(hpaResources, function(hpa) {
            return _.get(hpa, 'spec.scaleRef.kind') === 'ReplicationController';
          });
        };

        if (scaleTarget.kind === 'ReplicationController' &&
            isDeployment(scaleTarget) &&
            _.some(hpaResources, targetsRC)) {
          warnings.push({
            message: 'This deployment is scaled by both a deployment configuration and an autoscaler. ' +
                     'This is not recommended because they might compete with each other.',
            reason: 'DeploymentHasHPA'
          });
        }

        return warnings;
      });
    };

    return {
      convertRequestPercentToLimit: convertRequestPercentToLimit,
      convertLimitPercentToRequest: convertLimitPercentToRequest,
      hasCPURequest: hasCPURequest,
      hpaForDC: hpaForDC,
      hpaForRC: hpaForRC,
      getHPAWarnings: getHPAWarnings
    };
  });
