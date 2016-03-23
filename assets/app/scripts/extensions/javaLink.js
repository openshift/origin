'use strict';

angular
  .module('openshiftConsoleExtensions')
  .run([
    'AuthService',
    'BaseHref',
    'DataService',
    'extensionRegistry',
    function(AuthService, BaseHref, DataService, extensionRegistry) {

      var template = [
        '<div row ',
        'ng-show="item.url" ',
        'class="icon-row" ',
        'title="Connect to container">',
          '<div>',
            '<i class="fa fa-share" aria-hidden="true"></i>',
          '</div>',
          '<div flex>',
            '<a ng-click="item.onClick($event)" ',
              'ng-href="item.url">',
              'Open Java Console',
            '</a>',
          '</div>',
        '</div>'
      ].join('');

      var makeJolokiaUrl = function(namespace, podName, port) {
        return new URI(DataService.url({
                    resource: 'pods/proxy',
                    // always use https when connecting to jolokia in a pod
                    name: ['https', podName, port || ''].join(':'),
                    namespace: namespace
                  }))
                  .segment('jolokia/');
      };

      extensionRegistry.add('container-links', _.spread(function(container, pod) {
        var jolokiaPort = _.find((container.ports || []), function(port) {
          return port.name && port.name.toLowerCase() === 'jolokia';
        });
        if (!jolokiaPort) {
          return;
        }
        if(_.get(pod, 'status.phase') !== 'Running') {
          return;
        }
        var podName = pod.metadata.name;
        var namespace = pod.metadata.namespace;
        var jolokiaUrl = makeJolokiaUrl(namespace, podName, jolokiaPort.containerPort).toString();
        var gotoContainerView = function($event) {
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

        return {
          type: 'dom',
          node: template,
          onClick: gotoContainerView,
          url: jolokiaUrl
        };

      }));
    }
  ]);
