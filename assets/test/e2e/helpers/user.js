'use strict';

require('jasmine-beforeall');


// since all other tests depend on login,
// this suite should be able to be shared


var defaultUser = {
  name: 'e2e-user',
  pass: 'e2e-user'
};

exports.login = function(userName, pass, maxWait) {
  // The login page doesn't use angular, so we have to use the underlying WebDriver instance
  var driver = browser.driver;

  browser.get('/');
  driver.wait(function() {
    return driver.isElementPresent(by.name("username"));
  }, 3000);

  driver.findElement(by.name("username")).sendKeys(userName || defaultUser.name);
  driver.findElement(by.name("password")).sendKeys(pass || defaultUser.pass);
  driver.findElement(by.css("button[type='submit']")).click();

  driver.wait(function() {
    return driver.isElementPresent(by.css(".navbar-iconic .username"));
  }, maxWait || 5000);
};


exports.logout = function() {

};
