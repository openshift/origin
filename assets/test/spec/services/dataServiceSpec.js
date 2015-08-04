"use strict";

describe("DataService", function(){
  var DataService;

  beforeEach(function(){
    inject(function(_DataService_){
      DataService = _DataService_;
    });
  });

  describe("#url", function(){

    var tc = [
      // Empty tests
      [null,           null],
      [{},             null],

      // Unknown types
      [{type:''},      null],
      [{type:'bogus'}, null],

      // Type normalization
      [{type:'users'},             "http://localhost:8443/osapi/v1beta3/users"],
      [{type:'Users'},             "http://localhost:8443/osapi/v1beta3/users"],
      [{type:'oauthaccesstokens'}, "http://localhost:8443/osapi/v1beta3/oauthaccesstokens"],
      [{type:'OAuthAccessTokens'}, "http://localhost:8443/osapi/v1beta3/oauthaccesstokens"],
      [{type:'pods'},              "http://localhost:8443/api/v1/pods"],
      [{type:'Pods'},              "http://localhost:8443/api/v1/pods"],

      // Openshift type
      [{type:'builds'                           }, "http://localhost:8443/osapi/v1beta3/builds"],
      [{type:'builds', namespace:"foo"          }, "http://localhost:8443/osapi/v1beta3/namespaces/foo/builds"],
      [{type:'builds',                  id:"bar"}, "http://localhost:8443/osapi/v1beta3/builds/bar"],
      [{type:'builds', namespace:"foo", id:"bar"}, "http://localhost:8443/osapi/v1beta3/namespaces/foo/builds/bar"],

      // k8s type
      [{type:'replicationcontrollers'                           }, "http://localhost:8443/api/v1/replicationcontrollers"],
      [{type:'replicationcontrollers', namespace:"foo"          }, "http://localhost:8443/api/v1/namespaces/foo/replicationcontrollers"],
      [{type:'replicationcontrollers',                  id:"bar"}, "http://localhost:8443/api/v1/replicationcontrollers/bar"],
      [{type:'replicationcontrollers', namespace:"foo", id:"bar"}, "http://localhost:8443/api/v1/namespaces/foo/replicationcontrollers/bar"],

      // Subresources and webhooks
      [{type:'builds/clone',             id:"mybuild", namespace:"foo"                                 }, "http://localhost:8443/osapi/v1beta3/namespaces/foo/builds/mybuild/clone"],
      [{type:'buildconfigs/instantiate', id:"mycfg",   namespace:"foo"                                 }, "http://localhost:8443/osapi/v1beta3/namespaces/foo/buildconfigs/mycfg/instantiate"],
      [{type:'buildconfigs/webhooks',    id:"mycfg",   namespace:"foo", hookType:"github", secret:"123"}, "http://localhost:8443/osapi/v1beta3/namespaces/foo/buildconfigs/mycfg/webhooks/123/github"],

      // Watch
      [{type:'pods', namespace:"foo", isWebsocket:true                     }, "ws://localhost:8443/api/v1/watch/namespaces/foo/pods"],
      [{type:'pods', namespace:"foo", isWebsocket:true, resourceVersion:"5"}, "ws://localhost:8443/api/v1/watch/namespaces/foo/pods?resourceVersion=5"],

      // Namespaced subresource with params
      [{type:'pods/proxy', id:"mypod", namespace:"myns", myparam1:"myvalue"}, "http://localhost:8443/api/v1/namespaces/myns/pods/mypod/proxy?myparam1=myvalue"],
    ];

    angular.forEach(tc, function(item) {
      it('should generate a correct URL for ' + JSON.stringify(item[0]), function() {
        expect(DataService.url(item[0])).toEqual(item[1]);
      });
    });

  });

});
