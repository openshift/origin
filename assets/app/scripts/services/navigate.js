"use strict";

angular.module("openshiftConsole")
  .service("Navigate", function($location){
    return {
      /**
       * Navigate and display the error page.
       * 
       * @param {type} message    The message to display to the user
       * @param {type} errorCode  An optional error code to display
       * @returns {undefined}
       */
      toErrorPage: function(message, errorCode) {
        var redirect = URI('error').query({
          error_description: message,
          error: errorCode
        }).toString();
        $location.url(redirect);
      },

      /**
       * Navigate and display the project overview page.
       * 
       * @param {type} projectName  the project name
       * @returns {undefined}
       */
      toProjectOverview: function(projectName){
        $location.path(this.projectOverviewURL(projectName));
      },

      /**
       * Return the URL for the project overview
       * 
       * @param {type}     projectName
       * @returns {String} a URL string for the project overview
       */
      projectOverviewURL: function(projectName){
        return "project/" + encodeURIComponent(projectName) + "/overview";
      },

      /**
       * Navigate and display the next steps after creation page.
       * 
       * @param {type} projectName  the project name
       * @returns {undefined}
       */
      toNextSteps: function(name, projectName){
        $location.path("project/" + encodeURIComponent(projectName) + "/create/next").search("name", name);
      }
    };
  });