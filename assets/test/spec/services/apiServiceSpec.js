"use strict";

describe("APIService", function(){
  var APIService;

  beforeEach(function(){
    inject(function(_APIService_){
      APIService = _APIService_;
    });
  });

  describe('#qualifyResource', function() {
    // empty group represents the legacy v1 api/oapi
    describe('when given a bare string', function() {
      it('creates a fully qualified object with an empty group and the v1 version', function() {
        expect(APIService.qualifyResource('pods'))
          .toEqual({ resource : 'pods', group : '', version : 'v1' });
      });
    });

    describe('when given just a resource', function() {
      it('creates a fully qualified object with an empty group and the v1 version', function() {
        expect(APIService.qualifyResource({resource: 'pods'}))
          .toEqual({ resource : 'pods', group : '', version : 'v1' });
      });
    });

    describe('when given a resource and an empty group', function() {
      it('adds the v1 version to the qualified object', function() {
        expect(APIService.qualifyResource({resource: 'pods', group: ''}))
          .toEqual({ resource : 'pods', group : '', version : 'v1' });
      });
    });

    describe('when given a resource and a group with a known version', function() {
      it('adds the known version', function() {
        expect(APIService.qualifyResource({resource: 'horizontalpodautoscalers', group: 'extensions'}))
          .toEqual({ resource : 'horizontalpodautoscalers', group : 'extensions', version : 'v1beta1' });
      });
    });

    describe('when given a resource, group, and version', function() {
      it('returns the same object', function() {
        expect(APIService.qualifyResource({resource: 'horizontalpodautoscalers', group: 'extensions',  version : 'v1beta1'}))
          .toEqual({ resource : 'horizontalpodautoscalers', group : 'extensions', version : 'v1beta1' });
      });
    });

  });

  describe('#deriveResource', function() {
    // TODO: would be better to make these individual tests that demonstrate the functions behavior quality
    var tc = [
      [{
        apiVersion: "extensions/v1beta1",
        kind: "HorizontalPodAutoscaler",
        metadata: {},
        spec: {}
      }, {resource: 'horizontalpodautoscalers', group: 'extensions', version: 'v1beta1'}],
      [{
         kind: "Route",
         apiVersion: 'v1',
         metadata: {},
         spec: {}
      }, {resource: 'routes', group: '', version: 'v1'}],
      [{
        apiVersion: 'v1',
        kind: "DeploymentConfig",
        metadata: {}
      }, {resource: 'deploymentconfigs', group: '', version: 'v1'}]
    ];

    _.forEach(tc, _.spread(function(data, resource) {
      it('derives a qualified resource form a provided data object', function() {
          expect(APIService.deriveResource(data)).toEqual(resource);
      });
    }));
  });

  describe('#normalizeResource', function() {
    // rather than using real examples like `build/log`, the strings here illustrate the desired behavior
    describe('when the resource is all lowercase', function() {
      it('provides the resource as is', function() {
        var resource = 'alllowercase/beforeandafter';
        expect(APIService.normalizeResource(resource)).toEqual(resource);
      });
    });

    describe('when the resource contains mixed case before the slash', function() {
      it('lower cases the first segment', function() {
        var resource = 'MixedCaseStuff/beforeandafter';
        expect(APIService.normalizeResource(resource)).toEqual('mixedcasestuff/beforeandafter');
      });
    });
    describe('when the resource contains upper case after the slash', function() {
      it('does nothing to post slash segment', function() {
        var resource = 'FOO/BarBaz';
        expect(APIService.normalizeResource(resource)).toEqual('foo/BarBaz');
      });
    });

    describe('when the resource does not contain a slash', function() {
      it('provides the resource as is', function() {
        var resource = 'foobarbaz';
        expect(APIService.normalizeResource(resource)).toEqual(resource);
      });
    });

    describe('when the resource is falsy', function() {
      var falsyValues = [null, undefined, ''];
      _.each(falsyValues, function(isFalsy) {
        it('provides undefined', function() {
          expect(APIService.normalizeResource(isFalsy)).toEqual(undefined);
        });
      });
    });

  });

  describe('#kindToResource', function() {
    var tc = [
      ['Test',                'tests'],
      ['Route',               'routes'],
      ['ImageStream',         'imagestreams'],
      ['BuildConfig',         'buildconfigs'],
      ['DeploymentConfig',    'deploymentconfigs'],
      ['Service',             'services'],
      ['Policy',              'policies'],
      ['Identity',            'identities'],
      ['ClusterRole',         'clusterroles'],
      ['ComponentStatus',     'componentstatuses']
    ];
    _.forEach(tc, _.spread(function(kind, resource) {
      // TODO: implies it actually knows, but really it just lowercase/pluralizes a string
      // TEST: null, undefined, empty string... what should these do?
      it('converts a kind to a resource by lowercasing & pluralizing the string', function() {
          expect(APIService.kindToResource(kind)).toEqual(resource);
      });
    }));

  });

  describe('#apiExistsFor', function() {
    // var tc = [
    //   [{resource: "routes", group: "", version: "v1"}, true],
    //   [{resource: "imagestreams", group: "", version: "v1"}, true],
    //   [{resource: "buildconfigs", group: "", version: "v1"}, true]
    // ];

    it('indicates a known resource for a v1 api exists', function() {
      var resource = {resource: "services", group: "", version: "v1"};
      expect(APIService.apiExistsFor(resource)).toEqual(true);
    });

    it('indicates an unknown resource for a v1 api does not exist', function() {
      var resource = {resource: "blueberries", group: "", version: "v1"};
      expect(APIService.apiExistsFor(resource)).toEqual(false);
    });

    it('indicates that a resource in an apigroup exists, though we cannot be certain without discovery', function() {
      var resource = {resource:'horizontalpodautoscalers', group: 'extensions', version: 'v1beta1' };
      expect(APIService.apiExistsFor(resource)).toEqual(true);
    });

  });

  describe('#openshiftAPIBaseUrl', function() {
    // testable? depends on the window object.
    // TODO: will need to mock.
  });

  describe('#urlForResource', function() {

    it('creates a non-namespaced url for a projects request', function() {
      var args = ['projects', 'foo', null, {}, false, {}];
      var url = 'http://localhost:8443/oapi/v1/projects/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

    it('creates a namespaced url for a resource with a namespace in the params object', function() {
      var args = ['replicationcontrollers', 'foo', null, {}, false, {namespace: 'bar'}];
      var url = 'http://localhost:8443/api/v1/namespaces/bar/replicationcontrollers/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

    it('creates a namespaced url for a resource with a project on the context object', function() {
      var args = ['replicationcontrollers', 'foo', null, {project: {metadata: {name: 'bar'}}}];
      var url = 'http://localhost:8443/api/v1/namespaces/bar/replicationcontrollers/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

    it('creates a url for a resource with an API group defined using a default version for the group (if known)', function() {
      var args = [{resource:'horizontalpodautoscalers', group: 'extensions' }, 'foo', null, {project: {metadata: {name: 'bar'}}}];
      var url = 'http://localhost:8443/apis/extensions/v1beta1/namespaces/bar/horizontalpodautoscalers/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

    it('creates a url for a resource with an API group defined using a defined version', function() {
      var args = [{resource:'horizontalpodautoscalers', group: 'extensions', version: 'v1beta1' }, 'foo', null, {project: {metadata: {name: 'bar'}}}];
      var url = 'http://localhost:8443/apis/extensions/v1beta1/namespaces/bar/horizontalpodautoscalers/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

    // do not provide a namespace, generally a problem with shared scope across controllers/directives
    it('creates an INCORRECTLY namespaced url for a projects request if the context has a namespace', function() {
      var args = ['projects', 'foo', null, {namespace: 'foo'}, false, {}];
      var url = 'http://localhost:8443/oapi/v1/namespaces/foo/projects/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

    // do not provide a namespace, generally a problem with shared scope across controllers/directives
    it('creates an INCORRECTLY namespaced url for a projects request if the params object has a namespace', function() {
      var args = ['projects', 'foo', null, {}, false, {namespace: 'foo'}];
      var url = 'http://localhost:8443/oapi/v1/namespaces/foo/projects/foo';
      expect(APIService.urlForResource.apply(APIService, args).toString()).toEqual(url);
    });

  });


  describe("#url", function(){
    // TODO: break this into individual it('') statements that clearly describe the intended behavior
    var tc = [
      // Empty tests
      [null,           null],
      [{},             null],

      // Unknown resources
      [{resource:''},      null],
      [{resource:'bogus'}, null],

      // Kind is not allowed
      [{resource:'Pod'},  null],
      [{resource:'User'}, null],

      // resource normalization
      [{resource:'users'},             "http://localhost:8443/oapi/v1/users"],
      [{resource:'Users'},             "http://localhost:8443/oapi/v1/users"],
      [{resource:'oauthaccesstokens'}, "http://localhost:8443/oapi/v1/oauthaccesstokens"],
      [{resource:'OAuthAccessTokens'}, "http://localhost:8443/oapi/v1/oauthaccesstokens"],
      [{resource:'pods'},              "http://localhost:8443/api/v1/pods"],
      [{resource:'Pods'},              "http://localhost:8443/api/v1/pods"],

      // Openshift resource
      [{resource:'builds'                             }, "http://localhost:8443/oapi/v1/builds"],
      [{resource:'builds', namespace:"foo"            }, "http://localhost:8443/oapi/v1/namespaces/foo/builds"],
      [{resource:'builds',                  name:"bar"}, "http://localhost:8443/oapi/v1/builds/bar"],
      [{resource:'builds', namespace:"foo", name:"bar"}, "http://localhost:8443/oapi/v1/namespaces/foo/builds/bar"],

      // k8s resource
      [{resource:'replicationcontrollers'                             }, "http://localhost:8443/api/v1/replicationcontrollers"],
      [{resource:'replicationcontrollers', namespace:"foo"            }, "http://localhost:8443/api/v1/namespaces/foo/replicationcontrollers"],
      [{resource:'replicationcontrollers',                  name:"bar"}, "http://localhost:8443/api/v1/replicationcontrollers/bar"],
      [{resource:'replicationcontrollers', namespace:"foo", name:"bar"}, "http://localhost:8443/api/v1/namespaces/foo/replicationcontrollers/bar"],

      // Subresources and webhooks
      [{resource:'pods/proxy',                           name:"mypod:1123", namespace:"foo"}, "http://localhost:8443/api/v1/namespaces/foo/pods/mypod%3A1123/proxy"],
      [{resource:'builds/clone',                         name:"mybuild",    namespace:"foo"}, "http://localhost:8443/oapi/v1/namespaces/foo/builds/mybuild/clone"],
      [{resource:'buildconfigs/instantiate',             name:"mycfg",      namespace:"foo"}, "http://localhost:8443/oapi/v1/namespaces/foo/buildconfigs/mycfg/instantiate"],
      [{resource:'buildconfigs/webhooks/123/github',     name:"mycfg",      namespace:"foo"}, "http://localhost:8443/oapi/v1/namespaces/foo/buildconfigs/mycfg/webhooks/123/github"],
      [{resource:'buildconfigs/webhooks/123?234/github', name:"mycfg",      namespace:"foo"}, "http://localhost:8443/oapi/v1/namespaces/foo/buildconfigs/mycfg/webhooks/123%3F234/github"],
      // Subresources aren't lowercased
      [{resource:'buildconfigs/webhooks/Aa1/github',     name:"mycfg",      namespace:"foo"}, "http://localhost:8443/oapi/v1/namespaces/foo/buildconfigs/mycfg/webhooks/Aa1/github"],



      // Plain websocket
      [{resource:'pods', namespace:"foo", isWebsocket:true      }, "ws://localhost:8443/api/v1/namespaces/foo/pods"],

      // Watch resource
      [{resource:'pods', namespace:"foo", isWebsocket:true, watch: true                       }, "ws://localhost:8443/api/v1/namespaces/foo/pods?watch=true"],
      [{resource:'pods', namespace:"foo", isWebsocket:true, watch: true, resourceVersion:"5"  }, "ws://localhost:8443/api/v1/namespaces/foo/pods?watch=true&resourceVersion=5"],

      // Follow log
      [{resource:'pods/log', namespace:"foo", isWebsocket:true, follow: true                       }, "ws://localhost:8443/api/v1/namespaces/foo/pods?follow=true"],
      [{resource:'builds/log', namespace:"foo", isWebsocket:true, follow: true                     }, "ws://localhost:8443/oapi/v1/namespaces/foo/builds?follow=true"],

      // Namespaced subresource with params
      [{resource:'pods/proxy', name:"mypod", namespace:"myns", myparam1:"myvalue"}, "http://localhost:8443/api/v1/namespaces/myns/pods/mypod/proxy?myparam1=myvalue"],

      // APIGroups {resource: '', group: '', version: ''}
      [{resource:'horizontalpodautoscalers', group: 'extensions', version:'v1beta1', namespace: 'foo'}, "http://localhost:8443/apis/extensions/v1beta1/namespaces/foo/horizontalpodautoscalers"],
      [{resource: 'pods', version: 'v1'}, 'http://localhost:8443/api/v1/pods'],
      [{resource: 'pods', group: ''}, 'http://localhost:8443/api/v1/pods']
    ];

    _.each(tc, _.spread(function(obj, path) {
      it('should generate a correct URL for ' + JSON.stringify(obj), function() {
        expect(APIService.url(obj)).toEqual(path);
      });
    }));
  });

});
