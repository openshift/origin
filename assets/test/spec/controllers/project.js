'use strict';

describe('Controller: ProjectController', function () {

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
        getObject: function(type, id, callback, context, opts) {
          // TODO return mocked project data
          callback({});
        }
      }
    });
  }));

  it('should set the projectName', function () {
    expect(scope.projectName).toBe("foo");
  });

  it('should set up the promise and resolve it when project is returned', function () {
    expect(scope.project.state()).toBe("resolved");
  });
});
