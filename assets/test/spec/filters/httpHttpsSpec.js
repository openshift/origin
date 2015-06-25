'use strict';

describe('httpHttpsFilter', function(){

  it('should return https:// when true', function(){
    inject(function(httpHttpsFilter){
      expect(httpHttpsFilter(true)).toBe('https://');
    });
  });

  it('should return http:// when false', function(){
    inject(function(httpHttpsFilter){
      expect(httpHttpsFilter(false)).toBe('http://');
    });
  });
});


