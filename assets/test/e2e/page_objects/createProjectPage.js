'use strict';

var browse = require('../helpers/browser.js');
var input = require('../helpers/input.js');
var h = require('../helpers/helpers.js');

exports.visit = function() {
  return browse.goTo('/create-project');
};

exports.submitProject = function(project) {
  for (var key in project) {
    input.byModel(key).setValue(project[key]);
  }
  return h.clickWhenReady(element(by.buttonText('Create')));
};
