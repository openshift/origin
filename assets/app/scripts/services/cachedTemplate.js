'use strict';

angular.module("openshiftConsole")
  .service("CachedTemplateService", function(){

    var template = null;
    return {
      setTemplate: function(temp) {
        template = temp;
      },
      getTemplate: function() {
        return template;
      },
      clearTemplate: function() {
        template = null;
      }
    };

  });
