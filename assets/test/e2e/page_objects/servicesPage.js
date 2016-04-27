'use strict';

var browse = require('../helpers/browser.js');

exports.visit = function(project) {
  return browse.goTo('/project/' + project.name + '/browse/services');
};
