'use strict';

describe('yesNoFilter', function(){

  it('should return Yes when true', function(){
    inject(function(yesNoFilter){
      expect(yesNoFilter(true)).toBe('Yes');
    });
  });

  it('should return No when false', function(){
    inject(function(yesNoFilter){
      expect(yesNoFilter(false)).toBe('No');
    });
  });
});


