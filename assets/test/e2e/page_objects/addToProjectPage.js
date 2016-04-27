'use strict';

var browse = require('../helpers/browser.js');
var h = require('../helpers/helpers.js');

exports.uri = function(project) {
  return '/project/' + project.name + '/create';
};

exports.visit = function(project) {
  return browse.goTo(exports.uri(project));
};

exports.expectHeading = function(text) {
  expect(element(by.cssContainingText('h1', text)).isPresent()).toBe(true);
};

exports.expectFromSourceUrl = function() {
  expect(element(by.model('from_source_url')).isPresent()).toBe(true);
};

exports.expectTemplate = function(name) {
  expect(element(by.cssContainingText('.catalog h3 > a', name)).isPresent()).toBe(true);
};

exports.expectBuilderImage = function(name) {
  expect(element(by.cssContainingText('.catalog h3 > a', name)).isPresent()).toBe(true);
};

exports.setFromSource = function(sourceUrl) {
  h.setInputValue('from_source_url', sourceUrl);
};

exports.next = function() {
  var nextButton = element(by.buttonText('Next'));
  browser.wait(protractor.ExpectedConditions.elementToBeClickable(nextButton), 2000);
  nextButton.click();
};
