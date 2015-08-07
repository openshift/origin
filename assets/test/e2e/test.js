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
      expect(browser.driver.getTitle()).toEqual('Login - Red Hat® OpenShift Enterprise');

      login(true);

      expect(browser.getTitle()).toEqual("OpenShift Web Console");
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

    it('should be able to show the create project page', function() {
      browser.get('/createProject');
      expect(element(by.cssContainingText("h1","New Project")).isPresent()).toBe(true);
      // TODO: attempt creating a project with a taken name
      // TODO: attempt creating a new project, ensure all three fields are honored
    });

    it('should be able to list the test project', function() {
      browser.get('/');
      expect(element(by.cssContainingText("h2.project","test")).isPresent()).toBe(true);
    });

    it('should have access to the test project', function() {
      browser.get('/project/test');

      expect(element(by.css('h1')).getText()).toEqual("Project test");

      expect(element(by.cssContainingText(".component .service","database")).isPresent()).toBe(true);

      expect(element(by.cssContainingText(".component .service","frontend")).isPresent()).toBe(true);
      expect(element(by.cssContainingText(".component .route","www.example.com")).isPresent()).toBe(true);

      expect(element(by.cssContainingText(".pod-template-build","Build: ruby-sample-build")).isPresent()).toBe(true);

      expect(element(by.cssContainingText(".deployment-trigger","new image for origin-ruby-sample:latest")).isPresent()).toBe(true);

      expect(element.all(by.css(".pod-running")).count()).toEqual(3);

      // TODO: validate correlated images, builds, source
    });

    it('should browse builds', function() {
      browser.get('/project/test/browse/builds');
      expect(element(by.css('h1')).getText()).toEqual("Builds");
      // TODO: validate presented strategies, images, repos
    });

    it('should browse deployments', function() {
      browser.get('/project/test/browse/deployments');
      expect(element(by.css('h1')).getText()).toEqual("Deployments");
      // TODO: validate presented deployments
    });

    it('should browse events', function() {
      browser.get('/project/test/browse/events');
      expect(element(by.css('h1')).getText()).toEqual("Events");
      // TODO: validate presented events
    });

    it('should browse image streams', function() {
      browser.get('/project/test/browse/images');
      expect(element(by.css('h1')).getText()).toEqual("Image Streams");
      // TODO: validate presented images
    });

    it('should browse pods', function() {
      browser.get('/project/test/browse/pods');
      expect(element(by.css('h1')).getText()).toEqual("Pods");
      // TODO: validate presented pods, containers, correlated images, builds, source
    });

    it('should browse services', function() {
      browser.get('/project/test/browse/services');
      expect(element(by.css('h1')).getText()).toEqual("Services");
      // TODO: validate presented ports, routes, selectors
    });

    it('should browse settings', function() {
      browser.get('/project/test/settings');
      expect(element(by.css('h1')).getText()).toEqual("Project Settings");
      // TODO: validate presented project info, quota and resource info
    });

    it('should view the create page', function() {
      browser.get('/project/test/create');
      expect(element(by.css('h1')).getText()).toEqual("Create Using Your Code");
      // TODO: validate presented instant apps (load some during e2e)
    });

    it('should view the create from source repo page', function() {
      browser.get('/project/test/catalog/images?builderfor=https:%2F%2Fgithub.com%2Fopenshift%2Fnodejs-ex');
      expect(element(by.css('h1')).getText()).toEqual("Select a builder image");
      // TODO: validate presented builder images (load some during e2e)
    });

    // TODO: show final page of create from source flow

    // TODO: show final page of create from template flow

  });

});
