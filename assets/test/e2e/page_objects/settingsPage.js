'use strict';

var browse = require('../helpers/browser.js');
var h = require('../helpers/helpers.js');


exports.visit = function(project) {
  return browse.goTo('/project/' + project.name + '/settings');
};

// Delete Project Methods ------------------------------------------------------
exports.openMenu = function() {
  return element(by.css('.actions-dropdown-btn')).click();
};

exports.deleteProject = function() {
  element(by.css('.button-delete')).click();
  // OR could return the modal:
  // at present, the modal is an unnecessary detail to know about in the tests,
  // separate functions works fine.
  // return {
  //     delete: function() {
  //       element(by.cssContainingText(".modal-dialog .btn", "Delete")).click();
  //     },
  //     cancel: function() {
  //       // do cancel
  //     }
  // };
};

// must deleteProject() first, else the modal is not visible.
exports.confirmProjectDelete = function() {
  // NOTE: modal could be a separate internal object if needed..
  // cssContainingText is ugly.  We should have better hooks, this is likely what
  // QE is requesting as well.
  element(by.cssContainingText(".modal-dialog .btn", "Delete")).click();
};

exports.expectSuccessfulDelete = function() {
  h.waitForPresence(".alert-success", "marked for deletion");
};
