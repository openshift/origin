// TODO: shared with helpers.js
var waitForUri = function(uri) {
  browser.wait(function() {
    return browser.getCurrentUrl().then(function(url) {
      return url.indexOf(uri) > -1;
    });
  }, 5000, "URL hasn't changed to " + uri);
};

exports.goTo = function(uri) {
  browser.get(uri).then(function() {
    waitForUri(uri);
  });
};

exports.presenceOf = function(obj) {
  return protractor.ExpectedConditions.presenceOf(obj);
};

// example:
//  h.waitFor(h.presenceOf(page.header()))
exports.waitFor = function(item, timeout, msg) {
  timeout = timeout || 5000;
  msg = msg || '';
  return browser.wait(item, timeout, msg);
};


exports.urlMatches = function(uri) {
  browser.getCurrentUrl().then(function(url) {
    return url.indexOf(uri) > -1;
  });
};
