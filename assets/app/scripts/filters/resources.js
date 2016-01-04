'use strict';
/* jshint unused: false */

angular.module('openshiftConsole')
  // this filter is intended for use with the "track by" in an ng-repeat
  // when uid is not defined it falls back to object identity for uniqueness
  .filter('uid', function() {
    return function(resource) {
      if (resource && resource.metadata && resource.metadata.uid) {
        return resource.metadata.uid;
      }
      else {
        return resource;
      }
    };
  })
  .filter('annotationName', function() {
    return function(annotationKey) {
      // This maps an annotation key to all known synonymous keys to insulate
      // the referring code from key renames across API versions.
      var annotationMap = {
        "deploymentConfig":         ["openshift.io/deployment-config.name"],
        "deployment":               ["openshift.io/deployment.name"],
        "pod":                      ["openshift.io/deployer-pod.name"],
        "deployerPodFor":           ["openshift.io/deployer-pod-for.name"],
        "deploymentStatus":         ["openshift.io/deployment.phase"],
        "deploymentStatusReason":   ["openshift.io/deployment.status-reason"],
        "deploymentCancelled":      ["openshift.io/deployment.cancelled"],
        "encodedDeploymentConfig":  ["openshift.io/encoded-deployment-config"],
        "deploymentVersion":        ["openshift.io/deployment-config.latest-version"],
        "displayName":              ["openshift.io/display-name"],
        "description":              ["openshift.io/description"],
        "buildNumber":              ["openshift.io/build.number"],
        "buildPod":                 ["openshift.io/build.pod-name"]
      };
      return annotationMap[annotationKey] || null;
    };
  })
  .filter('annotation', function(annotationNameFilter) {
    return function(resource, key) {
      if (resource && resource.metadata && resource.metadata.annotations) {
        // If the key's already in the annotation map, return it.
        if (resource.metadata.annotations[key] !== undefined) {
          return resource.metadata.annotations[key];
        }
        // Try and return a value for a mapped key.
        var mappings = annotationNameFilter(key) || [];
        for (var i=0; i < mappings.length; i++) {
          var mappedKey = mappings[i];
          if (resource.metadata.annotations[mappedKey] !== undefined) {
            return resource.metadata.annotations[mappedKey];
          }
        }
        // Couldn't find anything.
        return null;
      }
      return null;
    };
  })
  .filter('imageStreamTagAnnotation', function() {
    // Look up annotations on ImageStream.spec.tags[tag].annotations
    return function(resource, key, /* optional */ tagName) {
      tagName = tagName || 'latest';
      if (resource && resource.spec && resource.spec.tags){
        var tags = resource.spec.tags;
        for(var i=0; i < tags.length; ++i){
          var tag = tags[i];
          if(tagName === tag.name && tag.annotations){
            return tag.annotations[key];
          }
        }
      }

      return null;
    };
  })
  .filter('description', function(annotationFilter) {
    return function(resource) {
      return annotationFilter(resource, "description");
    };
  })
  .filter('displayName', function(annotationFilter) {
    // annotationOnly - if true, don't fall back to using metadata.name when
    //                  there's no displayName annotation
    return function(resource, annotationOnly) {
      var displayName = annotationFilter(resource, "displayName");
      if (displayName || annotationOnly) {
        return displayName;
      }

      if (resource && resource.metadata) {
        return resource.metadata.name;
      }

      return null;
    };
  })
  .filter('uniqueDisplayName', function(displayNameFilter){
    function countNames(projects){
      var nameCount = {};
      angular.forEach(projects, function(project, key){
        var displayName = displayNameFilter(project);
        nameCount[displayName] = (nameCount[displayName] || 0) + 1;
      });
      return nameCount;
    }
    return function (resource, projects){
      var displayName = displayNameFilter(resource);
      var name = resource.metadata.name;
      if (displayName !== name && countNames(projects)[displayName] > 1 ){
        return displayName + ' (' + name + ')';
      }
      return displayName;
    };
  })
  .filter('tags', function(annotationFilter) {
    return function(resource, /* optional */ annotationKey) {
      annotationKey = annotationKey || "tags";
      var tags = annotationFilter(resource, annotationKey);
      if (!tags) {
        return [];
      }
      return tags.split(/\s*,\s*/);
    };
  })
  .filter('imageStreamTagTags', function(imageStreamTagAnnotationFilter) {
    // Return ImageStream.spec.tag[tag].annotation.tags as an array
    return function(resource, /* optional */ tagName) {
      var imageTags = imageStreamTagAnnotationFilter(resource, 'tags', tagName);
      if (!imageTags) {
        return [];
      }

      return imageTags.split(/\s*,\s*/);
    };
  })
  .filter('imageStreamLastUpdated', function() {
    return function(imageStream) {
      var lastUpdated = imageStream.metadata.creationTimestamp;
      var lastUpdatedMoment = moment(lastUpdated);
      angular.forEach(imageStream.status.tags, function(tag) {
        if (tag.items && tag.items.length > 0) {
          var tagUpdatedMoment = moment(tag.items[0].created);
          if (tagUpdatedMoment.isAfter(lastUpdatedMoment)) {
            lastUpdatedMoment = tagUpdatedMoment;
            lastUpdated = tag.items[0].created;
          }
        }
      });
      return lastUpdated;
    };
  })
  .filter('label', function() {
    return function(resource, key) {
      if (resource && resource.metadata && resource.metadata.labels) {
        return resource.metadata.labels[key];
      }
      return null;
    };
  })
  .filter('icon', function(annotationFilter) {
    return function(resource) {
      var icon = annotationFilter(resource, "icon");
      if (!icon) {
        //FIXME: Return default icon for resource.kind
        return "";
      } else {
        return icon;
      }
    };
  })
  .filter('iconClass', function(annotationFilter) {
    return function(resource, kind) {
      var icon = annotationFilter(resource, "iconClass");
      if (!icon) {
        if (kind === "template") {
          return "fa fa-clone";
        }

        return "";
      }

      return icon;
    };
  })
  .filter('imageStreamTagIconClass', function(imageStreamTagAnnotationFilter) {
    return function(resource, /* optional */ tagName) {
      var icon = imageStreamTagAnnotationFilter(resource, "iconClass", tagName);
      return (icon) ? icon : "fa fa-cube";
    };
  })
  .filter('imageName', function() {
    // takes an image name and strips off the leading <algorithm>: from it,
    // if it exists.
    return function(image) {
      if (!image) {
        return "";
      }

      if (!image.contains(":")) {
        return image;
      }

      return image.split(":")[1];
    };
  })
  .filter('imageStreamName', function() {
    return function(image) {
      if (!image) {
        return "";
      }
      // TODO move this parsing method into a utility method

      // remove @sha256:....
      var imageWithoutID = image.split("@")[0];

      var slashSplit = imageWithoutID.split("/");
      var semiColonSplit;
      if (slashSplit.length === 3) {
        semiColonSplit = slashSplit[2].split(":");
        return slashSplit[1] + '/' + semiColonSplit[0];
      }
      else if (slashSplit.length === 2) {
        // TODO umm tough... this could be registry/imageName or imageRepo/imageName
        // have to check if the first bit matches a registry pattern, will handle this later...
        return imageWithoutID;
      }
      else if (slashSplit.length === 1) {
        semiColonSplit = imageWithoutID.split(":");
        return semiColonSplit[0];
      }
    };
  })
  .filter('imageEnv', function() {
    return function(image, envKey) {
      var envVars = image.dockerImageMetadata.Config.Env;
      for (var i = 0; i < envVars.length; i++) {
        var keyValue = envVars[i].split("=");
        if (keyValue[0] === envKey) {
          return keyValue[1];
        }
      }
      return null;
    };
  })
  .filter('envVarsPair', function() {
    return function(configEnvVars) {
      var pairs = {};
      angular.forEach(configEnvVars, function(env) {
        pairs[env.name] = env.value;
      });
      return pairs;
    };
  })
  .filter('destinationSourcePair', function() {
    return function(destination) {
      var pairs = {};
      angular.forEach(destination, function(env) {
        pairs[env.sourcePath] = env.destinationDir;
      });
      return pairs;
    };
  })
  .filter('buildForImage', function() {
    return function(image, builds) {
      // TODO concerned that this gets called anytime any data is changed on the scope,
      // whether its relevant changes or not
      var envVars = _.get(image, 'dockerImageMetadata.Config.Env', []);
      for (var i = 0; i < envVars.length; i++) {
        var keyValue = envVars[i].split("=");
        if (keyValue[0] === "OPENSHIFT_BUILD_NAME") {
          return builds[keyValue[1]];
        }
      }
      return null;
    };
  })
  .filter('webhookURL', function(DataService) {
    return function(buildConfig, type, secret, project) {
      return DataService.url({
      	// arbitrarily many subresources can be included
      	// url encoding of the segments is handled by the url() function
      	// subresource segments cannot contain '/'
        resource: "buildconfigs/webhooks/" + secret + "/" + type.toLowerCase(),
        name: buildConfig,
        namespace: project
      });
    };
  })
  .filter('isWebRoute', function(){
    return function(route){
       //TODO: implement when we can tell if routes are http(s) or not web related which will drive
       // links in view
       return true;
    };
  })
  .filter('routeWebURL', function(routeHostFilter){
    return function(route, host){
        var scheme = (route.spec.tls && route.spec.tls.tlsTerminationType !== "") ? "https" : "http";
        var url = scheme + "://" + (host || routeHostFilter(route));
        if (route.spec.path) {
            url += route.spec.path;
        }
        return url;
    };
  })
  .filter('routeLabel', function(routeHostFilter) {
    return function(route, host) {
      var label = (host || routeHostFilter(route));
      if (route.spec.path) {
        label += route.spec.path;
      }
      return label;
    };
  })
  .filter('parameterPlaceholder', function() {
    return function(parameter) {
      if (parameter.generate) {
        return "(generated if empty)";
      } else {
        return "";
      }
    };
  })
 .filter('parameterValue', function() {
    return function(parameter) {
      if (!parameter.value && parameter.generate) {
        return "(generated)";
      } else {
        return parameter.value;
      }
    };
  })
  .filter('imageObjectRef', function(){
    return function(objectRef, /* optional */ nsIfUnspecified, shortOutput){
      if (!objectRef) {
        return "";
      }

      var ns = objectRef.namespace || nsIfUnspecified || "";
      if (ns.length > 0) {
        ns = ns + "/";
      }
      var kind = objectRef.kind;
      if (kind === "ImageStreamTag" || kind === "ImageStreamImage") {
        return ns+objectRef.name;
      }
      if (kind === "DockerImage") {
        // TODO: replace with real DockerImageReference parse function
        var name = objectRef.name;
        // TODO: should we be removing the n
        if (shortOutput) {
          name = name.substring(name.lastIndexOf("/")+1);
        }
        return name;
      }
      // TODO: we may want to indicate the actual type
      var ref = ns + objectRef.name;
      return ref;
    };
  })
  .filter('orderByDisplayName', function(displayNameFilter, toArrayFilter) {
    return function(items) {
      var itemsArray = toArrayFilter(items);
      itemsArray.sort(function(left, right) {
        var leftName = displayNameFilter(left) || '';
        var rightName = displayNameFilter(right) || '';
        return leftName.localeCompare(rightName);
      });

      return itemsArray;
    };
  })
  .filter('isPodStuck', function() {
    return function(pod) {
      if (pod.status.phase !== 'Pending') {
        return false;
      }

      // If this logic ever changes, update the message in podWarnings
      var fiveMinutesAgo = moment().subtract(5, 'm');
      var created = moment(pod.metadata.creationTimestamp);
      return created.isBefore(fiveMinutesAgo);
    };
  })
  .filter('isContainerLooping', function() {
    return function(containerStatus) {
      return containerStatus.state.waiting && containerStatus.state.waiting.reason === 'CrashLoopBackOff';
    };
  })
  .filter('isContainerFailed', function() {
    return function(containerStatus) {
      // If this logic ever changes, update the message in podWarnings
      return containerStatus.state.terminated && containerStatus.state.terminated.exitCode !== 0;
    };
  })
  .filter('isContainerUnprepared', function() {
    return function(containerStatus) {
      if (!containerStatus.state.running ||
          containerStatus.ready !== false ||
          !containerStatus.state.running.startedAt) {
        return false;
      }

      // If this logic ever changes, update the message in podWarnings
      var fiveMinutesAgo = moment().subtract(5, 'm');
      var started = moment(containerStatus.state.running.startedAt);
      return started.isBefore(fiveMinutesAgo);
    };
  })
  .filter('isTroubledPod', function(isPodStuckFilter, isContainerLoopingFilter, isContainerFailedFilter, isContainerUnpreparedFilter) {
    return function(pod) {
      if (pod.status.phase === 'Unknown') {
        // We always show Unknown pods in a warning state
        return true;
      }

      if (isPodStuckFilter(pod)) {
        return true;
      }

      if (pod.status.phase === 'Running' && pod.status.containerStatuses) {
        // Check container statuses and short circuit when we find any problem.
        var i;
        for (i = 0; i < pod.status.containerStatuses.length; ++i) {
          var containerStatus = pod.status.containerStatuses[i];
          if (!containerStatus.state) {
            continue;
          }
          if (isContainerFailedFilter(containerStatus)) {
            return true;
          }
          if (isContainerLoopingFilter(containerStatus)) {
            return true;
          }
          if (isContainerUnpreparedFilter(containerStatus)) {
            return true;
          }
        }
      }

      return false;
    };
  })
  .filter('podWarnings', function(isPodStuckFilter, isContainerLoopingFilter, isContainerFailedFilter, isContainerUnpreparedFilter) {
    return function(pod) {
      var warnings = [];

      if (pod.status.phase === 'Unknown') {
        // We always show Unknown pods in a warning state
        warnings.push({reason: 'Unknown', message: 'The state of this pod could not be obtained. This is typically due to an error communicating with the host of the pod.'});
      }

      if (isPodStuckFilter(pod)) {
        warnings.push({reason: "Stuck", message: "This pod has been stuck in the pending state for more than five minutes."});
      }

      if (pod.status.phase === 'Running' && pod.status.containerStatuses) {
        // Check container statuses and short circuit when we find any problem.
        var i;
        for (i = 0; i < pod.status.containerStatuses.length; ++i) {
          var containerStatus = pod.status.containerStatuses[i];
          if (!containerStatus.state) {
            continue;
          }
          if (isContainerFailedFilter(containerStatus)) {
            warnings.push({reason: "Failed", message: "The container " + containerStatus.name + " failed with a non-zero exit code " + containerStatus.state.terminated.exitCode + "."});
          }
          if (isContainerLoopingFilter(containerStatus)) {
            warnings.push({reason: "Looping", message: "The container " + containerStatus.name + " is crashing frequently. It must wait before it will be restarted again."});
          }
          if (isContainerUnpreparedFilter(containerStatus)) {
            warnings.push({reason: "Unprepared", message: "The container " + containerStatus.name + " has been running for more than five minutes and has not passed its readiness check."});
          }
        }
      }

      return warnings.length > 0 ? warnings : null;
    };
  })
  .filter('troubledPods', function(isTroubledPodFilter) {
    return function(pods) {
      var troublePods = [];
      angular.forEach(pods, function(pod){
        if (isTroubledPodFilter(pod)) {
          troublePods.push(pod);
        }
      });
      return troublePods;
    };
  })
  .filter('notTroubledPods', function(isTroubledPodFilter) {
    return function(pods) {
      var notTroublePods = [];
      angular.forEach(pods, function(pod){
        if (!isTroubledPodFilter(pod)) {
          notTroublePods.push(pod);
        }
      });
      return notTroublePods;
    };
  })
  .filter('projectOverviewURL', function(Navigate) {
    return function(project) {
      return angular.isString(project) ?
              Navigate.projectOverviewURL(project) :
              angular.isObject(project) ?
                Navigate.projectOverviewURL(project.metadata && project.metadata.name):
                Navigate.projectOverviewURL(''); // fail case will invoke default route (projects list)
    };
  })
  .filter('createFromSourceURL', function() {
    return function(projectName, sourceURL) {
      var createURI = URI.expand("project/{project}/catalog/images{?q*}", {
        project: projectName,
        q: {
          builderfor: sourceURL
        }
      });
      return createURI.toString();
    };
  })
  .filter('createFromImageURL', function() {
    return function(imageStream, imageTag, projectName) {
      var createURI = URI.expand("project/{project}/create/fromimage{?q*}", {
        project: projectName,
        q: {
          imageName: imageStream.metadata.name,
          imageTag: imageTag,
          namespace: imageStream.metadata.namespace
        }
      });
      return createURI.toString();
    };
  })
  .filter('createFromTemplateURL', function() {
    return function(template, projectName) {
      var createURI = URI.expand("project/{project}/create/fromtemplate{?q*}", {
        project: projectName,
        q: {
          name: template.metadata.name,
          namespace: template.metadata.namespace
        }
      });
      return createURI.toString();
    };
  })
  .filter('failureObjectName', function() {
    return function(failure) {
      if (!failure.data || !failure.data.details) {
        return null;
      }

      var details = failure.data.details;
      if (details.kind) {
        return (details.id) ? details.kind + " " + details.id : details.kind;
      }

      return details.id;
    };
  })
  .filter('isIncompleteBuild', function(ageLessThanFilter) {
    return function(build) {
      if (!build || !build.status || !build.status.phase) {
        return false;
      }

      switch (build.status.phase) {
        case 'New':
        case 'Pending':
        case 'Running':
          return true;
        default:
          return false;
      }
    };
  })
  .filter('isRecentBuild', function(ageLessThanFilter, isIncompleteBuildFilter) {
    return function(build) {
      if (!build || !build.status || !build.status.phase || !build.metadata) {
        return false;
      }

      if (isIncompleteBuildFilter(build)) {
        return true;
      }

      var timestamp = build.status.completionTimestamp || build.metadata.creationTimestamp;
      switch (build.status.phase) {
        case 'Complete':
        case 'Cancelled':
          return ageLessThanFilter(timestamp, 1, 'minutes');
        case 'Failed':
        case 'Error':
          /* falls through */
        default:
          return ageLessThanFilter(timestamp, 5, 'minutes');
      }
    };
  })
  .filter('deploymentCauses', function(annotationFilter) {
    return function(deployment) {
      if (!deployment) {
        return [];
      }

      var configJson = annotationFilter(deployment, 'encodedDeploymentConfig');
      if (!configJson) {
        return [];
      }

      try {
        var depConfig = $.parseJSON(configJson);
        if (!depConfig) {
          return [];
        }

        switch (depConfig.apiVersion) {
          case "v1beta1":
            return depConfig.details.causes;
          case "v1beta3":
          case "v1":
            return  depConfig.status.details ? depConfig.status.details.causes : [];
          default:
          // Unrecognized API version. Log an error.
          Logger.error('Unknown API version "' + depConfig.apiVersion +
                       '" in encoded deployment config for deployment ' +
                       deployment.metadata.name);

          // Try to fall back to the last thing we know.
          if (depConfig.status && depConfig.status.details && depConfig.status.details.causes) {
            return depConfig.status.details.causes;
          }

          return [];
        }
      }
      catch (e) {
        Logger.error("Failed to parse encoded deployment config", e);
        return [];
      }
    };
  })
  .filter('desiredReplicas', function() {
    return function(rc) {
      if (!rc || !rc.spec) {
        return 0;
      }

      // If unset, the default is 1.
      if (rc.spec.replicas === undefined) {
        return 1;
      }

      return rc.spec.replicas;
    };
  })
  .filter('serviceImplicitDNSName', function() {
    return function(service) {
      if (!service || !service.metadata || !service.metadata.name || !service.metadata.namespace) {
        return '';
      }

      // cluster.local suffix is customizable, so leave it off. <name>.<namespace>.svc resolves.
      return service.metadata.name + '.' + service.metadata.namespace + '.svc';
    };
  })
  .filter('podsForPhase', function() {
    return function(pods, phase) {
      var podsForPhase = [];
      angular.forEach(pods, function(pod){
        if (pod.status.phase === phase) {
          podsForPhase.push(pod);
        }
      });
      return podsForPhase;
    };
  })
  .filter('numContainersReady', function() {
    return function(pod) {
      var numReady = 0;
      angular.forEach(pod.status.containerStatuses, function(status) {
        if (status.ready) {
          numReady++;
        }
      });
      return numReady;
    };
  })
  .filter('numContainerRestarts', function() {
    return function(pod) {
      var numRestarts = 0;
      angular.forEach(pod.status.containerStatuses, function(status) {
        numRestarts += status.restartCount;
      });
      return numRestarts;
    };
  })
  .filter('newestResource', function() {
    return function(resources) {
      var newest = null;
      angular.forEach(resources, function(resource) {
        if (!newest) {
          if (resource.metadata.creationTimestamp) {
            newest = resource;
          }
          else {
            return;
          }
        }
        else if (moment(newest.metadata.creationTimestamp).isBefore(resource.metadata.creationTimestamp)) {
          newest = resource;
        }
      });
      return newest;
    };
  })
  .filter('deploymentIsLatest', function(annotationFilter) {
    return function(deployment, deploymentConfig) {
      if (!deploymentConfig || !deployment) {
        return false;
      }
      var deploymentVersion = parseInt(annotationFilter(deployment, 'deploymentVersion'));
      var deploymentConfigVersion = deploymentConfig.status.latestVersion;
      return deploymentVersion === deploymentConfigVersion;
    };
  })
  .filter('deploymentStatus', function(annotationFilter, isDeploymentFilter) {
    return function(deployment) {
      // We should show Cancelled as an actual status instead of showing Failed
      if (annotationFilter(deployment, 'deploymentCancelled')) {
        return "Cancelled";
      }
      var status = annotationFilter(deployment, 'deploymentStatus');
      // If it is just an RC (non-deployment) or it is a deployment with more than 0 replicas
      if (!isDeploymentFilter(deployment) || status === "Complete" && deployment.spec.replicas > 0) {
        return "Deployed";
      }
      return status;
    };
  })
  .filter('deploymentIsInProgress', function(deploymentStatusFilter) {
    return function(deployment) {
      return ['New', 'Pending', 'Running'].indexOf(deploymentStatusFilter(deployment)) > -1;
    };
  })
  .filter('isDeployment', function(annotationFilter) {
    return function(deployment) {
      return (annotationFilter(deployment, 'deploymentConfig')) ? true : false;
    };
  })
  .filter('isRecentDeployment', function(deploymentIsLatestFilter, deploymentIsInProgressFilter) {
    return function(deployment, deploymentConfig) {
      if (deploymentIsLatestFilter(deployment, deploymentConfig)) {
        return true;
      }

      if (deploymentIsInProgressFilter(deployment)) {
        return true;
      }

      return false;
    };
  })
  .filter('buildStrategy', function() {
    return function(build) {
      if (!build || !build.spec || !build.spec.strategy) {
        return null;
      }
      switch (build.spec.strategy.type) {
        case 'Source':
          return build.spec.strategy.sourceStrategy;
        case 'Docker':
          return build.spec.strategy.dockerStrategy;
        case 'Custom':
          return build.spec.strategy.customStrategy;
        default:
          return null;
      }
    };
  })
  .filter('humanizeKind', function (startCaseFilter) {
    return startCaseFilter;
  })
  .filter('humanizeQuotaResource', function() {
    return function(resourceType) {
      if (!resourceType) {
        return resourceType;
      }

      var nameFormatMap = {
        'configmaps': 'Config Maps',
        'cpu': 'CPU (Request)',
        'limits.cpu': 'CPU (Limit)',
        'limits.memory': 'Memory (Limit)',
        'memory': 'Memory (Request)',
        'openshift.io/imagesize': 'Image Size',
        'openshift.io/imagestreamsize': 'Image Stream Size',
        'openshift.io/projectimagessize': 'Project Image Size',
        'persistentvolumeclaims': 'Persistent Volume Claims',
        'pods': 'Pods',
        'replicationcontrollers': 'Replication Controllers',
        'requests.cpu': 'CPU (Request)',
        'requests.memory': 'Memory (Request)',
        'resourcequotas': 'Resource Quotas',
        'secrets': 'Secrets',
        'services': 'Services'
      };
      return nameFormatMap[resourceType] || resourceType;
    };
  })
  .filter('routeTargetPortMapping', function(RoutesService) {
    var portDisplayValue = function(servicePort, containerPort, protocol) {
      servicePort = servicePort || "<unknown>";
      containerPort = containerPort || "<unknown>";

      // \u2192 is a right arrow (see examples below)
      var mapping = "Service Port " + servicePort  + " \u2192 Container Port " + containerPort;
      if (protocol) {
        mapping += " (" + protocol + ")";
      }

      return mapping;
    };

    // Returns a display value for a route target port that includes the
    // service port, e.g.
    //   Service Port 8080 -> Container Port 8081
    // If no target port for the route or service is undefined, returns an
    // empty string.
    // If the corresponding port is not found, returns
    //   Service Port <unknown> -> Container Port 8081
    // or
    //   Service Port web -> Container Port <unknown>
    return function(route, service) {
      if (!route.spec.port || !route.spec.port.targetPort || !service) {
        return '';
      }

      var targetPort = route.spec.port.targetPort;
      // Find the corresponding service port.
      var servicePort = RoutesService.getServicePortForRoute(targetPort, service);
      if (!servicePort) {
        // Named ports refer to the service port name.
        if (angular.isString(targetPort)) {
          return portDisplayValue(targetPort, null);
        }

        // Numbers refer to the container port.
        return portDisplayValue(null, targetPort);
      }

      return portDisplayValue(servicePort.port, servicePort.targetPort, servicePort.protocol);
    };
  })
  .filter('podStatus', function() {
    // Return results that match kubernetes/pkg/kubectl/resource_printer.go
    return function(pod) {
      if (!pod || (!pod.metadata.deletionTimestamp && !pod.status)) {
        return '';
      }

      if (pod.metadata.deletionTimestamp) {
        return 'Terminating';
      }

      var reason = pod.status.reason || pod.status.phase;

      // Print detailed container reasons if available. Only the last will be
      // displayed if multiple containers have this detail.
      angular.forEach(pod.status.containerStatuses, function(containerStatus) {
        var containerReason = _.get(containerStatus, 'state.waiting.reason') || _.get(containerStatus, 'state.terminated.reason'),
            signal,
            exitCode;

        if (containerReason) {
          reason = containerReason;
          return;
        }

        signal = _.get(containerStatus, 'state.terminated.signal');
        if (signal) {
          reason = "Signal: " + signal;
          return;
        }

        exitCode = _.get(containerStatus, 'state.terminated.exitCode');
        if (exitCode) {
          reason = "Exit Code: " + exitCode;
        }
      });

      return reason;
    };
  })
  .filter('routeIngressCondition', function() {
    return function(ingress, type) {
      if (!ingress) {
        return null;
      }
      return _.find(ingress.conditions, {type: type});
    };
  })
  .filter('routeHost', function() {
    return function (route) {
      if (!route.status.ingress) {
        return route.spec.host;
      }
      var oldestAdmittedIngress = null;
      angular.forEach(route.status.ingress, function(ingress) {
        if (_.some(ingress.conditions, { type: "Admitted", status: "True" }) &&
            (!oldestAdmittedIngress || oldestAdmittedIngress.lastTransitionTime > ingress.lastTransitionTime)) {
          oldestAdmittedIngress = ingress;
        }
      });

      return oldestAdmittedIngress ? oldestAdmittedIngress.host : route.spec.host;
    };
  })
  .filter('isRequestCalculated', function(LimitRangesService) {
    return function(computeResource, project) {
      return LimitRangesService.isRequestCalculated(computeResource, project);
    };
  })
  .filter('isLimitCalculated', function(LimitRangesService) {
    return function(computeResource, project) {
      return LimitRangesService.isLimitCalculated(computeResource, project);
    };
  })
  .filter('hpaCPUPercent', function(HPAService, LimitRangesService) {
    // Convert between CPU request percentage and CPU limit percentage if
    // necessary to display an HPA value. Values are shown as percentages
    // of CPU limit if a request/limit override is set.
    return function(targetCPU, project) {
      if (!targetCPU) {
        return targetCPU;
      }

      if (!LimitRangesService.isRequestCalculated('cpu', project)) {
        return targetCPU;
      }

      return HPAService.convertRequestPercentToLimit(targetCPU, project);
    };
  });
