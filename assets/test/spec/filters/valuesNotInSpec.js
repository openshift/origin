'use strict';

describe('valuesNotInFilter', function(){
  var filter;
  var entries = {
    foo: 'bar',
    abc: 'xyz',
    another: 'value'
  };
  beforeEach(function(){
    inject(function(valuesNotInFilter){
      filter = valuesNotInFilter;
    });
  });

  it('should return a subset of the entries', function(){
    var results = filter(entries,'foo,another');
    delete entries.foo;
    delete entries.another;
    expect(results).toEqual(entries);
  });
});

