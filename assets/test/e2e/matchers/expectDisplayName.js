'use strict';

module.exports = function(displayName, className) {
  expect(element(by.css(className || '.project-display-name')).getText()).toEqual(displayName);
};
