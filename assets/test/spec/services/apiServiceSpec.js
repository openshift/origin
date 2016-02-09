"use strict";

describe("APIService", function(){
  var APIService;

  beforeEach(function(){
    inject(function(_APIService_){
      APIService = _APIService_;
    });
  });

  describe("#toResourceGroupVersion", function(){

    var tc = [
      // string args
      // simple
      ['pods',         {r:'pods',g:'',v:'v1'}],
      // normalization
      ['Pods',         {r:'pods',g:'',v:'v1'}],
      // normalization preserves subresources
      ['PODS/FOO',     {r:'pods/FOO',g:'',v:'v1'}],
      
      // structured, resource only
      // simple
      [{resource:'pods'},         {r:'pods',g:'',v:'v1'}],
      // normalization
      [{resource:'Pods'},         {r:'pods',g:'',v:'v1'}],
      // normalization preserves subresources
      [{resource:'PODS/FOO'},     {r:'pods/FOO',g:'',v:'v1'}],

      // structured, with group
      // groups default version if known
      [{resource:'pods',group:''},           {r:'pods',g:'',          v:'v1'     }],
      [{resource:'jobs',group:'extensions'}, {r:'jobs',g:'extensions',v:'v1beta1'}],
      // unknown groups do not default version
      [{resource:'foos',group:'unknown'},    {r:'foos',g:'unknown',   v:undefined}],
      
      // structured, with version
      // groups default
      [{resource:'pods',version:'v1'},      {r:'pods',g:'',v:'v1'     }],
      [{resource:'pods',version:'v1beta3'}, {r:'pods',g:'',v:'v1beta3'}],
      
      // structured, fully specified
      [{resource:'pods',group:'',          version:'v1'},      {r:'pods',g:'',          v:'v1'     }],
      [{resource:'pods',group:'',          version:'v1beta3'}, {r:'pods',g:'',          v:'v1beta3'}],
      [{resource:'jobs',group:'extensions',version:'v1'},      {r:'jobs',g:'extensions',v:'v1'     }],
      [{resource:'jobs',group:'extensions',version:'v1beta1'}, {r:'jobs',g:'extensions',v:'v1beta1'}],
      [{resource:'foos',group:'unknown',   version:'v1'},      {r:'foos',g:'unknown',   v:'v1'     }],
      [{resource:'foos',group:'unknown',   version:'v1beta1'}, {r:'foos',g:'unknown',   v:'v1beta1'}]
    ];

    angular.forEach(tc, _.spread(function(input, expectedRGV) {
      it('should result in ' + JSON.stringify(expectedRGV) + ' when called with ' + JSON.stringify(input), function() {
        // Call once and compare the components
        var actualRGV = APIService.toResourceGroupVersion(input);
        expect(actualRGV.resource).toEqual(expectedRGV.r);
        expect(actualRGV.group   ).toEqual(expectedRGV.g);
        expect(actualRGV.version ).toEqual(expectedRGV.v);
        
        // Call again with the result and make sure it is returns the same thing
        var actualRGV2 = APIService.toResourceGroupVersion(actualRGV);
        expect(actualRGV).toEqual(actualRGV2);
      });
    }));

  });
  
  describe("#parseGroupVersion", function(){
    var tc = [
      // invalid cases
      [null,          undefined],
      ["",            undefined],
      ['foo/bar/baz', undefined],
      // legacy
      ['v1',      {group:'',version:'v1'     }],
      ['v1beta3', {group:'',version:'v1beta3'}],
      // groups
      ['foo/bar',      {group:'foo',version:'bar'}],
      ['FOO/BAR',      {group:'FOO',version:'BAR'}]
    ];
    angular.forEach(tc, _.spread(function(input, expectedGroupVersion) {
      it('should result in ' + JSON.stringify(expectedGroupVersion) + ' when called with ' + JSON.stringify(input), function() {
        expect(APIService.parseGroupVersion(input)).toEqual(expectedGroupVersion);
      });
    }));
  });  
  
  describe("#objectToResourceGroupVersion", function(){
    var tc = [
      // invalid cases
      [null,              undefined],
      ["",                undefined],
      [{},                undefined],
      [{kind:"Pod"},      undefined],
      [{apiVersion:"v1"}, undefined],
      
      // legacy
      [{kind:"Pod",      apiVersion:"v1"}, {g:'',v:'v1',r:'pods'}],

      // extensions
      [{kind:"Job",apiVersion:"extensions/v1beta1"}, {g:'extensions',v:'v1beta1',r:'jobs'}],
      [{kind:"Foo",apiVersion:"unknown/v1beta6"},    {g:'unknown',   v:'v1beta6',r:'foos'}],
    ];
    angular.forEach(tc, _.spread(function(input, expectedRGV) {
      it('should result in ' + JSON.stringify(expectedRGV) + ' when called with ' + JSON.stringify(input), function() {
        // Call once and compare the components
        var actualRGV = APIService.objectToResourceGroupVersion(input);
        if (expectedRGV) {
          expect(actualRGV.resource).toEqual(expectedRGV.r);
          expect(actualRGV.group   ).toEqual(expectedRGV.g);
          expect(actualRGV.version ).toEqual(expectedRGV.v);
        } else {
          expect(actualRGV).toEqual(expectedRGV);
        }
      });
    }));
  });  
  
  describe("#kindToResource", function(){
    var tc = [
      // invalid cases
      [null,              ""],
      ["",                ""],
      
      // pluralization
      ["foo",             "foos"],
      // pluralization with s
      ["foos",            "fooses"],
      // pluralization with y
      ["Policy",          "policies"],
      // special cases
      ["Endpoints",                  "endpoints"],
      ["SecurityContextConstraints", "securitycontextconstraints"],
    ];
    angular.forEach(tc, _.spread(function(kind, resource) {
      it('should result in ' + JSON.stringify(resource) + ' when called with ' + JSON.stringify(kind), function() {
        expect(APIService.kindToResource(kind)).toEqual(resource);
      });
    }));
  });  

  describe("#deriveTargetResource", function(){
    var tc = [
      // invalid cases
      [null,null,              undefined],
      ["","",                  undefined],
      [{},{},                  undefined],
      
      // simple resource, matching object overrides group/version
      ['pods', {kind:"Pod",apiVersion:"v1"},                 {r:'pods',g:'',          v:'v1'     }],
      ['pods', {kind:"Pod",apiVersion:"v2"},                 {r:'pods',g:'',          v:'v2'     }],
      ['jobs', {kind:"Job",apiVersion:"extensions/v1beta1"}, {r:'jobs',g:'extensions',v:'v1beta1'}],
      ['jobs', {kind:"Job",apiVersion:"extensions/v1beta2"}, {r:'jobs',g:'extensions',v:'v1beta2'}],

      // simple resource, non-matching object leaves group/version alone
      ['pods', {kind:"Foo",apiVersion:"v1"},                 {r:'pods',g:'',v:'v1'}],
      ['pods', {kind:"Foo",apiVersion:"v2"},                 {r:'pods',g:'',v:'v1'}],
      ['jobs', {kind:"Foo",apiVersion:"extensions/v1beta1"}, {r:'jobs',g:'',v:'v1'}],
      ['jobs', {kind:"Foo",apiVersion:"extensions/v1beta2"}, {r:'jobs',g:'',v:'v1'}],
      // actual use:
      ['deploymentconfigs/scale', {kind:"Scale",apiVersion:"extensions/v1beta1"}, {r:'deploymentconfigs/scale',g:'',v:'v1'}],

      // complex resource, matching object kind and group overrides version
      [{resource:'pods',group:''                        }, {kind:"Pod",apiVersion:"v3"},                 {r:'pods',g:'',          v:'v3'     }],
      [{resource:'pods',group:'',           version:'v2'}, {kind:"Pod",apiVersion:"v3"},                 {r:'pods',g:'',          v:'v3'     }],
      [{resource:'jobs',group:'extensions'              }, {kind:"Job",apiVersion:"extensions/v1beta3"}, {r:'jobs',g:'extensions',v:'v1beta3'}],
      [{resource:'jobs',group:'extensions', version:'v2'}, {kind:"Job",apiVersion:"extensions/v1beta3"}, {r:'jobs',g:'extensions',v:'v1beta3'}],

      // complex resource, non-matching object group leaves group/version alone
      [{resource:'pods',                                }, {kind:"Pod",apiVersion:"othergroup/v3"},      {r:'pods',g:'',          v:'v1'     }],
      [{resource:'pods',group:''                        }, {kind:"Pod",apiVersion:"othergroup/v3"},      {r:'pods',g:'',          v:'v1'     }],
      [{resource:'pods',group:'',           version:'v2'}, {kind:"Pod",apiVersion:"othergroup/v3"},      {r:'pods',g:'',          v:'v2'     }],
      [{resource:'jobs',group:'extensions'              }, {kind:"Job",apiVersion:"othergroup/v1beta3"}, {r:'jobs',g:'extensions',v:'v1beta1'}],
      [{resource:'jobs',group:'extensions', version:'v2'}, {kind:"Job",apiVersion:"othergroup/v1beta3"}, {r:'jobs',g:'extensions',v:'v2'     }],
      // complex resource, non-matching object kind leaves group/version alone 
      [{resource:'pods',group:''                        }, {kind:"Foo",apiVersion:"v3"},                 {r:'pods',g:'',          v:'v1'     }],
      [{resource:'pods',group:'',           version:'v2'}, {kind:"Foo",apiVersion:"v3"},                 {r:'pods',g:'',          v:'v2'     }],
      // actual use:
      [{resource:'deploymentconfigs/scale'},               {kind:"Scale",apiVersion:"extensions/v1beta1"}, {r:'deploymentconfigs/scale',g:'',v:'v1'}],
    ];
    angular.forEach(tc, _.spread(function(resource, apiObject, expectedRGV) {
      it('should result in ' + JSON.stringify(expectedRGV) + ' when called with ' + JSON.stringify(resource)+","+JSON.stringify(apiObject), function() {
        // Call once and compare the components
        var actualRGV = APIService.deriveTargetResource(resource, apiObject);
        if (expectedRGV) {
          expect(actualRGV.resource).toEqual(expectedRGV.r);
          expect(actualRGV.group   ).toEqual(expectedRGV.g);
          expect(actualRGV.version ).toEqual(expectedRGV.v);
        } else {
          expect(actualRGV).toEqual(expectedRGV);
        }
      });
    }));
  });  

  describe("#primaryResource", function(){
    var tc = [
      // invalid cases
      [null,              ""],
      ["",                ""],

      // no subresources
      ["foo",             "foo"],
      ["FOO",             "foo"],
      
      // subresource cases
      ["foo/bar",         "foo"],
      ["FOO/bar/baz",     "foo"]
    ];
    angular.forEach(tc, _.spread(function(resource, primaryResource) {
      it('should result in ' + JSON.stringify(primaryResource) + ' when called with ' + JSON.stringify(resource), function() {
        expect(APIService.toResourceGroupVersion(resource).primaryResource()).toEqual(primaryResource);
      });
    }));
  });  
  
  describe("#subresources", function(){
    var tc = [
      // invalid cases
      [null,              []],
      ["",                []],

      // no subresources
      ["foo",             []],
      ["FOO",             []],
      
      // subresource cases
      ["foo/bar",         ["bar"]],
      ["FOO/bar/baz",     ["bar","baz"]],
      ["FOO/Bar/Baz",     ["Bar","Baz"]]
    ];
    angular.forEach(tc, _.spread(function(resource, subresources) {
      it('should result in ' + JSON.stringify(subresources) + ' when called with ' + JSON.stringify(resource), function() {
        expect(APIService.toResourceGroupVersion(resource).subresources()).toEqual(subresources);
      });
    }));
  });    
  
});
