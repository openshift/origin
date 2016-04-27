'use strict';

var Project = function() {
  var timestamp = (new Date()).getTime();
  this.name = 'console-test-project-' + timestamp;
  this.displayName = 'Console integration test Project ' + timestamp;
  this.description = 'Created by assets/test/integration/rest-api/project.js';
};

// var project = new Project();
module.exports = Project;
