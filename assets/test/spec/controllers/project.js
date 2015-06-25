'use strict';
/* jshint unused: false */

describe('Controller: ProjectController', function () {

  // Angular is refusing to recognize the HawtioNav stuff
  // when testing even though its being loaded
   beforeEach(module(function ($provide) {
    $provide.provider('HawtioNavBuilder', function() {
      function Mocked() {}
      this.create = function() {return this;};
      this.id = function() {return this;};
      this.title = function() {return this;};
      this.template = function() {return this;};
      this.isSelected = function() {return this;};
      this.href = function() {return this;};
      this.page = function() {return this;};
      this.subPath = function() {return this;};
      this.build = function() {return this;};
      this.join = function() {return '';};
      this.$get = function() {return new Mocked();};
    });

    $provide.factory('HawtioNav', function(){
      return {add: function() {}};
    });
  }));

  // Make sure a base location exists in the generated test html
  if (!$('head base').length) {
    $('head').append($('<base href="/">'));
  }

  angular.module('openshiftConsole').config(function(AuthServiceProvider) {
    AuthServiceProvider.UserStore('MemoryUserStore');
  });

  // load the controller's module
  beforeEach(module('openshiftConsole'));

  var ProjectController,
    scope,
    timeout;

  // Initialize the controller and a mock scope
  beforeEach(inject(function ($controller, $rootScope, $q, $timeout, MemoryUserStore) {
    // Set up a stub user
    MemoryUserStore.setToken('myToken');
    MemoryUserStore.setUser({metadata: {name: 'My User'}});

    scope = $rootScope.$new();
    timeout = $timeout;

    ProjectController = $controller('ProjectController', {
      $scope: scope,
      $routeParams: {project: 'foo'},
      DataService: {
        get: function(type, id, context, opts) {
          // TODO return mocked project data
          return $q.when({});
        },
        list: function() {
          // TODO return mocked data for different types
        },
        watch: function() {
          // TODO return mocked data for different types
        }
      }
    });
  }));

  it('should set the user', function () {
    // Flush async withUser and DataService calls
    timeout.flush();
    expect(scope.user).toBeDefined();
    expect(scope.user).not.toBe(null);
  });

  it('should set the projectName', function () {
    // Flush async withUser and DataService calls
    timeout.flush();
    expect(scope.projectName).toBe('foo');
  });

  it('should set up the promise and resolve it when project is returned', function () {
    // Flush async withUser and DataService calls
    timeout.flush();
    expect(scope.projectPromise.state()).toBe('resolved');
  });
});
