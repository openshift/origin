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

  beforeEach(function(){
    inject(function(_$controller_, $q){
      // The injector unwraps the underscores (_) from around the parameter names when matching
      controller = _$controller_("CreateFromImageController", {
        $scope: $scope,
        $routeParams: $routeParams,
        DataService: {
          get: function(kind){
            var deferred = $q.defer();
            deferred.resolve({});
            return deferred.promise;
          }
        },
        Navigate: {
          toErrorPage: function(message){}
        }
      });
    });
  });

  it("valid http URL", function(){
    var match = 'http://example.com/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("valid http URL, without http part", function(){
    var match = 'example.com/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });


  it("valid http URL with user and password", function(){
    var match = 'http://user:pass@example.com/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("invalid http URL with user and password containing space", function(){
    var match = 'http://user:pa ss@example.com/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).toBeNull();
  });

  it("valid http URL with user and password with special characters", function(){
    var match = 'https://user:my!password@example.com/gerrit/p/myrepo.git'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("valid http URL with port", function(){
    var match = 'http://example.com:8080/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("valid http URL with port, user and password", function(){
    var match = 'http://user:pass@example.com:8080/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("valid https URL", function(){
    var match = 'https://example.com/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("valid ftp URL", function(){
    var match = 'ftp://example.com/dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("valid git+ssh URL", function(){
    var match = 'git@example.com:dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).not.toBeNull();
  });

  it("invalid git+ssh URL (double @@)", function(){
    var match = 'git@@example.com:dir1/dir2'.match($scope.sourceURLPattern);
    expect(match).toBeNull();
  });
});
