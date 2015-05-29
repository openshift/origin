'use strict';

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
    }
  })
  .filter('annotation', function() {
    // This maps a an annotation key to all known synonymous keys to insulate
    // the referring code from key renames across API versions.
    var annotationMap = {
      "deploymentConfig": ["openshift.io/deployment-config.name"],
      "deployment": ["openshift.io/deployment.name"],
      "pod": ["openshift.io/deployer-pod.name"],
      "deploymentStatus": ["openshift.io/deployment.phase"],
      "encodedDeploymentConfig": ["openshift.io/encoded-deployment-config"],
      "deploymentVersion": ["openshift.io/deployment-config.latest-version"]
    };
    return function(resource, key) {
      if (resource && resource.spec && resource.spec.tags && key.indexOf(".") !== -1){
        var tagAndKey = key.split(".");
        var tags = resource.spec.tags;
        for(var i=0; i < tags.length; ++i){
          var tag = tags[i];
          var tagName = tagAndKey[0];
          var tagKey = tagAndKey[1];
          if(tagName === tag.name && tag.annotations){
            return tag.annotations[tagKey];
          }
        }
      }
      if (resource && resource.metadata && resource.metadata.annotations) {
        // If the key's already in the annotation map, return it.
        if (resource.metadata.annotations[key] !== undefined) {
          return resource.metadata.annotations[key]
        }
        // Try and return a value for a mapped key.
        var mappings = annotationMap[key] || [];
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
  .filter('tags', function(annotationFilter) {
    return function(resource, annotationKey) {
      annotationKey = annotationKey || "tags";
      var tags = annotationFilter(resource, annotationKey);
      if (!tags) {
        return [];
      }
      return tags.split(/\s*,\s*/);
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
    return function(resource, kind, annotationKey) {
      annotationKey = annotationKey || "iconClass";
      var icon = annotationFilter(resource, annotationKey);
      if (!icon) {
        if (kind === "template") {
          return "fa fa-bolt";
        }
        if (kind === "image") {
          return "fa fa-cube";
        }
        else {
          return "";
        }
      }
      else {
        return icon;
      }
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
    }
  })
  .filter('imageStreamName', function() {
    return function(image) {
      if (!image) {
        return "";
      }
      // TODO move this parsing method into a utility method

      // remove @sha256:....
      var imageWithoutID = image.split("@")[0]

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
  .filter('buildForImage', function() {
    return function(image, builds) {
      // TODO concerned that this gets called anytime any data is changed on the scope, whether its relevant changes or not
      var envVars = image.dockerImageMetadata.Config.Env;
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
        type: "buildConfigHooks",
        id: buildConfig,
        namespace: project,
        secret: secret,
        hookType: type,
      });
    };
  })
  .filter('isWebRoute', function(){
    return function(route){
       //TODO: implement when we can tell if routes are http(s) or not web related which will drive links in view
       return true;
    };
  })
  .filter('routeWebURL', function(){
    return function(route){
        var scheme = (route.tls && route.tls.tlsTerminationType !== "") ? "https" : "http";
        var url = scheme + "://" + route.host;
        if (route.path) {
            url += route.path;
        }
        return url;
    };
  })
  .filter('routeLabel', function() {
    return function(route) {
      var label = route.host;
      if (route.path) {
        label += route.path;
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
  .filter('provider', function() {
    return function(resource) {
      return (resource && resource.annotations && resource.annotations.provider) ||
        (resource && resource.metadata && resource.metadata.namespace);
    };
  })
  .filter('imageObjectRef', function(){
    return function(objectRef){
      // TODO: needs to handle the current namespace
      var ns = objectRef.namespace || "";
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
        name = name.substring(name.lastIndexOf("/")+1);
        return name;
      }
      // TODO: we may want to indicate the actual type
      var ref = ns + objectRef.name;
      return ref;
    };
  })
  .filter('imageRepoReference', function(){
    return function(objectRef, kind, tag){
      var ns = objectRef.namespace || "";
      ns = ns === "" ? ns : ns + "/";

      if (kind === "ImageStreamTag" || kind === "ImageStreamImage") {
        return ns+objectRef.name;
      }
      var ref = ns + objectRef.name;
      // until v1beta2, the ImageStreamImage Kind isn't being set so we need to
      // manually check if the name looks like an ImageStreamImage
      if (objectRef.name.indexOf("@") === -1) {
        tag = tag || "latest";
        ref += " [" + tag + "]";
      }
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
  .filter('isTroubledPod', function() {
    // Scenario - Stuck Pod
    // Check if the pod has been pending for a long time.
    var isStuck = function(pod) {
      if (pod.status.phase !== 'Pending') {
        return false;
      }

      var fiveMinutesAgo = moment().subtract(5, 'm');
      var created = moment(pod.metadata.creationTimestamp);
      return created.isBefore(fiveMinutesAgo);
    };

    // Scenario - Looping Container
    // Check if the container is frequently restarting.
    var isLooping = function(containerStatus) {
      if (containerStatus.restartCount < 3 ||
          !containerStatus.state.running ||
          !containerStatus.state.running.startedAt) {
        return false;
      }

      // Only return true if the container has restarted recently.
      var fiveMinutesAgo = moment().subtract(5, 'm');
      var started = moment(containerStatus.state.running.startedAt);
      return started.isAfter(fiveMinutesAgo);
    };

    // Scenario - Failed Container
    // Check if the terminated container exited with a non-zero exit code
    var isFailed = function(containerStatus) {
      return containerStatus.state.termination && containerStatus.state.termination.exitCode !== 0;
    };

    // Scenario - Unprepared Container
    // Check if the container still isn't ready after a length of time.
    var isUnprepared = function(containerStatus) {
      if (!containerStatus.state.running ||
          containerStatus.ready !== false ||
          !containerStatus.state.running.startedAt) {
        return false;
      }

      var fiveMinutesAgo = moment().subtract(5, 'm');
      var started = moment(containerStatus.state.running.startedAt);
      return started.isBefore(fiveMinutesAgo);
    };

    return function(pod) {
      if (isStuck(pod)) {
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
          if (isFailed(containerStatus)) {
            return true;
          }
          if (isLooping(containerStatus)) {
            return true;
          }
          if (isUnprepared(containerStatus)) {
            return true;
          }
        }
      }

      return false;
    };
  })
  .filter('projectOverviewURL', function(Navigate) {
    return function(projectName) {
      return Navigate.projectOverviewURL(projectName);
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
  });
