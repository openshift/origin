'use strict';

describe('defaultIfBlankFilter', function(){
  var defaultIfBlank;

  beforeEach(
    inject(function(defaultIfBlankFilter){
      defaultIfBlank = defaultIfBlankFilter;
    })
  );

  it('should return the value if a non-string', function(){
    expect(defaultIfBlank(1, 'foo')).toBe('1');
  });

  it('should return the value if a non empty string', function(){
    expect(defaultIfBlank('theValue', 'foo')).toBe('theValue');
  });

  it('should return the default if a null string', function(){
    expect(defaultIfBlank(null, 'foo')).toBe('foo');
  });

  it('should return the default if an empty string', function(){
    expect(defaultIfBlank('', 'foo')).toBe('foo');
  });

  it('should return the default if a blank string', function(){
    expect(defaultIfBlank('  ', 'foo')).toBe('foo');
  });

});
