'use strict';

/**
 * @ngdoc function
 * @name openshiftConsoleExtensions.extension:JavaLinkController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsoleExtensions', ['openshiftConsole'])
  .factory("ProxyPod", function(API_CFG) {
    return function(namespace, podId, port) {
      var kubeProxyURI = new URI().host(API_CFG.k8s.hostPort).path(API_CFG.k8s.prefix);
      var apiVersion = API_CFG.k8s.version || 'v1beta3';
      if (port) {
        podId = podId + ':' + port;
      }
      kubeProxyURI.segment(apiVersion)
                  .segment('namespaces').segment(namespace)
                  .segment('pods').segment(podId)
                  .segment('proxy');
      return kubeProxyURI;
    };
  })
  .run(function(ProxyPod, BaseHref, HawtioExtension, $templateCache, $compile, AuthService) {
    var template =
      '<span ng-show="jolokiaUrl" title="View java details">' +
      '  <a ng-click="gotoContainerView($event, container, jolokiaUrl)" ng-href="jolokiaUrl">' +
      '    <i class="fa fa-external-link"></i>' +
      '  </a>' +
      '</span>';
    HawtioExtension.add('container-links', function ($scope) {
      var container = $scope.container;
      if (!container) {
        return;
      }
      var jolokiaPort = _.find((container.ports || []), function(port) {
        return port.name && port.name.toLowerCase() === 'jolokia';
      });
      if (!jolokiaPort) {
        return;
      }
      var pod = $scope.$eval('podTemplate');
      // TODO distinguish between pod and pod templates for now...
      if (!pod || !pod.status) {
        return;
      }
      var podId = pod.metadata.name;
      var namespace = pod.metadata.namespace;
      $scope.jolokiaUrl = ProxyPod(namespace, podId, jolokiaPort.containerPort).segment('jolokia/').toString();
      $scope.gotoContainerView = function($event, container, jolokiaUrl) {
        $event.preventDefault();
        $event.stopPropagation();
        var returnTo = window.location.href;
        var title = container.name || 'Untitled Container';
        var token = AuthService.UserStore().getToken() || '';
        var targetURI = new URI().path(BaseHref)
                                 .segment('java')
                                 .segment('index.html')
                                 .hash(token)
                                 .query({
                                   jolokiaUrl: jolokiaUrl,
                                   title: title,
                                   returnTo: returnTo
                                 });
        window.location.href = targetURI.toString();
      };
      var answer = $compile(template)($scope);
      return answer;
    });
  });

hawtioPluginLoader.addModule('openshiftConsoleExtensions');
