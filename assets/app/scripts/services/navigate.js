"use strict";

angular.module("openshiftConsole")
  .service("Navigate", function($location, $window, $timeout, annotationFilter, LabelFilter){
    return {
      /**
       * Navigate and display the error page.
       *
       * @param {type} message    The message to display to the user
       * @param {type} errorCode  An optional error code to display
       * @returns {undefined}
       */
      toErrorPage: function(message, errorCode, reload) {
        var redirect = URI('error').query({
          error_description: message,
          error: errorCode
        }).toString();
        if (!reload) {
          $location.url(redirect);
        }
        else {
          $window.location.href = redirect;
        }
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
       * Return the URL for the fromTemplate page for the picked template
       *
       * @param {String}      projectName  Project name
       * @param {String}      name         Template name
       * @param {String}      namespace    Namespace from which the Template should be loaded
       * @returns {String} a URL string for the fromTemplate page. If the namespace is not set
       * read the template from TemplateService.
       */
      fromTemplateURL: function(projectName, name, namespace){
        namespace = namespace || "";
        return "project/" + encodeURIComponent(projectName) + "/create/fromtemplate?name=" + name + "&namespace=" + namespace;
      },

      /**
       * Navigate and display the next steps after creation page.
       *
       * @param {type} projectName  the project name
       * @returns {undefined}
       */
      toNextSteps: function(name, projectName, searchPart){
        var search = $location.search();
        search.name = name;
        if (_.isObject(searchPart)) {
          _.extend(search, searchPart);
        }
        $location.path("project/" + encodeURIComponent(projectName) + "/create/next").search(search);
      },

      toPodsForDeployment: function(deployment) {
        $location.url("/project/" + deployment.metadata.namespace + "/browse/pods");
        $timeout(function() {
          LabelFilter.setLabelSelector(new LabelSelector(deployment.spec.selector, true));
        }, 1);
      },

      // Resource is either a resource object, or a name.  If resource is a name, kind and namespace must be specified
      // Note that builds and deployments can only have their URL built correctly (including their config in the URL)
      // if resource is an object, otherwise they will fall back to the non-nested URL.
      // TODO - if we do ever need to create a build URL without the build object but with a known build (deployment)
      // name and buildConfig (deploymentConfig) name, then we will need either a specialized method for that, or an
      // additional opts param for extra opts.
      resourceURL: function(resource, kind, namespace, action) {
        action = action || "browse";
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

        var url = "project/" + encodedNamespace + "/" + action + "/";
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
      },
      /**
       * Navigate to a list view for a resource type
       *
       * @param {String} resource      the resource (e.g., builds or replicationcontrollers)
       * @param {String} projectName   the project name
       * @returns {undefined}
       */
      toResourceList: function(resource, projectName) {
        var routeMap = {
          'builds': 'builds',
          'buildconfigs': 'builds',
          'deployments': 'deployments',
          'deploymentconfigs': 'deployments',
          'imagestreams': 'images',
          'pods': 'pods',
          'replicationcontrollers': 'deployments',
          'routes': 'routes',
          'services': 'services',
          'persistentvolumeclaims': 'storage'
        };

        var redirect = URI.expand("project/{projectName}/browse/{browsePath}", {
          projectName: projectName,
          browsePath: routeMap[resource]
        });

        $location.url(redirect);
      },
      healthCheckURL: function(projectName, kind, name) {
        return URI.expand("project/{projectName}/edit/health-checks?kind={kind}&name={name}", {
          projectName: projectName,
          kind: kind,
          name: name
        }).toString();
      }
    };
  });
