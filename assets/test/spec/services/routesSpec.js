"use strict";

describe("RoutesService", function(){
  var RoutesService;

  beforeEach(function(){
    inject(function(_RoutesService_){
      RoutesService = _RoutesService_;
    });
  });

  describe("#getRouteWarnings", function(){
    var routeTemplate = {
      kind: "Route",
      apiVersion: 'v1',
      metadata: {
        name: "ruby-hello-world",
        labels : {
          "app": "ruby-hello-world"
        },
        annotations: {
          "openshift.io/generated-by": "OpenShiftWebConsole"
        }
      },
      spec: {
        to: {
          kind: "Service",
          name: "frontend"
        },
        host: "www.example.com"
      },
      status: {
        ingress: null
      }
    };

    var serviceTemplate = {
      kind: "Service",
      apiVersion: "v1",
      metadata: {
        name: "frontend",
        labels : {
          app : "ruby-hello-world"
        },
        annotations: {
          "openshift.io/generated-by": "OpenShiftWebConsole"
        }
      },
      spec: {
        ports: [{
          port: 8080,
          targetPort : 8080,
          protocol: "TCP",
          name: "8080-tcp"
        }],
        selector: {
          "deploymentconfig": "ruby-hello-world"
        }
      }
    };

    it("should warn if service has been deleted", function() {
      var warnings = RoutesService.getRouteWarnings(routeTemplate, null);
      expect(warnings).toEqual(['Routes to service "frontend", but service does not exist.']);
    });

    it("should warn if route has no target port for multi-port service", function() {
      var route = angular.copy(routeTemplate);
      var service = angular.copy(serviceTemplate);
      service.spec.ports.push({
        port: 8081,
        targetPort: 8081,
        protocol: "TCP",
        name: "8081-tcp"
      });
      var warnings = RoutesService.getRouteWarnings(route, service);
      expect(warnings).toEqual(['Route has no target port, but service "frontend" has multiple ports. ' +
        'The route will round robin traffic across all exposed ports on the service.']);
    });

    it("should warn if route has named target port not in service", function() {
      var route = angular.copy(routeTemplate);
      route.spec.port = {
        targetPort: "http"
      };
      var warnings = RoutesService.getRouteWarnings(route, serviceTemplate);
      expect(warnings).toEqual(['Route target port is set to "http", but service "frontend" has no port with that name.']);
    });

    it("should warn if route has target port number that's not a service target port", function() {
      var route = angular.copy(routeTemplate);
      route.spec.port = {
        targetPort: 80
      };
      var warnings = RoutesService.getRouteWarnings(route, serviceTemplate);
      expect(warnings).toEqual(['Route target port is set to "80", but service "frontend" does not expose that port.']);
    });

    it("should warn if route has TLS configuration, but no termination type", function() {
      var route = angular.copy(routeTemplate);
      route.spec.tls = {
        certificate: "dummy-cert",
        key: "dummy-key"
      };
      var warnings = RoutesService.getRouteWarnings(route, serviceTemplate);
      expect(warnings).toEqual(['Route has a TLS configuration, but no TLS termination type is specified. TLS will not be enabled until a termination type is set.']);
    });

    it("should warn if route uses passthrough termination with a path", function() {
      var route = angular.copy(routeTemplate);
      route.spec.path = '/test';
      route.spec.tls = {
        termination: 'passthrough'
      };
      var warnings = RoutesService.getRouteWarnings(route, serviceTemplate);
      expect(warnings).toEqual(['Route path "/test" will be ignored since the route uses passthrough termination.']);
    });

    it("should warn if route has an ingress that has not been admitted", function() {
      var route = angular.copy(routeTemplate);
      route.spec.path = '/test';
      route.status.ingress = [{
        host: 'www.example.com',
        routerName: 'foo',
        conditions: [{
          type: "Admitted",
          status: "False",
          lastTransitionTime: "2016-02-17T17:18:51Z",
          reason: "HostAlreadyClaimed",
          message: "route bar already exposes www.example.com and is older"
        }]
      }];
      var warnings = RoutesService.getRouteWarnings(route, serviceTemplate);
      expect(warnings).toEqual(["Requested host www.example.com was rejected by the router. Reason: route bar already exposes www.example.com and is older."]);
    });    

    it("should not warn if there are no problems", function() {
      var route = angular.copy(routeTemplate);
      route.spec.port = {
        targetPort: "8080-tcp"
      };
      route.spec.tls = {
        termination: "edge"
      };
      var service = angular.copy(serviceTemplate);
      service.spec.ports.push({
        port: 8081,
        targetPort: 8081,
        protocol: "TCP",
        name: "8081-tcp"
      });
      var warnings = RoutesService.getRouteWarnings(route, serviceTemplate);
      expect(warnings.length).toEqual(0);
    });

    it("should warn about multiple problems", function() {
      var route = angular.copy(routeTemplate);
      // Missing TLS termination.
      route.spec.tls = {
        certificate: "dummy-cert",
        key: "dummy-key"
      };

      var service = angular.copy(serviceTemplate);
      // Missing target port for multi-port service.
      service.spec.ports.push({
        port: 8081,
        targetPort: 8081,
        protocol: "TCP",
        name: "8081-tcp"
      });

      var warnings = RoutesService.getRouteWarnings(route, service);
      expect(warnings.length).toEqual(2);
    });
  });

  describe("#getPreferredDisplayRoute", function() {
    var routeTemplate = {
      kind: "Route",
      apiVersion: 'v1',
      metadata: {
        name: "ruby-hello-world",
        labels : {
          "app": "ruby-hello-world"
        },
        annotations: {
          "openshift.io/generated-by": "OpenShiftWebConsole",
          "openshift.io/host.generated": "true"
        }
      },
      spec: {
        to: {
          kind: "Service",
          name: "frontend"
        },
        host: "example.com"
      },
      status: {
        ingress: null
      }
    };

    it("should prefer an admitted route", function() {
      var customHost = angular.copy(routeTemplate);
      delete customHost.metadata.annotations["openshift.io/host.generated"];
      customHost.spec.tls = {
        termination: "edge"
      };

      var admitted = angular.copy(routeTemplate);
      admitted.status.ingress = [{
        host: "example.com",
        routerName: "router",
        conditions: [{
          type: "Admitted",
          status: "True",
          lastTransitionTime: '2016-03-01T14:15:05Z'
        }]
      }];

      var preferred = RoutesService.getPreferredDisplayRoute(customHost, admitted);
      expect(preferred).toEqual(admitted);
    });

    it("should prefer a custom route", function() {
      var customHost = angular.copy(routeTemplate);
      delete customHost.metadata.annotations["openshift.io/host.generated"];

      var secure = angular.copy(routeTemplate);
      secure.spec.tls = {
        termination: "edge"
      };

      var preferred = RoutesService.getPreferredDisplayRoute(customHost, secure);
      expect(preferred).toEqual(customHost);
    });

    it("should prefer a secure route", function() {
      var vanilla = angular.copy(routeTemplate);

      var secure = angular.copy(routeTemplate);
      secure.spec.tls = {
        termination: "edge"
      };

      var preferred = RoutesService.getPreferredDisplayRoute(vanilla, secure);
      expect(preferred).toEqual(secure);
    });
  });
});
