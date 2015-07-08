"use strict";
/* jshint unused: false */

describe("CreateController", function(){
  var controller, form;
  var $scope = {
    projectTemplates: {},
    openshiftTemplates: {},
    templatesByTag: {}
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
