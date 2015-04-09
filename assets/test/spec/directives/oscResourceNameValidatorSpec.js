"use strict";

describe("oscResourceNameValidator", function(){
  
  var $scope, form;
  beforeEach(function(){
    inject(function($compile, $rootScope){
      $scope = $rootScope;
      var element = angular.element(
        '<form name="form">' +
        '<input ng-model="model.name" name="name" osc-resource-name-validator />' +
        '</form>'
      );
      $scope.model = { name: null };
      $compile(element)($scope);
      form = $scope.form;
    });
  });
  
  it("should disallow a null name", function(){
    form.name.$setViewValue(null);
    expect(form.name.$valid).toBe(false);
  });
  
  it("should disallow an empty name", function(){
    form.name.$setViewValue("");
    expect(form.name.$valid).toBe(false);
  });
  
  it("should disallow a blank name", function(){
    form.name.$setViewValue(" ");
    expect(form.name.$valid).toBe(false);
  });
  
  it("should disallow a name with a blank", function(){
    form.name.$setViewValue("foo bar");
    expect(form.name.$valid).toBe(false);
  });
  
  it("should disallow a name with starting with a .", function(){
    form.name.$setViewValue(".foobar");
    expect(form.name.$valid).toBe(false);
  });
  
  it("should disallow a name that is too long", function(){
    form.name.$setViewValue("abcdefghijklmnopqrstuvwxy");
    expect(form.name.$valid).toBe(false);
  });
  
  
  
  it("should allow a name with a dash", function(){
    form.name.$setViewValue("foo-bar");
    expect(form.name.$valid).toBe(true);
  });
  
  it("should allow a name with a dot", function(){
    form.name.$setViewValue("foo99.bar");
    expect(form.name.$valid).toBe(true);
  });
  
});

