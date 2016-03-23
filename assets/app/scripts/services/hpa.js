'use strict';

angular.module("openshiftConsole")
  .factory("HPAService", function(LimitRangesService) {
    var getCPURequestToLimitPercent = function(project) {
      return LimitRangesService.getRequestToLimitPercent('cpu', project);
    };

    // Converts a percentage of request to a percentage of the limit based on
    // the request-to-limit ratio.
    // If CPU request percent is 120% and CPU request-to-limit percent is 50%, returns 60%
    var convertRequestPercentToLimit = function(requestPercent, project) {
      if (!requestPercent) {
        return requestPercent;
      }

      var cpuRequestToLimitPercent = getCPURequestToLimitPercent(project);
      var limitPercent = (cpuRequestToLimitPercent / 100) * requestPercent;
      return Math.round(limitPercent);
    };

    // Converts a percentage of limit to a percentage of the request based on
    // the request-to-limit ratio.
    // If CPU limit percent is 60% and CPU request-to-limit percent is 50%, returns 120%
    var convertLimitPercentToRequest = function(limitPercent, project) {
      if (!limitPercent) {
        return limitPercent;
      }

      var cpuRequestToLimitPercent = getCPURequestToLimitPercent(project);
      var requestPercent = limitPercent / (cpuRequestToLimitPercent / 100);
      return Math.round(requestPercent);
    };

    // Checks if any container has a value set for the compute resource request or limit.
    //
    // computeResource  - 'cpu' or 'memory'
    // requestsOrLimits - 'requests' or 'limits'
    // containers       - array of containters from a deployment config or replication controller
    var hasRequestOrLimit = function(computeResource, requestsOrLimits, containers) {
      return _.some(containers, function(container) {
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
    // limitsRanges     - collection of LimitRange objects (hash or array)
    var hasDefault = function(computeResource, defaultType, limitRanges) {
      // Check each limit range.
      return _.some(limitRanges, function(limitRange) {
        // Check each limit in the limit range.
        var limits = _.get(limitRange, 'spec.limits', []);
        return _.some(limits, function(limit) {
          return limit.type === 'Container' &&
                 _.get(limit, [defaultType, computeResource]);
        });
      });
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
    // limitsRanges     - collection of LimitRange objects (hash or array)
    // project          - the project to determine if a request/limit ratio is set
    var hasCPURequest = function(containers, limitRanges, project) {
      if (LimitRangesService.isRequestCalculated('cpu', project)) {
        // If request is calculated, check that the corresponding limit value is set or defaulted.
        return canCalculateCPURequest(containers, limitRanges, project);
      }

      return hasRequestSet('cpu', containers) ||
             hasDefaultRequest('cpu', limitRanges);
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

    return {
      convertRequestPercentToLimit: convertRequestPercentToLimit,
      convertLimitPercentToRequest: convertLimitPercentToRequest,
      hasCPURequest: hasCPURequest,
      hpaForDC: hpaForDC,
      hpaForRC: hpaForRC
    };
  });
