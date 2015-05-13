require('jasmine-beforeall');

describe('', function() {
  var commonTeardown = function() {
    browser.executeScript('window.sessionStorage.clear();');
    browser.executeScript('window.localStorage.clear();');
  };

  afterAll(function(){
    // Just to be sure lets teardown at the end of EVERYTHING, and then we need to sleep to make sure it is flushed to disk
    commonTeardown();
    browser.driver.sleep(1000);
  });


  // This UI test suite expects to be run as part of hack/test-end-to-end.sh
  // It requires the example project be created with all of its resources in order to pass

  var commonSetup = function() {
      // The default phantom window size is a mobile resolution, let's make it bigger
      browser.driver.manage().window().setSize(1024, 768);
  };

  var login = function(loginPageAlreadyLoaded) {
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
  };

  describe('unauthenticated user', function() {
    beforeEach(function() {
      commonSetup();
    });

    afterEach(function() {
      commonTeardown();
    });

    it('should be able to log in', function() {
      browser.get('/');
      // The login page doesn't use angular, so we have to use the underlying WebDriver instance
      var driver = browser.driver;
      driver.wait(function() {
        return driver.isElementPresent(by.name("username"));
      }, 3000);

      expect(browser.driver.getCurrentUrl()).toMatch(/\/login/);
      expect(browser.driver.getTitle()).toEqual('Login - OpenShift Origin');

      login(true);

      expect(browser.getTitle()).toEqual("OpenShift Management Console");
      expect(element(by.css(".navbar-utility .username")).getText()).toEqual("e2e-user");
    });

  });

  describe('authenticated e2e-user', function() {
    beforeEach(function() {
      commonSetup();
      login();
    });

    afterEach(function() {
      commonTeardown();
    });

    it('should be able to list the test project', function() {
      browser.get('/');
      
      expect(element(by.cssContainingText("h2.project","test")).isPresent()).toBe(true);
    });

    it('should have access to the test project', function() {
      browser.get('/project/test');

      expect(element(by.css('h1')).getText()).toEqual("Project test");
      expect(element(by.cssContainingText("h2.service","database")).isPresent()).toBe(true);
      expect(element(by.cssContainingText("h2.service","frontend")).isPresent()).toBe(true);
      expect(element(by.cssContainingText(".pod-template-image","Build: ruby-sample-build")).isPresent()).toBe(true);
      expect(element(by.cssContainingText(".deployment-trigger","new image for test/origin-ruby-sample:latest")).isPresent()).toBe(true);
      expect(element.all(by.css(".pod-running")).count()).toEqual(3);
    });
  }); 

});