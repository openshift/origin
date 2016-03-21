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
      return new URI(DataService.url({
        resource: 'pods/proxy',
        // always use https when connecting to jolokia in a pod
        name: ['https', podName, port || ''].join(':'),
        namespace: namespace
      }));
    };
  })
  .run(function(ProxyPod, BaseHref, HawtioExtension, $templateCache, $compile, AuthService) {

    var template = [
      '<div row ',
        'class="icon-row" ',
        'ng-show="jolokiaUrl" ',
        'title="Connect to container">',
        '<div>',
          '<i class="fa fa-share" aria-hidden="true"></i>',
        '</div>',
        '<div flex>',
          '<a ng-click="gotoContainerView($event, container, jolokiaUrl)" ng-href="jolokiaUrl">',
            'Open Java Console',
          '</a>',
        '</div>',
      '</div>'
    ].join('');

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
      var containerStatuses = pod.status.containerStatuses;
      var containerStatus = _.find(containerStatuses, function(status) {
        return status.name === container.name;
      });
      if (!containerStatus || !containerStatus.ready) {
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
      var answer = $compile(template)($scope);
      return answer;
    });
  });

hawtioPluginLoader.addModule('openshiftConsoleExtensions');
