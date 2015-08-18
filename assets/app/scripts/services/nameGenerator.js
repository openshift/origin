"use strict";

angular.module("openshiftConsole")
  .service("NameGenerator", function(){
    return {

      /**
       * Get a name suggestion for resources based on the the source URL
       * 
       * @param {String} sourceUrl  the sourceURL
       * @param {Array} kinds  the kinds of resources to check
       * @param {String} the namespace to use when querying for existence of a resource
       * @returns {String} a suggested name
       */
      suggestFromSourceUrl: function(sourceUrl){
        var projectName = sourceUrl.substr(sourceUrl.lastIndexOf("/")+1, sourceUrl.length);
        var index = projectName.indexOf(".");
        if(index !== -1){
          projectName = projectName.substr(0,index);
        }
        // User can submit a URL with a ref on the end, dont include that in the name suggestion
        return projectName.split("#")[0];
      }
    };
  });

