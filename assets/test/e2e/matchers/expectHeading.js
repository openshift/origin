'use strict';

module.exports = function(text, heading) {
  expect(element(by.cssContainingText(heading || 'h1', text)).isPresent()).toBe(true);
};
