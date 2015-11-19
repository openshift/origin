"use strict";
/* jshint unused: false */

describe("CreateController", function(){
  var controller, form;
  var $scope = {
  };

  beforeEach(function(){
    inject(function(_$controller_){
      // The injector unwraps the underscores (_) from around the parameter names when matching
      controller = _$controller_("CreateController", {
        $scope: $scope,
        DataService: {
          list: function(templates){}
        }
      });
    });
  });
});
