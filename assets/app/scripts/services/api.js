'use strict';

// ResourceGroupVersion represents a fully qualified resource
function ResourceGroupVersion(resource, group, version) {
  this.resource = resource;
  this.group    = group;
  this.version  = version;
  return this;
}
// toString() includes the group and version information if present
ResourceGroupVersion.prototype.toString = function() {
  var s = this.resource;
  if (this.group)   { s += "/" + this.group;   }
  if (this.version) { s += "/" + this.version; }
  return s;
};
// primaryResource() returns the resource with any subresources removed
ResourceGroupVersion.prototype.primaryResource = function() {
  if (!this.resource) { return ""; }
  var i = this.resource.indexOf('/');
  if (i === -1) { return this.resource; }
  return this.resource.substring(0,i);
};
// subresources() returns a (possibly empty) list of subresource segments
ResourceGroupVersion.prototype.subresources = function() {
  var segments = (this.resource || '').split("/");
  segments.shift();
  return segments;
};
// equals() returns true if the given resource, group, and version match.
// If omitted, group and version are not compared.
ResourceGroupVersion.prototype.equals = function(resource, group, version) {
  if (this.resource !== resource) { return false; }
  if (arguments.length === 1)     { return true;  }
  if (this.group !== group)       { return false; }
  if (arguments.length === 2)     { return true;  }
  if (this.version !== version)   { return false; }
  return true;
};


angular.module('openshiftConsole')
.factory('APIService', function(API_CFG, APIS_CFG, AuthService, Constants, Logger, $q, $http, Navigate, $filter) {
  // Set the default api versions the console will use if otherwise unspecified
  var defaultVersion = {
    "":           "v1",
    "extensions": "v1beta1"
  };

  // toResourceGroupVersion() returns a ResourceGroupVersion.
  // If resource is already a ResourceGroupVersion, returns itself.
  //
  // if r is a string, the empty group and default version for the empty group are assumed.
  //
  // if r is an object, the resource, group, and version attributes are read.
  // a missing group attribute defaults to the legacy group.
  // a missing version attribute defaults to the default version for the group, or undefined if the group is unknown.
  //
  // if r is already a ResourceGroupVersion, it is returned as-is
  var toResourceGroupVersion = function(r) {
    if (r instanceof ResourceGroupVersion) {
      return r;
    }
    var resource, group, version;
    if (angular.isString(r)) {
      resource = normalizeResource(r);
      group = '';
      version = defaultVersion[group];
    } else if (r && r.resource) {
      resource = normalizeResource(r.resource);
      group = r.group || '';
      version = r.version || defaultVersion[group] || _.get(APIS_CFG.groups[group], "preferredVersion.version");
    }
    return new ResourceGroupVersion(resource, group, version);
  };

  // normalizeResource lowercases the first segment of the given resource. subresources can be case-sensitive.
  function normalizeResource(resource) {
    if (!resource) {
      return resource;
    }
    var i = resource.indexOf('/');
    if (i === -1) {
      return resource.toLowerCase();
    }
    return resource.substring(0, i).toLowerCase() + resource.substring(i);
  }

  // port of group_version.go#ParseGroupVersion
  var parseGroupVersion = function(apiVersion) {
    if (!apiVersion) {
      return undefined;
    }
    var parts = apiVersion.split("/");
    if (parts.length === 1) {
      return {group:'', version: parts[0]};
    }
    if (parts.length === 2) {
      return {group:parts[0], version: parts[1]};
    }
    Logger.warn('Invalid apiVersion "' + apiVersion + '"');
    return undefined;
  };

  var objectToResourceGroupVersion = function(apiObject) {
    if (!apiObject || !apiObject.kind || !apiObject.apiVersion) {
      return undefined;
    }
    var resource = kindToResource(apiObject.kind);
    if (!resource) {
      return undefined;
    }
    var groupVersion = parseGroupVersion(apiObject.apiVersion);
    if (!groupVersion) {
      return undefined;
    }
    return new ResourceGroupVersion(resource, groupVersion.group, groupVersion.version);
  };

  // deriveTargetResource figures out the fully qualified destination to submit the object to.
  // if resource is a string, and the object's kind matches the resource, the object's group/version are used.
  // if resource is a ResourceGroupVersion, and the object's kind and group match, the object's version is used.
  // otherwise, resource is used as-is.
  var deriveTargetResource = function(resource, object) {
    if (!resource || !object) {
      return undefined;
    }
    var objectResource = kindToResource(object.kind);
    var objectGroupVersion = parseGroupVersion(object.apiVersion);
    var resourceGroupVersion = toResourceGroupVersion(resource);
    if (!objectResource || !objectGroupVersion || !resourceGroupVersion) {
      return undefined;
    }

    // We specified something like "pods"
    if (angular.isString(resource)) {
      // If the object had a matching kind {"kind":"Pod","apiVersion":"v1"}, use the group/version from the object
      if (resourceGroupVersion.equals(objectResource)) {
        resourceGroupVersion.group = objectGroupVersion.group;
        resourceGroupVersion.version = objectGroupVersion.version;
      }
      return resourceGroupVersion;
    }

    // If the resource was already a fully specified object,
    // require the group to match as well before taking the version from the object
    if (resourceGroupVersion.equals(objectResource, objectGroupVersion.group)) {
      resourceGroupVersion.version = objectGroupVersion.version;
    }
    return resourceGroupVersion;
  };

  // port of restmapper.go#kindToResource
  // humanize will add spaces between words in the resource
  function kindToResource(kind, humanize) {
    if (!kind) {
      return "";
    }
    var resource = kind;
    if (humanize) {
      var humanizeKind = $filter("humanizeKind");
      resource = humanizeKind(resource);
    }
    resource = String(resource).toLowerCase();
    if (resource === 'endpoints' || resource === 'securitycontextconstraints') {
      // no-op, plural is the singular
    }
    else if (resource[resource.length-1] === 's') {
      resource = resource + 'es';
    }
    else if (resource[resource.length-1] === 'y') {
      resource = resource.substring(0, resource.length-1) + 'ies';
    }
    else {
      resource = resource + 's';
    }

    return resource;
  }
  
  // apiInfo returns the host/port, prefix, group, and version for the given resource,
  // or undefined if the specified resource/group/version is known not to exist.
  var apiInfo = function(resource) {
    // If API discovery had any failures, calls to api info should redirect to the error page
    if (APIS_CFG.API_DISCOVERY_ERRORS) {
      var possibleCertFailure  = _.every(APIS_CFG.API_DISCOVERY_ERRORS, function(error){
        return _.get(error, "data.status") === 0;
      });
      if (possibleCertFailure && !AuthService.isLoggedIn()) {
        // will trigger a login flow which will redirect to the api server
        AuthService.withUser();
        return;
      }
      // Otherwise go to the error page, the server might be down.
      Navigate.toErrorPage("Unable to load details about the server. If the problem continues, please contact your system administrator.", "API_DISCOVERY", true);
      return;
    }
      
    resource = toResourceGroupVersion(resource);
    var primaryResource = resource.primaryResource();

    // API info for resources in an API group, if the resource was not found during discovery return undefined
    if (resource.group) {
      if (!_.get(APIS_CFG, ["groups", resource.group, "versions", resource.version, "resources", primaryResource])) {
        return undefined;
      }
      return {
        hostPort: APIS_CFG.hostPort,
        prefix:   APIS_CFG.prefix,
        group:    resource.group,
        version:  resource.version
      };
    }

    // Resources without an API group could be legacy k8s or origin resources.
    // Scan through resources to determine which this is.
    var api;
    for (var apiName in API_CFG) {
      api = API_CFG[apiName];
      if (!_.get(api, ["resources", resource.version, primaryResource])) {
        continue;
      }
      return {
        hostPort: api.hostPort,
        prefix:   api.prefix,
        version:  resource.version
      };
    }
    return undefined;
  };

  var invalidObjectKindOrVersion = function(apiObject) {
    var kind = "<none>";
    var version = "<none>";
    if (apiObject && apiObject.kind)       { kind    = apiObject.kind;       }
    if (apiObject && apiObject.apiVersion) { version = apiObject.apiVersion; }
    return "Invalid kind ("+kind+") or API version ("+version+")";
  };
  var unsupportedObjectKindOrVersion = function(apiObject) {
    var kind = "<none>";
    var version = "<none>";
    if (apiObject && apiObject.kind)       { kind    = apiObject.kind;       }
    if (apiObject && apiObject.apiVersion) { version = apiObject.apiVersion; }
    return "The API version "+version+" for kind " + kind + " is not supported by this server";
  };

  // Returns an array of available kinds, including their group
  var calculateAvailableKinds = function(includeClusterScoped) {
    var kinds = [];   
    var rejectedKinds = Constants.AVAILABLE_KINDS_BLACKLIST;
    
    // Legacy openshift and k8s kinds
    _.each(API_CFG, function(api) {
      _.each(api.resources.v1, function(resource) {
        if (resource.namespaced || includeClusterScoped) {
          // Exclude subresources and any rejected kinds
          if (resource.name.indexOf("/") >= 0 || _.contains(rejectedKinds, resource.kind)) {
            return;
          }

          kinds.push({
            kind: resource.kind
          });
        }
      });      
    });
   
   // Kinds under api groups
    _.each(APIS_CFG.groups, function(group) {
      // Use the console's default version first, and the server's preferred version second
      var preferredVersion = defaultVersion[group.name] || group.preferredVersion;
      _.each(group.versions[preferredVersion].resources, function(resource) {
        // Exclude subresources and any rejected kinds
        if (resource.name.indexOf("/") >= 0 || _.contains(rejectedKinds, resource.kind)) {
          return;
        }

        // Exclude duplicate kinds we know about that map to the same storage as another group/kind
        // This is unusual, so we are special casing these
        if (group.name === "autoscaling" && resource.kind === "HorizontalPodAutoscaler" ||
            group.name === "batch" && resource.kind === "Job"
        ) {
          return;
        }
               
        if (resource.namespaced || includeClusterScoped) {
          kinds.push({
            kind: resource.kind,
            group: group.name
          });
        }
      });
    });

    return _.uniq(kinds, false, function(value) {
      return value.group + "/" + value.kind;
    });
  };
  
  var namespacedKinds = calculateAvailableKinds(false);
  var allKinds = calculateAvailableKinds(true);
  
  var availableKinds = function(includeClusterScoped) {
    return includeClusterScoped ? allKinds : namespacedKinds;
  };

  return {
    toResourceGroupVersion: toResourceGroupVersion,

    parseGroupVersion: parseGroupVersion,

    objectToResourceGroupVersion: objectToResourceGroupVersion,

    deriveTargetResource: deriveTargetResource,

    kindToResource: kindToResource,

    apiInfo: apiInfo,

    invalidObjectKindOrVersion: invalidObjectKindOrVersion,
    unsupportedObjectKindOrVersion: unsupportedObjectKindOrVersion,
    availableKinds: availableKinds
  };
});
