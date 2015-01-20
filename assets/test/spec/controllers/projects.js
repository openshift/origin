'use strict';

describe('Controller: ProjectsController', function () {

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

  var ProjectsController,
    scope;

  // Initialize the controller and a mock scope
  beforeEach(inject(function ($controller, $rootScope) {
    scope = $rootScope.$new();
    ProjectsController = $controller('ProjectsController', {
      $scope: scope,
      DataService: {
        list: function(type, context, callback, opts) {
          // TODO return mocked project data
          callback({by: function(){return {}}});
        }
      }
    });
  }));

  it('should create the empty project list', function () {
    expect(scope.projects).toBeDefined();
    expect(scope.projects).not.toBe(null);
  });
});
