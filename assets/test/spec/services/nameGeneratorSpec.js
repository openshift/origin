"use strict";

describe("NameGenerator", function(){
  var NameGenerator;

  beforeEach(function(){

    inject(function(_NameGenerator_){
      NameGenerator = _NameGenerator_;
    });
  });

  describe("#suggestFromSourceUrl", function(){

    var sourceUrl = "git@github.com:openshift/ruby-hello-world.git";

    it("should suggest a name based on git source url ending with 'git'", function(){
      var result = NameGenerator.suggestFromSourceUrl(sourceUrl);
      expect(result).toEqual("ruby-hello-world");
    });

    it("should suggest a name based on git source url not ending with 'git'", function(){

      sourceUrl = "git@github.com:openshift/ruby-hello-world";
      var result = NameGenerator.suggestFromSourceUrl(sourceUrl);
      expect(result).toEqual("ruby-hello-world");
    });

  });

});
