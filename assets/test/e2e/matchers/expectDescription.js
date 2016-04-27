'use strict';

module.exports =  function(description, className) {
  expect(element(by.css(className || '.project-description')).getText()).toEqual(description);
};
