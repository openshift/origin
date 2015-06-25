'use strict';

describe('KeyValuesEntryController', function(){
    var scope, controller;
    beforeEach(function(){
      scope = {
        value: 'foo'
      };
      inject(function(_$controller_){
        // The injector unwraps the underscores (_) from around the parameter names when matching
       controller = _$controller_('KeyValuesEntryController', {$scope: scope});
      });
    });

    describe('#edit', function(){
      it('should copy the original value', function(){
        scope.edit();
        expect(scope.value).toEqual(scope.originalValue);
        expect(scope.editing).toEqual(true);
      });
    });

    describe('#cancel', function(){
      it('should reset value to the original value', function(){
        scope.originalValue = 'bar';
        scope.cancel();
        expect(scope.value).toEqual('bar');
        expect(scope.editing).toEqual(false);
      });
    });

    describe('#update', function(){
      var entries = { foo: 'abc'};
      it('should update the entries for the key when the value is not empty', function(){
        scope.update('foo', 'bar', entries);
        expect(entries.foo).toEqual('bar');
        expect(scope.editing).toEqual(false);
      });
    });
});

describe('KeyValuesController', function(){
  var scope, controller;

  beforeEach(function(){
    scope = {
      entries: { 'foo': 'bar'},
      form: {
        $setPristine: function(){},
        $setUntouched: function(){},
        $setValidity: function(){},
      },
      readonlyKeys: ''
    };
    inject(function(_$controller_){
      // The injector unwraps the underscores (_) from around the parameter names when matching
     controller = _$controller_('KeyValuesController', {$scope: scope});
    });

  });

  describe('#allowDelete', function(){

    it('should when the deletePolicy equals always', function(){
      expect(scope.allowDelete('foo')).toBe(true);
    });

    it('should not when the deletePolicy equals never', function(){
      scope.deletePolicy = 'never';
      expect(scope.allowDelete('foo')).toBe(false);
    });

    it('should when the deletePolicy equals added and the entry was not originally in entries', function(){
      scope.deletePolicy = 'added';
      scope.key = 'abc';
      scope.value = 'def';
      scope.addEntry();
      expect(scope.allowDelete('abc')).toBe(true);
    });

    it('should not when the deletePolicy equals added and the entry was originally in entries', function(){
      scope.deletePolicy = 'added';
      expect(scope.allowDelete('foo')).toBe(false);
    });

  });

  describe('#addEntry', function(){
    it('should not add the entry if the key is in the readonly list', function(){
      scope.readonlyKeys = 'abc';
      scope.key = 'abc';
      scope.value = 'xyz';
      scope.addEntry();
      expect(scope.entries.abc).toBe(undefined);
      expect(scope.key).toEqual('abc');
      expect(scope.value).toEqual('xyz');
    });

    it('should add the key/value to the scope', function(){
      scope.key = 'foo';
      scope.value = 'bar';
      scope.addEntry();
      scope.key = 'abc';
      scope.value = 'def';
      scope.addEntry();
      expect(scope.entries.foo).toEqual('bar');
      expect(scope.entries.abc).toEqual('def');
      expect(scope.key).toEqual(null);
      expect(scope.value).toEqual(null);
    });

  });

  describe('#deleteEntry', function(){
    //TODO add test for nonrecognized key?

    it('should delete the key/value from the scope', function(){
      scope.deleteEntry('foo');
      expect(scope.entries.foo).toBe(undefined);
    });
  });
});


