'use strict';

describe('Controller: ProjectController', function () {

  // Angular is refusing to recognize the HawtioNav stuff
  // when testing even though its being loaded
   beforeEach(module(function ($provide) {
    $provide.provider("HawtioNavBuilder", function() {
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
      this.join = function() {return "";};
      this.$get = function() {return new Mocked();};
    });

    $provide.factory("HawtioNav", function(){
      return {add: function() {}};
    });
  }));

  // load the controller's module
  beforeEach(module('openshiftConsole'));

  var ProjectController,
    scope;

  // Initialize the controller and a mock scope
  beforeEach(inject(function ($controller, $rootScope) {
    scope = $rootScope.$new();
    ProjectController = $controller('ProjectController', {
      $scope: scope,
      $routeParams: {project: "foo"},
      DataService: {
        get: function(type, id, context, callback, opts) {
          // TODO return mocked project data
          callback({});
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

  it('should set the projectName', function () {
    expect(scope.projectName).toBe("foo");
  });

  it('should set up the promise and resolve it when project is returned', function () {
    expect(scope.projectPromise.state()).toBe("resolved");
  });
});
