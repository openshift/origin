'use strict';

module.exports = function(name, className) {
  expect(element(by.css(className || '.project-name')).getText()).toEqual(name);
};
