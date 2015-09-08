"use strict";

angular.module("openshiftConsole")

  .service("ApplicationGenerator", function(DataService){
    var oApiVersion = DataService.oApiVersion;
    var k8sApiVersion = DataService.k8sApiVersion;

    var scope = {};

    scope._generateSecret = function(){
        //http://stackoverflow.com/questions/105034/create-guid-uuid-in-javascript
        function s4() {
          return Math.floor((1 + Math.random()) * 0x10000)
              .toString(16)
              .substring(1);
          }
        return s4()+s4()+s4()+s4();
      };

    /**
    * Find the 'first' port of exposed ports.
    * @param            ports  list of ports (e.g {containerPort: 80, protocol: "tcp"})
    * @return {integer} The port/protocol pair of the lowest container port
    */
    scope._getFirstPort = function(ports){
      var first = "None";
      ports.forEach(function(port){
        if(first === "None"){
            first = port;
          }else{
            if(port.containerPort < first.containerPort){
              first = port;
            }
          }
        }
      );
      return first;
    };

    /**
     * Generate resource definitions to support the given input
     * @param {type} input
     * @returns Hash of resource definitions
     */
    scope.generate = function(input){
      //map ports to k8s structure
      var parsePorts = function(portSpec){
        var ports = [];
        angular.forEach(portSpec, function(value, key){
          var parts = key.split("/");
          if(parts.length === 1){
            parts.push("tcp");
          }

          ports.push(
            {
              containerPort: parseInt(parts[0]),
              protocol: parts[1].toUpperCase()
            });
        });
        return ports;
      };
      var ports = input.image.dockerImageMetadata.Config ? parsePorts(input.image.dockerImageMetadata.Config.ExposedPorts) : [];
      if(ports.length === 0 && input.image.dockerImageMetadata.ContainerConfig){
        ports = parsePorts(input.image.dockerImageMetadata.ContainerConfig.ExposedPorts);
      }

      //augment labels
      input.labels.app = input.name;
      input.annotations["openshift.io/generatedby"] = "OpenShiftWebConsole";

      var imageSpec;
      if(input.buildConfig.sourceUrl !== null){
        imageSpec = {
          name: input.name,
          tag: "latest",
          kind: "ImageStreamTag",
          toString: function(){
            return this.name + ":" + this.tag;
          }
        };
      }

      var resources = {
        imageStream: scope._generateImageStream(input),
        buildConfig: scope._generateBuildConfig(input, imageSpec, input.labels),
        deploymentConfig: scope._generateDeploymentConfig(input, imageSpec, ports, input.labels),
        service: scope._generateService(input, input.name, scope._getFirstPort(ports))
      };
      resources.route = scope._generateRoute(input, input.name, resources.service.metadata.name);
      return resources;
    };

    scope._generateRoute = function(input, name, serviceName){
      if(!input.routing.include) {
        return null;
      }
      return {
        kind: "Route",
        apiVersion: oApiVersion,
        metadata: {
          name: name,
          labels: input.labels,
          annotations: input.annotations
        },
        spec: {
          to: {
            kind: "Service",
            name: serviceName
          }
        }
      };
    };

    scope._generateDeploymentConfig = function(input, imageSpec, ports){
      var env = [];
      angular.forEach(input.deploymentConfig.envVars, function(value, key){
        env.push({name: key, value: value});
      });
      var templateLabels = angular.copy(input.labels);
      templateLabels.deploymentconfig = input.name;

      var deploymentConfig = {
        apiVersion: oApiVersion,
        kind: "DeploymentConfig",
        metadata: {
          name: input.name,
          labels: input.labels,
          annotations: input.annotations
        },
        spec: {
          replicas: input.scaling.replicas,
          selector: {
            deploymentconfig: input.name
          },
          triggers: [],
          template: {
            metadata: {
              labels: templateLabels
            },
            spec: {
              containers: [
                {
                  image: imageSpec.toString(),
                  name: input.name,
                  ports: ports,
                  env: env
                }
              ]
            }
          }
        }
      };
      if(input.deploymentConfig.deployOnNewImage){
        deploymentConfig.spec.triggers.push(
          {
            type: "ImageChange",
            imageChangeParams: {
              automatic: true,
              containerNames: [
                input.name
              ],
              from: {
                kind: imageSpec.kind,
                name: imageSpec.toString()
              }
            }
          }
        );
      }
      if(input.deploymentConfig.deployOnConfigChange){
        deploymentConfig.spec.triggers.push({type: "ConfigChange"});
      }
      return deploymentConfig;
    };

    scope._generateBuildConfig = function(input, imageSpec){
      var triggers = [
        {
          generic: {
            secret: scope._generateSecret()
          },
          type: "Generic"
        },
        {
          type: "ConfigChange"
        }
      ];
      if(input.buildConfig.buildOnSourceChange){
        triggers.push({
            github: {
              secret: scope._generateSecret()
            },
            type: "GitHub"
          }
        );
      }
      if(input.buildConfig.buildOnImageChange){
        triggers.push({
          imageChange: {},
          type: "ImageChange"
        });
      }

      // User can input a URL that contains a ref
      var uri = new URI(input.buildConfig.sourceUrl);
      var sourceRef = uri.fragment();
      if (!sourceRef || sourceRef.length === 0) {
        sourceRef = "master";
      }
      uri.fragment("");
      var sourceUrl = uri.href();

      return {
        apiVersion: oApiVersion,
        kind: "BuildConfig",
        metadata: {
          name: input.name,
          labels: input.labels,
          annotations: input.annotations
        },
        spec: {
          output: {
            to: {
              name: imageSpec.toString(),
              kind: imageSpec.kind
            }
          },
          source: {
            git: {
              ref: sourceRef,
              uri: sourceUrl
            },
            type: "Git"
          },
          strategy: {
            type: "Source",
            sourceStrategy: {
              from: {
                kind: "ImageStreamTag",
                name: input.imageName + ":" + input.imageTag,
                namespace: input.namespace
              }
            }
          },
          triggers: triggers
        }
      };
    };

    scope._generateImageStream = function(input){
      return {
        apiVersion: oApiVersion,
        kind: "ImageStream",
        metadata: {
          name: input.name,
          labels: input.labels,
          annotations: input.annotations
        }
      };
    };

    scope._generateService  = function(input, serviceName, port){
      if(port === 'None') {
        return null;
      }
      var service = {
        kind: "Service",
        apiVersion: k8sApiVersion,
        metadata: {
          name: serviceName,
          labels: input.labels,
          annotations: input.annotations
        },
        spec: {
          selector: {
            deploymentconfig: input.name
          }
        }
      };
      //TODO add in when server supports headless services without a port spec
//      if(port === 'None'){
//        service.spec.clusterIP = 'None';
//      }else{
        service.spec.ports = [{
          port: port.containerPort,
          targetPort: port.containerPort,
          protocol: port.protocol
        }];
//      }
      return service;
    };

    return scope;
  }
);
