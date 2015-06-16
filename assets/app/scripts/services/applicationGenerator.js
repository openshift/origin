"use strict";

angular.module("openshiftConsole")

  .service("ApplicationGenerator", function(DataService){
    var osApiVersion = DataService.osApiVersion;
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
              name: input.name + "-" + parts[1] + "-" + parts[0],
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
      input.labels.name = input.name;
      input.labels.generatedby = "OpenShiftWebConsole";

      var imageSpec;
      if(input.buildConfig.sourceUrl !== null){
        imageSpec = {
          name: input.name, 
          tag: "latest",
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
      if(!input.routing.include) return null;
      return {
        kind: "Route",
        apiVersion: osApiVersion,
        metadata: {
          name: name,
          labels: input.labels
        },
        spec: {
          to: {
            kind: "Service",
            name: serviceName
          }
        }
      };
    };

    scope._generateDeploymentConfig = function(input, imageSpec, ports, labels){
      var env = [];
      angular.forEach(input.deploymentConfig.envVars, function(value, key){
        env.push({name: key, value: value});
      });
      labels = angular.copy(labels);
      labels.deploymentconfig = input.name;

      var deploymentConfig = {
        apiVersion: osApiVersion,
        kind: "DeploymentConfig",
        metadata: {
          name: input.name,
          labels: labels
        },
        spec: {
          strategy: {
              type: "Recreate"
          },
          replicas: input.scaling.replicas,
          selector: {
            deploymentconfig: input.name
          },
          triggers: [],
          template: {
            metadata: {
              labels: labels
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
                kind: "ImageStreamTag",
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

    scope._generateBuildConfig = function(input, imageSpec, labels){
      var triggers = [
        {
          generic: {
            secret: scope._generateSecret()
          },
          type: "generic"
        }
      ];
      if(input.buildConfig.buildOnSourceChange){
        triggers.push({
            github: {
              secret: scope._generateSecret()
            },
            type: "github"
          }
        );
      }
      if(input.buildConfig.buildOnImageChange){
        triggers.push({
          imageChange: {},
          type: "imageChange"
        });
      }
      return {
        apiVersion: osApiVersion,
        kind: "BuildConfig",
        metadata: {
          name: input.name,
          labels: labels
        },
        spec: {
          output: {
            to: {
              name: imageSpec.name
            }
          },
          source: {
            git: {
              ref: "master",
              uri: input.buildConfig.sourceUrl
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
        apiVersion: osApiVersion,
        kind: "ImageStream",
        metadata: {
          name: input.name,
          labels: input.labels
        }
      };
    };

    scope._generateService  = function(input, serviceName, port){
      if(port === 'None') return null;
      var service = {
        kind: "Service",
        apiVersion: k8sApiVersion,
        metadata: {
          name: serviceName,
          labels: input.labels
        },
        spec: {
          selector: {
            deploymentconfig: input.name
          }
        }
      };
      //TODO add in when server supports headless services without a port spec
//      if(port === 'None'){
//        service.spec.portalIP = 'None';
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
