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
        templateUrl: 'views/projects.html',
        controller: 'ProjectsController'
      })
      .when('/project/:project', {
        templateUrl: 'views/project.html',
        controller: 'ProjectController'
      })      
      .when('/pods', {
        templateUrl: 'views/pods.html',
        controller: 'PodsController'
      })
      .otherwise({
        redirectTo: '/'
      });
  });
