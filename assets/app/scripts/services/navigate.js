"use strict";

angular.module("openshiftConsole")
  .service("Navigate", function($location, annotationFilter){
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
      },

      // Resource is either a resource object, or a name.  If resource is a name, kind and namespace must be specified
      // Note that builds and deployments can only have their URL built correctly (including their config in the URL)
      // if resource is an object, otherwise they will fall back to the non-nested URL.
      // TODO - if we do ever need to create a build URL without the build object but with a known build (deployment) 
      // name and buildConfig (deploymentConfig) name, then we will need either a specialized method for that, or an
      // additional opts param for extra opts.
      resourceURL: function(resource, kind, namespace) {
        if (!resource || (!resource.metadata && (!kind || !namespace))) {
          return null;
        }

        // normalize based on the kind of args we got
        if (!kind) {
          kind = resource.kind;
        }
        if (!namespace) {
          namespace = resource.metadata.namespace; 
        }
        var encodedNamespace = encodeURIComponent(namespace);

        var name = resource;
        if (resource.metadata) {
          name = resource.metadata.name;
        }

        var encodedName = encodeURIComponent(name);

        var url = "project/" + encodedNamespace + "/browse/";
        switch(kind) {
          case "Build":
            if (resource.metadata && resource.metadata.labels && resource.metadata.labels.buildconfig) {
              url += "builds/" + encodeURIComponent(resource.metadata.labels.buildconfig) + "/" + encodedName;
            }
            else {
              url += "builds-noconfig/" + encodedName;
            }
            break;
          case "BuildConfig":
            url += "builds/" + encodedName;
            break;
          case "DeploymentConfig":
            url += "deployments/" + encodedName;
            break;
          case "ReplicationController":
            var depConfig = resource.metadata ? annotationFilter(resource, 'deploymentConfig') : null;
            if (depConfig) {
              url += "deployments/" + encodeURIComponent(depConfig) + "/" + encodedName;
            }
            else {
              url += "deployments-replicationcontrollers/" + encodedName;
            }
            break;  
          case "ImageStream":
            url += "images/" + encodedName;
            break;
          default:
            url += kind.toLowerCase() + "s/" + encodedName;
        }
        return url;
      }
    };
  });