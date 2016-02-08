'use strict';

var browse = require('../helpers/browser.js');
var h = require('../helpers/helpers.js');

exports.visit = function(project) {
  return browse.goTo('/project/' + project.name + '/browse/builds');
};
