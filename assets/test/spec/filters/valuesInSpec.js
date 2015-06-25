'use strict';

describe('valuesInFilter', function(){
  var filter;
  var entries = {
    foo: 'bar',
    abc: 'xyz',
    another: 'value'
  };
  beforeEach(function(){
    inject(function(valuesInFilter){
      filter = valuesInFilter;
    });
  });

  it('should return a subset of the entries', function(){
    var results = filter(entries,'foo,another');
    delete entries.abc;
    expect(results).toEqual(entries);
  });
});

