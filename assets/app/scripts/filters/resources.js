angular.module('openshiftConsole')
  .filter('annotation', function() {
    return function(resource, key) {
      if (resource && resource.metadata && resource.metadata.annotations) {
        return resource.metadata.annotations[key];
      }
      return null;
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
  .filter('imageName', function() {
    return function(image) {
      // TODO move this parsing method into a utility method
      var slashSplit = image.split("/");
      var semiColonSplit;
      if (slashSplit.length === 3) {
        semiColonSplit = slashSplit[2].split(":");
        return slashSplit[1] + '/' + semiColonSplit[0];
      }
      else if (slashSplit.length === 2) {
        // TODO umm tough... this could be registry/imageName or imageRepo/imageName
        // have to check if the first bit matches a registry pattern, will handle this later...
        return image;
      }
      else if (slashSplit.length === 1) {
        semiColonSplit = image.split(":");
        return semiColonSplit[0];         
      }
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
  });