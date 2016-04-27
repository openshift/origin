'use strict';

// by.model is the ng-model attribute
module.exports = function(model) {
  expect(element(by.model(model)).isPresent()).toBe(true);
};
