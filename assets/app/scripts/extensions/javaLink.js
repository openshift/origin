'use strict';

/**
 * @ngdoc function
 * @name openshiftConsoleExtensions.extension:JavaLinkController
 * @description
 * # ProjectController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsoleExtensions', ['openshiftConsole'])
  .factory("ProxyPod", function(DataService) {
    return function(namespace, podName, port) {
      if (port) {
        podName = podName + ':' + port;
      }
      return new URI(DataService.url({
        resource: 'pods/proxy',
        name: podName,
        namespace: namespace
      }));
    };
  })
  .run(function(ProxyPod, BaseHref, HawtioExtension, $templateCache, $compile, AuthService) {
    var template =
      '<span class="connect-link" ng-show="jolokiaUrl" title="Connect to container">' +
      '  <a ng-click="gotoContainerView($event, container, jolokiaUrl)" ng-href="jolokiaUrl">' +
      '    <i class="fa fa-sign-in"></i>Connect' +
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
      if (!pod || !pod.status || pod.status.phase !== 'Running') {
        return;
      }
      var podName = pod.metadata.name;
      var namespace = pod.metadata.namespace;
      $scope.jolokiaUrl = ProxyPod(namespace, podName, jolokiaPort.containerPort).segment('jolokia/').toString();
      $scope.gotoContainerView = function($event, container, jolokiaUrl) {
        $event.preventDefault();
        $event.stopPropagation();
        var returnTo = window.location.href;
        var title = container.name || 'Untitled Container';
        var token = AuthService.UserStore().getToken() || '';
        var targetURI = new URI().path(BaseHref)
                                 .segment('java/') // Must have a trailing slash to avoid runtime errors in Safari
                                 .hash(token)
                                 .query({
                                   jolokiaUrl: jolokiaUrl,
                                   title: title,
                                   returnTo: returnTo
                                 });
        window.location.href = targetURI.toString();
      };
      console.log("added");
      var answer = $compile(template)($scope);
      return answer;
    });
  });

hawtioPluginLoader.addModule('openshiftConsoleExtensions');
