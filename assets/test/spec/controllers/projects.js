'use strict';

describe('Controller: ProjectsController', function () {

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
