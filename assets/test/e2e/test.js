require('jasmine-beforeall');
var fs = require('fs');

describe('', function() {
  var commonTeardown = function() {
    browser.executeScript('window.sessionStorage.clear();');
    browser.executeScript('window.localStorage.clear();');
  };

  afterEach(function () {
    // var spec = jasmine.getEnv().currentSpec;
    // var filename = __dirname + '/screenshots/' + spec.description.split(' ').join('_') + '.png';
    // if (!spec.results().passed()) {
    //   browser.takeScreenshot().then(function(png) {
    //     var stream = fs.createWriteStream(filename);
    //     stream.write(new Buffer(png, 'base64'));
    //     stream.end();
    //   });
    // }
  });

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
    return driver.findElement(by.css("button[type='submit']")).click();
  };

  var goToAddToProjectPage = function(projectName) {
    var uri = '/project/' + projectName + '/create';
    browser.get(uri);
    expect(element(by.cssContainingText('h1', "Create Using Your Code")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h1', "Create Using a Template")).isPresent()).toBe(true);
    expect(element(by.model('from_source_url')).isPresent()).toBe(true);
    expect(element(by.cssContainingText('.catalog h3 > a', "ruby-helloworld-sample")).isPresent()).toBe(true);
  }

  var goToCreateProjectPage = function() {
    browser.get('/createProject');
    expect(element(by.cssContainingText('h1', "New Project")).isPresent()).toBe(true);
    expect(element(by.model('name')).isPresent()).toBe(true);
    expect(element(by.model('displayName')).isPresent()).toBe(true);
    expect(element(by.model('description')).isPresent()).toBe(true);
  }

  var requestCreateFromSource = function(projectName, sourceUrl) {
    var uri = '/project/' + projectName + '/create';
    expect(browser.getCurrentUrl()).toContain(uri);
    element(by.model('from_source_url')).clear().sendKeys(sourceUrl);
    var nextButton = element(by.buttonText('Next'));
    browser.wait(protractor.ExpectedConditions.elementToBeClickable(nextButton), 3000);
    return nextButton.click();
  }

  var requestCreateFromTemplate = function(projectName, templateName) {
    var uri = '/project/' + projectName + '/create';
    expect(browser.getCurrentUrl()).toContain(uri);
    var template = element(by.cssContainingText('.catalog h3 > a', templateName));
    expect(template.isPresent()).toBe(true);
    return template.click();
  }

  var attachBuilderImageToSource = function(projectName, builderImageName) {
    var uri = '/project/' + projectName + '/catalog/images';
    expect(browser.getCurrentUrl()).toContain(uri);
    expect(element(by.cssContainingText('h1', "Select a builder image")).isPresent()).toBe(true);
    var builderImageLink = element(by.cssContainingText('h3 > a', builderImageName));
    expect(builderImageLink.isPresent()).toBe(true);
    return builderImageLink.click();
  }

  var createFromSourceSummary = function(projectName, builderImageName, appName) {
    var uri = '/project/' + projectName + '/create/fromimage';
    expect(browser.getCurrentUrl()).toContain(uri);
    expect(element(by.css('.create-from-image h1')).getText()).toEqual(builderImageName);
    expect(element(by.cssContainingText('h2', "Name")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Routing")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Deployment Configuration")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Build Configuration")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Scaling")).isPresent()).toBe(true);
    expect(element(by.cssContainingText('h2', "Labels")).isPresent()).toBe(true);
    element(by.name('appname')).clear().sendKeys(appName);
    return element(by.buttonText("Create")).click();
  }

  var createFromTemplateSummary = function(projectName, templateName, parameterNames, labelNames) {
    var uri = '/project/' + projectName + '/create/fromtemplate';
    expect(browser.getCurrentUrl()).toContain(uri);
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
    return element(by.buttonText("Create")).click();
  }

  var checkServiceCreated = function(projectName, serviceName) {
    var uri = '/project/' + projectName + '/overview';
    expect(browser.getCurrentUrl()).toContain(uri);
    var service = element(by.cssContainingText('.component .service', serviceName));
    browser.wait(protractor.ExpectedConditions.presenceOf(service), 5000);
  }

  var checkProjectSettings = function(projectName, displayName, description) {
    var uri = '/project/' + projectName + '/settings';
    browser.get(uri);
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

    describe('with test project', function() {
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

      describe('when adding to project', function() {
        it('should view the create page', function() { goToAddToProjectPage("test"); });

        it('should create from source', function() {
          var projectName = "test";
          var sourceUrl = "https://github.com/openshift/rails-ex";
          var appName = "my-rails-ex";
          var builderImage = "ruby";
          goToAddToProjectPage(projectName);
          requestCreateFromSource(projectName, sourceUrl).then(function() {
            attachBuilderImageToSource(projectName, builderImage).then(function() {
              createFromSourceSummary(projectName, builderImage, appName).then(function() {
                checkServiceCreated(projectName, appName);
              });
            });
          });
        });

        it('should create from template', function() {
          var projectName = "test";
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
          requestCreateFromTemplate(projectName, templateName).then(function() {
            createFromTemplateSummary(projectName, templateName, parameterNames, labelNames).then(function() {
              checkServiceCreated(projectName, "frontend");
              checkServiceCreated(projectName, "database");
            });
          });
        });
      });
    });

    describe('when creating a new project', function() {
      it('should be able to show the create project page', goToCreateProjectPage);

      it('should successfully create a new project', function() {
        var project = {
          name:        'e2e-test-project',
          displayName: 'End-to-end Test Project',
          description: 'Project created by the e2e web console tests.'
        };

        goToCreateProjectPage();
        for (var key in project) {
          element(by.model(key)).clear().sendKeys(project[key]);
        }
        element(by.buttonText("Create")).click().then(function() {
          var uri = '/project/' + project['name'] + '/overview';
          expect(browser.getCurrentUrl()).toContain(uri);
          expect(element(by.cssContainingText("h1", project['displayName'])).isPresent()).toBe(true);
          checkProjectSettings(project['name'], project['displayName'], project['description']);
        });
      });

      it('should validate taken name when trying to create', function() {
        goToCreateProjectPage();
        element(by.model('name')).clear().sendKeys("test");
        element(by.cssContainingText('button', "Create")).click().then(function() {
          expect(element(by.css("[ng-if=nameTaken]")).isDisplayed()).toBe(true);
          expect(browser.getCurrentUrl()).toMatch(/\/createProject$/);
        });    
      });
    });
  });
});
