"use strict";

describe("CreateFromImageController", function(){
  var controller;
  var $scope = {
    name: "apPname",
    projectName: "aProjectName"
  };
  var $routeParams = {
    imageName: "anImageName",
    imageTag: "latest",
    namespace: "aNamespace"
  };
  var DataService = {};
  var Navigate = {};
  
  
  beforeEach(function(){
    inject(function(_$controller_){
      // The injector unwraps the underscores (_) from around the parameter names when matching
      controller = _$controller_("CreateFromImageController", {
        $scope: $scope,
        $routeParams: $routeParams,
        DataService: {
          get: function(kind){
            return {};
          }
        },
        Navigate: {
          toErrorPage: function(message){}
        },
        NameGenerator: {
          suggestFromSourceUrl: function(sourceUrl, kinds, namespace){
            return "aName";
          }
        }
      });
    });
  });
});
