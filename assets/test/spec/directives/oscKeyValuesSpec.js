"use strict";

describe("KeyValuesEntryController", function(){
    var scope, controller;
    beforeEach(function(){
      scope = {
        value: "foo"
      };
      inject(function(_$controller_){
        // The injector unwraps the underscores (_) from around the parameter names when matching
       controller = _$controller_("KeyValuesEntryController", {$scope: scope});
      });
    });

    describe("#edit", function(){
      it("should copy the original value", function(){
        scope.edit();
        expect(scope.value).toEqual(scope.originalValue);
        expect(scope.editing).toEqual(true);
      });
    });

    describe("#cancel", function(){
      it("should reset value to the original value", function(){
        scope.originalValue = "bar";
        scope.cancel();
        expect(scope.value).toEqual("bar");
        expect(scope.editing).toEqual(false);
      });
    });

    describe("#update", function(){
      var entries = { foo: "abc"};
      it("should update the entries for the key when the value is not empty", function(){
        scope.update("foo", "bar", entries);
        expect(entries["foo"]).toEqual("bar");
        expect(scope.editing).toEqual(false);
      });
    });
});

describe('oscKeyValues', function() {
  var scope;
  var isolateScope;
  var ctrl;
  var elem;

  beforeEach(module('openshiftConsole'));
  beforeEach(inject(function($rootScope, $compile) {
    elem = angular.element('<osc-key-values></osc-key-values>');
    scope = $rootScope.$new();
    $compile(elem)(scope);
    ctrl = elem.controller;
    $rootScope.$digest();
  }));

  describe('#addEntry', function() {
    it("should not add the entry if the key is in the readonly list", function(){
      // a gotcha of testing directives with an isolate $scope,
      // you cannot use the scope passed to $compile(elem)(scope);
      isolateScope = elem.isolateScope();
      isolateScope.entries = {};
      isolateScope.readonlyKeys = "abc";
      isolateScope.key = "abc";
      isolateScope.value = "xyz";
      isolateScope.addEntry();
      expect(isolateScope.entries["abc"]).toBe(undefined);
      expect(isolateScope.key).toEqual("abc");
      expect(isolateScope.value).toEqual("xyz");
    });

    it("should add the key/value to the scope", function(){
      isolateScope = elem.isolateScope();
      isolateScope.entries = {};
      isolateScope.key = "foo";
      isolateScope.value = "bar";
      isolateScope.addEntry();
      isolateScope.key = "abc";
      isolateScope.value = "def";
      isolateScope.addEntry();
      expect(isolateScope.entries["foo"]).toEqual("bar");
      expect(isolateScope.entries["abc"]).toEqual("def");
      expect(isolateScope.key).toEqual(null);
      expect(isolateScope.value).toEqual(null);
    });
  });

  describe('#deleteEntry', function() {
    it("should delete the key/value from the scope", function(){
      isolateScope = elem.isolateScope();
      isolateScope.entries = {
        "foo" : "bar"
      };
      isolateScope.deleteEntry("foo");
      expect(isolateScope.entries["foo"]).toBe(undefined);
      expect(isolateScope.form.$dirty).toBe(true);
    });
  });

  describe("#allowDelete", function(){

    it("should when the deletePolicy equals always", function(){
      isolateScope = elem.isolateScope();
      expect(isolateScope.allowDelete("foo")).toBe(true);
    });

    it("should not when the deletePolicy equals never", function(){
      isolateScope = elem.isolateScope();
      isolateScope.deletePolicy = "never";
      expect(isolateScope.allowDelete("foo")).toBe(false);
    });

    it("should when the deletePolicy equals added and the entry was not originally in entries", function(){
      isolateScope = elem.isolateScope();
      isolateScope.entries = {};
      isolateScope.deletePolicy = "added";
      isolateScope.key = "abc";
      isolateScope.value = "def";
      isolateScope.addEntry();
      expect(isolateScope.allowDelete("abc")).toBe(true);
    });

    it("should not when the deletePolicy equals added and the entry was originally in entries", function(){
      isolateScope.deletePolicy = "added";
      expect(isolateScope.allowDelete("foo")).toBe(false);
    });

  });

});
