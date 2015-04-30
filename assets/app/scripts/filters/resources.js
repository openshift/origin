'use strict';

angular.module('openshiftConsole')
  .filter('annotation', function() {
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
        return resource.metadata.annotations[key];
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
    return function(resource) {
      return annotationFilter(resource, "displayName");
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
  });
