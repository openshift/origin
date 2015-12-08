var commonTeardown = function() {
  browser.executeScript('window.sessionStorage.clear();');
  browser.executeScript('window.localStorage.clear();');
};
exports.commonTeardown = commonTeardown;

exports.commonSetup = function() {
  // Want a longer browser size since the screenshot reporter only grabs the visible window
  browser.driver.manage().window().setSize(1024, 2048);
};

exports.afterAllTeardown = function() {
  commonTeardown();
  browser.driver.sleep(1000);
};

exports.login = function(loginPageAlreadyLoaded) {
  // The login page doesn't use angular, so we have to use the underlying WebDriver instance
  var driver = browser.driver;
  if (!loginPageAlreadyLoaded) {
    browser.get('/');
    driver.wait(function() {
      return driver.isElementPresent(by.name("username"));
    }, 3000);
  }

  driver.findElement(by.name("username")).sendKeys("e2e-user");
  driver.findElement(by.name("password")).sendKeys("e2e-user");
  driver.findElement(by.css("button[type='submit']")).click();

  driver.wait(function() {
    return driver.isElementPresent(by.css(".navbar-utility .username"));
  }, 3000);
};

exports.setInputValue = function(name, value) {
  var input = element(by.model(name));
  expect(input).toBeTruthy();
  input.clear();
  input.sendKeys(value);
  expect(input.getAttribute("value")).toBe(value);
  return input;
};

exports.clickAndGo = function(buttonText, uri) {
  var button = element(by.buttonText(buttonText));
  browser.wait(protractor.ExpectedConditions.elementToBeClickable(button), 2000);
  button.click().then(function() {
    return browser.getCurrentUrl().then(function(url) {
      return url.indexOf(uri) > -1;
    });
  });
};

var waitForUri = function(uri) {
  browser.wait(function() {
    return browser.getCurrentUrl().then(function(url) {
      return url.indexOf(uri) > -1;
    });
  }, 5000, "URL hasn't changed to " + uri);
};
exports.waitForUri = waitForUri;

exports.waitForPresence = function(selector, elementText, timeout) {
  if (!timeout) { timeout = 5000; }
  var el;
  if (elementText) {
    el = element(by.cssContainingText(selector, elementText));
  }
  else {
    el = element(by.css(selector));
  }
  browser.wait(protractor.ExpectedConditions.presenceOf(el), timeout, "Element not found: " + selector);
};

exports.goToPage = function(uri) {
  browser.get(uri).then(function() {
    waitForUri(uri);
  });
};
