'use strict';

var win = require('./window.js');
var user = require('./user.js');


exports.setInputValue = function(name, value) {
  var input = element(by.model(name));
  expect(input).toBeTruthy();
  input.clear();
  input.sendKeys(value);
  expect(input.getAttribute("value")).toBe(value);
  return input;
};


exports.clickWhenReady = function(button, maxTime) {
  browser.wait(protractor.ExpectedConditions.elementToBeClickable(button), maxTime || 2000);
  return button.click();
};


exports.clickAndGo = function(buttonText, uri, maxTime) {
  var button = element(by.buttonText(buttonText));
  browser.wait(protractor.ExpectedConditions.elementToBeClickable(button), maxTime || 2000);
  button.click().then(function() {
    return browser.getCurrentUrl().then(function(url) {
      return url.indexOf(uri) > -1;
    });
  });
};


exports.waitForPresence = function(selector, elementText, timeout) {
  var el;
  if (elementText) {
    el = element(by.cssContainingText(selector, elementText));
  }
  else {
    el = element(by.css(selector));
  }
  browser.wait(protractor.ExpectedConditions.presenceOf(el), timeout || 5000, "Element not found: " + selector);
};
