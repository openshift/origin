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
      // Want a longer browser size since the screenshot reporter only grabs the visible window
      browser.driver.manage().window().setSize(1024, 2048);
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

    driver.wait(function() {
      return driver.isElementPresent(by.css(".navbar-utility .username"));
    }, 3000);
  };

  var setInputValue = function(name, value) {
    var input = element(by.model(name));
    expect(input).toBeTruthy();
    input.clear();
    input.sendKeys(value);
    expect(input.getAttribute("value")).toBe(value);
    return input;
  };

  var clickAndGo = function(buttonText, uri) {
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

  var waitForPresence = function(selector, elementText, timeout) {
    if (!timeout) { timeout = 5000; }
    var el = element(by.cssContainingText(selector, elementText));
    browser.wait(protractor.ExpectedConditions.presenceOf(el), timeout, "Element not found: " + selector);
  };

  var goToPage = function(uri) {
    browser.get(uri).then(function() {
      waitForUri(uri);
    });
  };

  var goToAddToProjectPage = function(projectName) {
    var uri = '/project/' + projectName + '/create';
    goToPage(uri);
    expect(element(by.cssContainingText('h1', "Create Using Your Code")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h1', "Create Using a Template")).isPresent()).toBe(true);
    expect(element(by.model('from_source_url')).isPresent()).toBe(true);
    expect(element(by.cssContainingText('.catalog h3 > a', "ruby-helloworld-sample")).isPresent()).toBe(true);
  }

  var goToCreateProjectPage = function() {
    goToPage('/createProject');
    expect(element(by.cssContainingText('h1', "New Project")).isPresent()).toBe(true);
    expect(element(by.model('name')).isPresent()).toBe(true);
    expect(element(by.model('displayName')).isPresent()).toBe(true);
    expect(element(by.model('description')).isPresent()).toBe(true);
  }

  var requestCreateFromSource = function(projectName, sourceUrl) {
    var uri = '/project/' + projectName + '/create';
    waitForUri(uri);
    setInputValue('from_source_url', sourceUrl);
    var nextButton = element(by.buttonText('Next'));
    browser.wait(protractor.ExpectedConditions.elementToBeClickable(nextButton), 2000);
    nextButton.click();
  }

  var requestCreateFromTemplate = function(projectName, templateName) {
    var uri = '/project/' + projectName + '/create';
    waitForUri(uri);
    var template = element(by.cssContainingText('.catalog h3 > a', templateName));
    expect(template.isPresent()).toBe(true);
    template.click();
  }

  var attachBuilderImageToSource = function(projectName, builderImageName) {
    var uri = '/project/' + projectName + '/catalog/images';
    waitForUri(uri);
    expect(element(by.cssContainingText('h1', "Select a builder image")).isPresent()).toBe(true);
    var builderImageLink = element(by.cssContainingText('h3 > a', builderImageName));
    expect(builderImageLink.isPresent()).toBe(true);
    builderImageLink.click();
  }

  var createFromSource = function(projectName, builderImageName, appName) {
    var uri = '/project/' + projectName + '/create/fromimage';
    waitForUri(uri);
    expect(element(by.css('.create-from-image h1')).getText()).toEqual(builderImageName);
    expect(element(by.cssContainingText('h2', "Name")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Routing")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Deployment Configuration")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Build Configuration")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Scaling")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Labels")).isPresent()).toBe(true);
    var appNameInput = element(by.name('appname'));
    appNameInput.clear();
    appNameInput.sendKeys(appName);
    clickAndGo('Create', '/project/' + projectName + '/overview');
  }

  var createFromTemplate = function(projectName, templateName, parameterNames, labelNames) {
    var uri = '/project/' + projectName + '/create/fromtemplate';
    waitForUri(uri);
    expect(element(by.css('.create-from-template h1')).getText()).toEqual(templateName);
    expect(element(by.cssContainingText('h2', "Images")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Parameters")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Labels")).isPresent()).toBe(true);
    if (parameterNames) {
      for (i = 0; i < parameterNames.length; i++) {
        expect(element(by.cssContainingText('.env-variable-list label.key', parameterNames[i])).isPresent()).toBe(true);
      }
    }
    if (labelNames) {
      for (i = 0; i < labelNames.length; i++) {
        expect(element(by.cssContainingText('.label-list span.key', labelNames[i])).isPresent()).toBe(true);
      }
    }
    clickAndGo('Create', '/project/' + projectName + '/overview');
  }

  var checkServiceCreated = function(projectName, serviceName) {
    var uri = '/project/' + projectName + '/overview';
    goToPage(uri);
    waitForPresence('.component .service', serviceName, 10000);
    var uri = '/project/' + projectName + '/browse/services';
    goToPage(uri);
    waitForPresence('h3', serviceName, 10000);
  }

  var checkProjectSettings = function(projectName, displayName, description) {
    var uri = '/project/' + projectName + '/settings';
    goToPage(uri);
    expect(element.all(by.css("dl > dd")).get(0).getText()).toEqual(projectName);
    expect(element.all(by.css("dl > dd")).get(1).getText()).toEqual(displayName);
    expect(element.all(by.css("dl > dd")).get(2).getText()).toEqual(description);
  }

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

    describe('new project', function() {
      describe('when creating a new project', function() {
        it('should be able to show the create project page', goToCreateProjectPage);

        it('should successfully create a new project', function() {
          var project = {
            name:        'console-integration-test-project',
            displayName: 'Console integration test Project',
            description: 'Created by assets/test/e2e/test.js'
          };

          goToCreateProjectPage();
          for (var key in project) {
            setInputValue(key, project[key]);
          }
          clickAndGo('Create', '/project/' + project['name'] + '/overview');
          waitForPresence('.navbar-project .bootstrap-select .selected .text', project['displayName']);
          checkProjectSettings(project['name'], project['displayName'], project['description']);
        });

        it('should validate taken name when trying to create', function() {
          goToCreateProjectPage();
          element(by.model('name')).clear().sendKeys("console-integration-test-project");
          element(by.buttonText("Create")).click();
          expect(element(by.css("[ng-if=nameTaken]")).isDisplayed()).toBe(true);
          expect(browser.getCurrentUrl()).toMatch(/\/createProject$/);
        });
      });

/*
      describe('when using console-integration-test-project', function() {
        describe('when adding to project', function() {
          it('should view the create page', function() { goToAddToProjectPage("console-integration-test-project"); });

          it('should create from source', function() {
            var projectName = "console-integration-test-project";
            var sourceUrl = "https://github.com/openshift/rails-ex#master";
            var appName = "rails-ex-mine";
            var builderImage = "ruby";

            goToAddToProjectPage(projectName);
            requestCreateFromSource(projectName, sourceUrl);
            attachBuilderImageToSource(projectName, builderImage);
            createFromSource(projectName, builderImage, appName);
            checkServiceCreated(projectName, appName);
          });

          it('should create from template', function() {
            var projectName = "console-integration-test-project";
            var templateName = "ruby-helloworld-sample";
            var parameterNames = [
              "ADMIN_USERNAME",
              "ADMIN_PASSWORD",
              "MYSQL_USER",
              "MYSQL_PASSWORD",
              "MYSQL_DATABASE"
            ];
            var labelNames = ["template"];

            goToAddToProjectPage(projectName);
            requestCreateFromTemplate(projectName, templateName);
            createFromTemplate(projectName, templateName, parameterNames, labelNames);
            checkServiceCreated(projectName, "frontend");
            checkServiceCreated(projectName, "database");
          });
        });
      });
*/
    });

    describe('with test project', function() {
      it('should be able to list the test project', function() {
        browser.get('/').then(function() {
          waitForPresence('h2.project', 'test');
        });
      });

      it('should have access to the test project', function() {
        goToPage('/project/test');
        waitForPresence('h1', 'Project test');
        waitForPresence('.component .service', 'database');
        waitForPresence('.component .service', 'frontend');
        waitForPresence('.component .route', 'www.example.com');
        waitForPresence('.pod-template-build a', '#1');
        waitForPresence('.deployment-trigger', 'from image change');
        expect(element.all(by.css(".pod-running")).count()).toEqual(3);
        // TODO: validate correlated images, builds, source
      });

      it('should browse builds', function() {
        goToPage('/project/test/browse/builds');
        waitForPresence('h1', 'Builds');
        // TODO: validate presented strategies, images, repos
      });

      it('should browse deployments', function() {
        goToPage('/project/test/browse/deployments');
        waitForPresence("h1", "Deployments");
        // TODO: validate presented deployments
      });

      it('should browse events', function() {
        goToPage('/project/test/browse/events');
        waitForPresence("h1", "Events");
        // TODO: validate presented events
      });

      it('should browse image streams', function() {
        goToPage('/project/test/browse/images');
        waitForPresence("h1", "Image Streams");
        // TODO: validate presented images
      });

      it('should browse pods', function() {
        goToPage('/project/test/browse/pods');
        waitForPresence("h1", "Pods");
        // TODO: validate presented pods, containers, correlated images, builds, source
      });

      it('should browse services', function() {
        goToPage('/project/test/browse/services');
        waitForPresence("h1", "Services");
        // TODO: validate presented ports, routes, selectors
      });

      it('should browse settings', function() {
        goToPage('/project/test/settings');
        waitForPresence("h1", "Project Settings");
        // TODO: validate presented project info, quota and resource info
      });
    });
  });
});
