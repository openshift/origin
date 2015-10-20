'use strict';

angular.module('openshiftConsoleExtensions', ['openshiftConsole'])
  .factory('ApimanLink', function() {
    return {
      hasApimanConsole: false,
      href: '/'
    };
  })
  .run(['HawtioNav', 'ApimanLink', 'AuthService', 'DataService', '$rootScope', '$routeParams', '$timeout', function(nav, ApimanLink, AuthService, DataService, $rootScope, $routeParams, $timeout) {
    var $scope = $rootScope.$new();
    $scope.$routeParams = $routeParams;
    $scope.projectPromise = $.Deferred();
    
    var handle = null;

    function clearApimanLink() {
      ApimanLink.hasApimanConsole = false;
      ApimanLink.href = '/';
    }

    function discoverApiman(projectName) {
      var last = $scope.projectName;
      $scope.projectName = projectName;
      if ($scope.projectName === last) {
        return;
      }
      if (handle) {
        DataService.unwatch(handle);
        handle = null;
      }
      if ($scope.projectName) {
        handle = DataService.watch('routes', $scope, function(data) {
          var routes = data._data;
          if (!routes || !('apiman' in routes)) {
            clearApimanLink();
            return;
          }
          var apimanRoute = routes.apiman;
          var args = {
            backTo: window.location.href,
            token: AuthService.UserStore().getToken() || ''
          };
          ApimanLink.href = new URI()
          .host(apimanRoute.spec.host)
          .scheme('http')
          .path('/apimanui/index.html')
          .hash(URI.encode(angular.toJson(args)))
          .toString();
          ApimanLink.hasApimanConsole = true;
        });
        $scope.projectPromise.resolve({
          metadata: {
            name: $scope.projectName
          }
        });
      } else {
        clearApimanLink();
      }
    }

    $scope.$watch('$routeParams.project', discoverApiman);

    nav.add({
      id: 'apiman-link',
      icon: 'wrench',
      title: function() { return 'Manage APIs'; },
      // avoid having query params added to our link
      oldHref: function() { return ''; },
      href: function() { return ApimanLink.href; },
      isValid: function() { return ApimanLink.hasApimanConsole; },
      template: function() { return '<sidebar-nav-item></sidebar-nav-item>'; }
    });

    // trigger discovery since $routeChangeSuccess doesn't happen if you
    // click refresh
    $timeout(function() {
      $rootScope.$broadcast('discoverApiman');
    }, 50);
  }]);
