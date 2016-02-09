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
.factory('APIService', function(API_CFG, APIS_CFG, Logger) {
  
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
      version = r.version || defaultVersion[group];
    }
    return new ResourceGroupVersion(resource, group, version);
  };
  
  // normalizeResource lowercases the first segment of the given resource. subresources can be case-sensitive.
  var normalizeResource = function(resource) {
    if (!resource) {
      return resource;
    }
    var i = resource.indexOf('/');
    if (i === -1) {
      return resource.toLowerCase();
    }
    return resource.substring(0, i).toLowerCase() + resource.substring(i);
  };
  
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
  var kindToResource = function(kind) {
    if (!kind) {
      return "";
    }
    var resource = String(kind).toLowerCase();
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
  };
  
  // apiInfo returns the host/port, prefix, group, and version for the given resource,
  // or undefined if the specified resource/group/version is known not to exist.
  var apiInfo = function(resource) {
    resource = toResourceGroupVersion(resource);
    var primaryResource = resource.primaryResource();
    
    // API info for resources in an API group can just be derived
    // TODO: use API discovery to determine available groups, versions, and resources
    // If this is called before discovery loads, just return the derived data, but if we know from discovery that a group/version/resource is invalid, return undefined.
    if (resource.group) {
      return {
        hostPort: APIS_CFG.hostPort,
        prefix:   APIS_CFG.prefix,
        group:    resource.group,
        version:  resource.version
      };
    }
    
    // Resources without an API group could be legacy k8s or origin resources.
    // Scan through resources to determine which this is.
    var api, prefix;
    for (var apiName in API_CFG) {
      api = API_CFG[apiName];
      if (!api.resources[primaryResource]) {
        continue;
      }
      prefix = api.prefixes[resource.version];
      if (!prefix) {
        continue;
      }
      return {
        hostPort: api.hostPort,
        prefix:   prefix,
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
    
  return {
    toResourceGroupVersion: toResourceGroupVersion,
    
    parseGroupVersion: parseGroupVersion,
    
    objectToResourceGroupVersion: objectToResourceGroupVersion,
    
    deriveTargetResource: deriveTargetResource,
    
    kindToResource: kindToResource,
    
    apiInfo: apiInfo,
    
    invalidObjectKindOrVersion: invalidObjectKindOrVersion,
    unsupportedObjectKindOrVersion: unsupportedObjectKindOrVersion
  };
});
