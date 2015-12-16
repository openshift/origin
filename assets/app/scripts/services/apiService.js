'use strict';
/* jshint unused:false */

// see gist: https://gist.github.com/benjaminapetersen/540ac605d3c1b0660062
// for discussion leading up to the creation of this file

angular.module('openshiftConsole')
  .factory('APIService', function($window, API_CFG, APIGROUP_CFG) {

    var defaultVersionForGroup = {
      'extensions':   'v1beta1',
      'experimental': 'v1beta1',
      '':             'v1'  // this group applies to legacy openshift & k8s (/oapi & /api)
    };


    // {group: '', resource: '', version: ''}
    var findAPIFor = function(qualified) {
      var resource = qualified.resource;
      // handle API Groups
      if(qualified.group && qualified.group.length) {
        // TODO: cannot validate until we have API discovery :(
        return {
          hostPort: APIGROUP_CFG.hostPort,
          prefix: APIGROUP_CFG.prefix,
          version: qualified.version || defaultVersionForGroup[qualified.group]
        };
      }

      // else, legacy
      var found = _.find(API_CFG, function(api) {
        return api.resources[_.first(resource.split('/'))] || api.resources['*']; // ['*'] ?
      });

      return found ?
              {
                hostPort: found.hostPort,
                prefix: found.prefixes[qualified.version] ||
                        found.prefixes[defaultVersionForGroup[qualified.group]] ||
                        found.prefixes['*'],
                version: qualified.version || defaultVersionForGroup[qualified.group]
              } :
              undefined;
    };


    // TODO: improve this when we have API discovery...
    // takes: {resource: '', group:'', verison: ''} or string "pods"
    // if string, will result in a v1 api, oapi response
    var apiExistsFor = function(unqualified) {
      var qualified = qualifyResource(unqualified);

      // if we have a group, this is the best we can do until we get discovery.
      // we have no way to know if the resources really exists at the endpoint at this time.
      if(qualified.group === '') {
        return !!findAPIFor(qualified);
      }
      return true;
    };


    // URITemplate has no ability to do conditional {+group}, so templates are now broken up
    var API_TEMPLATE = "{protocol}://{+hostPort}{+prefix}/{version}/";
    var API_GROUP_TEMPLATE = "{protocol}://{+hostPort}{+prefix}/{+group}/{version}/";

    var URL_GET_LIST              = "{resource}{?q*}";
    var URL_OBJECT                = "{resource}/{name}{/subresource*}{?q*}";
    var URL_NAMESPACED_GET_LIST   = "namespaces/{namespace}/{resource}{?q*}";
    var URL_NAMESPACED_OBJECT     = "namespaces/{namespace}/{resource}/{name}{/subresource*}{?q*}";


    var findTemplateFor = function(name, namespace, group) {
      var base = group ? API_GROUP_TEMPLATE : API_TEMPLATE;
      var rest = namespace ? URL_NAMESPACED_GET_LIST : URL_GET_LIST;
      if(name) {
        rest = namespace ? URL_NAMESPACED_OBJECT : URL_OBJECT;
      }
      return (base + rest);
    };


    var protocol = function(isWebsocket) {
      if(isWebsocket) {
        return $window.location.protocol === "http:" ?  "ws" : "wss";
      }
      return $window.location.protocol === "http:" ? "http" : "https";
    };


    var cleanCopyParams = function(params) {
      params = (params &&  angular.copy(params)) || {};
      delete params.namespace;
      return params;
    };


    // generates something like:
    // https://localhost:8443
    var openshiftAPIBaseUrl = function() {
      var protocol = $window.location.protocol === "http:" ? "http" : "https";
      var hostPort = API_CFG.openshift.hostPort;
      return new URI({protocol: protocol, hostname: hostPort}).toString();
    };

    // TODO: can this be implmeneted in a meangful way at this point?
    var resourceInfo = function(resource) {
      console.warn('TODO: resourceInfo is not implemented....!', !!findAPIFor(qualifyResource(resource)));
      return !!findAPIFor(qualifyResource(resource));
    };


    // port of restmapper.go#normalizeResource
    // TODO: upate to match the current go version func of the same name
    var normalizeResource = function(resource) {
       if (!resource) {
        return;
       }
       // only lowercase the first segment, leaving subresources as-is (some are case-sensitive)
       var segments = resource.split("/");
       segments[0] = segments[0].toLowerCase();
       var normalized = segments.join("/");
       if (resource !== normalized) {
         Logger.warn('Non-lower case resource "' + resource + '"');
       }
       return normalized;
    };


    // port of restmapper.go#kindToResource,
    // TODO: upate to match the current go version func of the same name
    var kindToResource = function(kind) {
      if (!kind) {
        return "";
      }
      var resource = String(kind).toLowerCase();

      if (_.endsWith(resource, 'status')) {
        resource = resource + 'es';
      } else if (_.endsWith(resource, 's')) {
        // no-op
      } else if (_.endsWith(resource, 'y')) {
        resource = resource.substring(0, resource.length-1) + 'ies';
      } else {
        resource = resource + 's';
      }
      // NOTE: previously had a check here for a 'known resource'
      return resource;
    };


    // if resource is string will convert to the newer object structure {resource: '', group: '', version: ''}
    // TODO: ensure kindToResouce() has been handed already...
    var qualifyResource = function(resourceWithSubresource, apiVersion) {

      if(!resourceWithSubresource) {
        return;
      }
      var qualified = _.isString(resourceWithSubresource) ? { resource: resourceWithSubresource } : _.clone(resourceWithSubresource); // clone for manipulation

      //qualified.resource = kindToResource(qualified.kind || resource);
      //qualified.resource = normalizeResource((qualified.kind && kindToResource(qualified.kind)) || qualified.resource);
      qualified.resource = normalizeResource(qualified.kind || qualified.resource);

      if(!qualified.group) {
        qualified.group = '';
      }
      if(!qualified.version) {
        qualified.version = apiVersion || defaultVersionForGroup[qualified.group];
      }
      // delete qualified.version; // apiVersion
      delete qualified.kind;
      return qualified;
    };

    // This function should be used by Data.js in situations where the resource is undefined/null:
    //  DataService.update(null, 'foo', data)
    //    .update() can internally call apiService.deriveResource(data)
    //    to get object {resource: '', group:'', version: ''} needed to submit
    // This will not work for GET as there is no data object being sent, but would work for POST, etc
    var deriveResource = function(data) {
      var parts = data.apiVersion && data.apiVersion.split('/') || [];
      var group = data.group || _.first(parts);
      var version = data.version || _.rest(parts);
      // in this case, we have a legacy apiVersion
      if(parts.length === 1) {
        version = parts;
        group = '';
      }
      return {
        resource:  (data.kind && kindToResource(data.kind)) || data.resource,
        group: group || '',
        version: (version && version[0]) || defaultVersionForGroup[group]
      };
    };


    var findNamespace = function(context, params) {
      if(params && params.namespace) {
        return params.namespace;
      }
      if(context && context.namespace) {
        return context.namespace;
      }
      if(context && context.project) {
        return context.project.metadata.name;
      }
    };


    var urlForResource = function(unqualifiedResource, name, apiVersion, context, isWebsocket, params) {
      var qualified = qualifyResource(unqualifiedResource, apiVersion);
      var namespace = findNamespace(context, params);

      return findAPIFor(qualified) ?
              URI
                .expand(
                  findTemplateFor(name, namespace, qualified.group),
                  _.extend({}, qualified, findAPIFor(qualified), {
                    resource: _.first(qualified.resource.split('/')),
                    subresource: _.rest(qualified.resource.split('/')),
                    name: name,
                    namespace: namespace,
                    q: cleanCopyParams(params),
                    protocol: protocol(isWebsocket)
                  })) :
                  null;
    };


    // NOTE: this is used in 2 places in app,
    // and is really just a proxy for urlForResource w/a
    // different syntax.  may be best to factor it out...
    var url = function(options) {
        if (options && options.resource) {
          var opts = angular.copy(options);
          delete opts.resource;
          delete opts.name;
          delete opts.apiVersion;
          delete opts.version;
          delete opts.group;
          delete opts.isWebsocket;
          // will let urlForResource qualify
          var unqualified = {
            resource: normalizeResource(options.resource),
            group: options.group,
            version: options.version || options.apiVersion
          };
          var u = urlForResource(unqualified, options.name, options.apiVersion, null, !!options.isWebsocket, opts);
          if (u) {
            return u.toString();
          }
        }
        return null;
      };


    return {
      qualifyResource: qualifyResource,
      deriveResource: deriveResource,             // DataService.js (future impl)
      normalizeResource: normalizeResource,       // DataService.js
      kindToResource: kindToResource,             // DataService.js, createFromImage.js
      apiExistsFor: apiExistsFor,                 // DataService.js
      openshiftAPIBaseUrl: openshiftAPIBaseUrl,   // nextSteps.js
      urlForResource: urlForResource,             // DataService.js
      url: url                                    // javaLink.js, resources.js
    };

  });
