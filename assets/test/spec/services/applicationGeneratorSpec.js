'use strict';

describe('ApplicationGenerator', function(){
  var ApplicationGenerator;
  var input;

  beforeEach(function(){
    module('openshiftConsole', function($provide){
      $provide.value('DataService',{
        osApiVersion: 'v1beta3',
        k8sApiVersion: 'v1beta3'
      });
    });

    inject(function(_ApplicationGenerator_){
      ApplicationGenerator = _ApplicationGenerator_;
      ApplicationGenerator._generateSecret = function(){
        return 'secret101';
      };
    });

    input = {
      name: 'ruby-hello-world',
      routing: {
        include: true
      },
      buildConfig: {
        sourceUrl: 'https://github.com/openshift/ruby-hello-world.git',
        buildOnSourceChange: true,
        buildOnImageChange: true
      },
      deploymentConfig: {
        deployOnConfigChange: true,
        deployOnNewImage: true,
        envVars: {
          'ADMIN_USERNAME' : 'adminEME',
          'ADMIN_PASSWORD' : 'xFSkebip',
          'MYSQL_ROOT_PASSWORD' : 'qX6JGmjX',
          'MYSQL_DATABASE' : 'root'
        }
      },
      labels : {
        foo: 'bar',
        abc: 'xyz'
      },
      scaling: {
        replicas: 1
      },
      imageName: 'origin-ruby-sample',
      imageTag: 'latest',
      imageRepo: {
        'kind': 'ImageStream',
        'apiVersion': 'v1beta3',
        'metadata': {
          'name': 'origin-ruby-sample',
          'namespace': 'test',
          'selfLink': '/osapi/v1beta3/test/imagestreams/origin-ruby-sample',
          'uid': 'ea1d67fc-c358-11e4-90e6-080027c5bfa9',
          'resourceVersion': '150',
          'creationTimestamp': '2015-03-05T16:58:58Z'
        },
        'spec': {
          'dockerImageRepository': 'openshift/origin-ruby-sample',
          'tags': [
            {
              'name': 'latest'
            }
          ]
        },
        'status': {
          'dockerImageRepository': 'openshift/origin-ruby-sample',
          'tags': [
            {
              'name': 'latest',
              'items': [
                {
                  'created': '2015-06-01T18:54:14Z',
                  'dockerImageReference': 'openshift/origin-ruby-sample:latest',
                  'image': 'f4589d5a40dc3ddcea72d58f51ec0a5c3915494f1f5a6b4964212120494880b6'
                }
              ]
            }
          ]
        }
      },
      image: {
        'kind' : 'Image',
        'metadata' : {
          'name' : 'ea15999fd97b2f1bafffd615697ef8c14abdfd9ab17ff4ed67cf5857fec8d6c0'
        },
        'dockerImageMetadata' : {
          'ContainerConfig' : {
            'ExposedPorts': {
              '443/tcp': {},
              '80/tcp': {}
            },
            'Env': [
              'STI_SCRIPTS_URL'
            ]
          }
        }
      }
    };
  });

  describe('#_generateService', function(){

    it('should not generate a service if no ports are exposed', function(){
      var service = ApplicationGenerator._generateService(input, 'theServiceName', 'None');
      expect(service).toEqual(null);
    });

    //TODO - add in when server supports headless services without a port spec
//    it('should generate a headless service when no ports are exposed', function(){
//      var copy = angular.copy(input);
//      copy.image.dockerImageMetadata.ContainerConfig.ExposedPorts = {};
//      var service = ApplicationGenerator._generateService(copy, 'theServiceName', 'None');
//      expect(service).toEqual(
//        {
//            'kind': 'Service',
//            'apiVersion': 'v1beta3',
//            'metadata': {
//                'name': 'theServiceName',
//                'labels' : {
//                  'foo' : 'bar',
//                  'abc' : 'xyz'                }
//            },
//            'spec': {
//                'portalIP' : 'None',
//                'selector': {
//                    'deploymentconfig': 'ruby-hello-world'
//                }
//            }
//        });
//    });
  });

  describe('#_generateRoute', function(){

    it('should generate nothing if routing is not required', function(){
      input.routing.include = false;
      expect(ApplicationGenerator._generateRoute(input, input.name, 'theServiceName')).toBe(null);
    });

    it('should generate an unsecure Route when routing is required', function(){
      var route = ApplicationGenerator._generateRoute(input, input.name, 'theServiceName');
      expect(route).toEqual({
        kind: 'Route',
        apiVersion: 'v1beta3',
        metadata: {
          name: 'ruby-hello-world',
          labels : {
            'foo' : 'bar',
            'abc' : 'xyz'
          }
        },
        spec: {
          to: {
            kind: 'Service',
            name: 'theServiceName'
          }
        }
      });
    });
  });

  describe('generating applications from image that includes source', function(){
    var resources;
    beforeEach(function(){
      resources = ApplicationGenerator.generate(input);
    });

    it('should generate a BuildConfig for the source', function(){
      expect(resources.buildConfig).toEqual(
        {
            'apiVersion': 'v1beta3',
            'kind': 'BuildConfig',
            'metadata': {
                'name': 'ruby-hello-world',
                'labels': {
                  'foo' : 'bar',
                  'abc' : 'xyz',
                  'name': 'ruby-hello-world',
                  'generatedby': 'OpenShiftWebConsole'
                }
            },
            'spec': {
                'output': {
                    'to': {
                        'name': 'ruby-hello-world'
                    }
                },
                'source': {
                    'git': {
                        'ref': 'master',
                        'uri': 'https://github.com/openshift/ruby-hello-world.git'
                    },
                    'type': 'Git'
                },
                'strategy': {
                    'type': 'Source',
                    'sourceStrategy' : {
                      'from': {
                        'kind': 'ImageStreamTag',
                        'name': 'origin-ruby-sample:latest'
                      }
                    }
                },
                'triggers': [
                    {
                        'generic': {
                            'secret': 'secret101'
                        },
                        'type': 'generic'
                    },
                    {
                        'github': {
                            'secret': 'secret101'
                        },
                        'type': 'github'
                    },
                    {
                      'imageChange' : {},
                      'type' : 'imageChange'
                    }
                ]
            }
          }
      );
    });

    it('should generate an ImageStream for the build output', function(){
      expect(resources.imageStream).toEqual(
        {
          'apiVersion': 'v1beta3',
          'kind': 'ImageStream',
          'metadata': {
              'name': 'ruby-hello-world',
              labels : {
                'foo' : 'bar',
                'abc' : 'xyz',
                'name': 'ruby-hello-world',
                'generatedby': 'OpenShiftWebConsole'
              }
          }
        }
      );
    });

    it('should generate a Service for the build output', function(){
      expect(resources.service).toEqual(
        {
            'kind': 'Service',
            'apiVersion': 'v1beta3',
            'metadata': {
                'name': 'ruby-hello-world',
                'labels' : {
                  'foo' : 'bar',
                  'abc' : 'xyz',
                  'name': 'ruby-hello-world',
                  'generatedby': 'OpenShiftWebConsole'
                }
            },
            'spec': {
                'ports': [{
                  'port': 80,
                  'targetPort' : 80,
                  'protocol': 'TCP'
                }],
                'selector': {
                    'deploymentconfig': 'ruby-hello-world'
                }
            }
        }
      );
    });

    it('should generate a DeploymentConfig for the BuildConfig output image', function(){
      var resources = ApplicationGenerator.generate(input);
      expect(resources.deploymentConfig).toEqual(
        {
          'apiVersion': 'v1beta3',
          'kind': 'DeploymentConfig',
          'metadata': {
            'name': 'ruby-hello-world',
            'labels': {
              'foo' : 'bar',
              'abc' : 'xyz',
              'name': 'ruby-hello-world',
              'generatedby' : 'OpenShiftWebConsole',
              'deploymentconfig': 'ruby-hello-world'
            }
          },
          'spec': {
            'strategy': {
              'type': 'Recreate'
            },
            'replicas': 1,
            'selector': {
              'deploymentconfig': 'ruby-hello-world'
            },
            'triggers': [
              {
                'type': 'ImageChange',
                'imageChangeParams': {
                  'automatic': true,
                  'containerNames': [
                    'ruby-hello-world'
                  ],
                  'from': {
                    'kind': 'ImageStreamTag',
                    'name': 'ruby-hello-world:latest'
                  }
                }
              },
              {
                'type': 'ConfigChange'
              }
            ],
            'template': {
              'metadata': {
                'labels': {
                  'foo' : 'bar',
                  'abc' : 'xyz',
                  'name': 'ruby-hello-world',
                  'generatedby' : 'OpenShiftWebConsole',
                  'deploymentconfig': 'ruby-hello-world'
                }
              },
              'spec': {
                'containers': [
                  {
                    'image': 'ruby-hello-world:latest',
                    'name': 'ruby-hello-world',
                    'ports': [
                      {
                        'containerPort': 443,
                        'name': 'ruby-hello-world-tcp-443',
                        'protocol': 'TCP'
                      },
                      {
                        'containerPort': 80,
                        'name': 'ruby-hello-world-tcp-80',
                        'protocol': 'TCP'
                      }
                    ],
                    'env' : [
                      {
                        'name': 'ADMIN_USERNAME',
                        'value': 'adminEME'
                      },
                      {
                        'name': 'ADMIN_PASSWORD',
                        'value': 'xFSkebip'
                      },
                      {
                        'name': 'MYSQL_ROOT_PASSWORD',
                        'value': 'qX6JGmjX'
                      },
                      {
                        'name': 'MYSQL_DATABASE',
                        'value': 'root'
                      }
                    ]
                  }
                ]
              }
            }
          }
        }
      );
    });

  });

  describe('generating service where the ports are defined on the image config block', function(){
    var resources;
    beforeEach(function(){
      input.image.dockerImageMetadata.Config = {
        'ExposedPorts': {
          '999/tcp': {},
          '777/tcp': {}
        }
      };
      resources = ApplicationGenerator.generate(input);
    });

    it('should use the first port found from the config block for the service ports', function(){
        expect(resources.service).toEqual(
        {
            'kind': 'Service',
            'apiVersion': 'v1beta3',
            'metadata': {
                'name': 'ruby-hello-world',
                'labels' : {
                  'foo' : 'bar',
                  'abc' : 'xyz',
                  'name': 'ruby-hello-world',
                  'generatedby': 'OpenShiftWebConsole'
                }
            },
            'spec': {
                'ports': [{
                  'port': 777,
                  'targetPort' : 777,
                  'protocol': 'TCP'
                }],
                'selector': {
                    'deploymentconfig': 'ruby-hello-world'
                }
            }
        }
      );
    });
  });
});
