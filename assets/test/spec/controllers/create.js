"use strict";
/* jshint unused: false */

describe("CreateController", function(){
  var controller, form;
  var $scope = {
  };

  beforeEach(function(){
    inject(function(_$controller_, $q){
      // The injector unwraps the underscores (_) from around the parameter names when matching
      controller = _$controller_("CreateController", {
        $scope: $scope,
        DataService: {
          list: function(resource, context, callback, opts) {
            return $q.when({
              _data: {},
              by: function() {
                return {};
              }
            });
          }
        },
        ProjectsService: {
          get: function(name) {
            return $q.when([
              {
                metadata: {
                  'name': 'foo',
                  'selfLink': '/oapi/v1/projects/foo',
                  'uid': 'c6fdde8d-979b-11e5-8493-080027c5bfa9',
                  'resourceVersion': '25334',
                  'creationTimestamp': '2015-11-30T19:51:41Z',
                  'annotations': {
                    'openshift.io/description': 'Foo',
                    'openshift.io/display-name': 'foo'
                  }
                },
                spec: {
                  'finalizers': [
                    'openshift.io/origin',
                    'kubernetes'
                  ]
                },
                status: {
                  'phase': 'Active'
                }
              }, {
                projectPromise: $q.when({}),
                projectName: 'foo',
                project: undefined
              }
            ]);
          }
        }
      });
    });
  });
});
