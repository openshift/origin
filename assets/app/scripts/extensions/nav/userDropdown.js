'use strict';

angular.module('openshiftConsole')
  .run(function(extensionRegistry) {
    extensionRegistry
      .add('nav-user-dropdown', function() {
        return [{
          type: 'dom',
          node: '<li><a href="logout">Log out</a></li>'
        }];
      });
  });
