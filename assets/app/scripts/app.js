'use strict';

/**
 * @ngdoc overview
 * @name openshiftConsole
 * @description
 * # openshiftConsole
 *
 * Main module of the application.
 */
angular
  .module('openshiftConsole', [
    'ngAnimate',
    'ngCookies',
    'ngResource',
    'ngRoute',
    'ngSanitize',
    'ngTouch'
  ])
  .config(function ($routeProvider) {
    $routeProvider
      .when('/', {
        templateUrl: 'views/main.html',
        controller: 'MainCtrl'
      })
      .when('/about', {
        templateUrl: 'views/about.html',
        controller: 'AboutCtrl'
      })
      .when('/pods', {
        templateUrl: 'views/pods.html',
        controller: 'PodsController'
      })
      .when('/pods/:pod', {
        templateUrl: 'views/pod.html',
        controller: 'PodController'
      })
      .when('/minions', {
        templateUrl: 'views/minions.html',
        controller: 'MinionsController'
      })
      .otherwise({
        redirectTo: '/'
      });
  });
