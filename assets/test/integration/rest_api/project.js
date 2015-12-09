require('jasmine-beforeall');
var h = require('../helpers.js');

var goToAddToProjectPage = function(projectName) {
  var uri = '/project/' + projectName + '/create';
  h.goToPage(uri);
  expect(element(by.cssContainingText('h1', "Create Using Your Code")).isPresent()).toBe(true);
  expect(element(by.cssContainingText('h1', "Create Using a Template")).isPresent()).toBe(true);
  expect(element(by.model('from_source_url')).isPresent()).toBe(true);
  expect(element(by.cssContainingText('.catalog h3 > a', "ruby-helloworld-sample")).isPresent()).toBe(true);
}

var goToCreateProjectPage = function() {
  h.goToPage('/createProject');
  expect(element(by.cssContainingText('h1', "New Project")).isPresent()).toBe(true);
  expect(element(by.model('name')).isPresent()).toBe(true);
  expect(element(by.model('displayName')).isPresent()).toBe(true);
  expect(element(by.model('description')).isPresent()).toBe(true);
}

var requestCreateFromSource = function(projectName, sourceUrl) {
  var uri = '/project/' + projectName + '/create';
  h.waitForUri(uri);
  h.setInputValue('from_source_url', sourceUrl);
  var nextButton = element(by.buttonText('Next'));
  browser.wait(protractor.ExpectedConditions.elementToBeClickable(nextButton), 2000);
  nextButton.click();
}

var requestCreateFromTemplate = function(projectName, templateName) {
  var uri = '/project/' + projectName + '/create';
  h.waitForUri(uri);
  var template = element(by.cssContainingText('.catalog h3 > a', templateName));
  expect(template.isPresent()).toBe(true);
  template.click();
}

var attachBuilderImageToSource = function(projectName, builderImageName) {
  var uri = '/project/' + projectName + '/catalog/images';
  h.waitForUri(uri);
  expect(element(by.cssContainingText('h1', "Select a builder image")).isPresent()).toBe(true);
  var builderImageLink = element(by.cssContainingText('h3 > a', builderImageName));
  expect(builderImageLink.isPresent()).toBe(true);
  builderImageLink.click();
}

var createFromSource = function(projectName, builderImageName, appName) {
  var uri = '/project/' + projectName + '/create/fromimage';
  h.waitForUri(uri);
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
  h.clickAndGo('Create', '/project/' + projectName + '/overview');
}

var createFromTemplate = function(projectName, templateName, parameterNames, labelNames) {
  var uri = '/project/' + projectName + '/create/fromtemplate';
  h.waitForUri(uri);
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
  h.clickAndGo('Create', '/project/' + projectName + '/overview');
}

var checkServiceCreated = function(projectName, serviceName) {
  var uri = '/project/' + projectName + '/overview';
  h.goToPage(uri);
  h.waitForPresence('.component .service', serviceName, 10000);
  var uri = '/project/' + projectName + '/browse/services';
  h.goToPage(uri);
  h.waitForPresence('h3', serviceName, 10000);
}

var checkProjectSettings = function(projectName, displayName, description) {
  var uri = '/project/' + projectName + '/settings';
  h.goToPage(uri);
  expect(element.all(by.css("dl > dd")).get(0).getText()).toEqual(projectName);
  expect(element.all(by.css("dl > dd")).get(1).getText()).toEqual(displayName);
  expect(element.all(by.css("dl > dd")).get(2).getText()).toEqual(description);
}


describe('', function() {
  afterAll(function(){
    h.afterAllTeardown();
  });
  describe('authenticated e2e-user', function() {
    beforeEach(function() {
      h.commonSetup();
      h.login();
    });

    afterEach(function() {
      h.commonTeardown();
    });

    describe('new project', function() {
      describe('when creating a new project', function() {
        it('should be able to show the create project page', goToCreateProjectPage);
        var timestamp = (new Date()).getTime();
        var project = {
          name:        'console-test-project-' + timestamp,
          displayName: 'Console integration test Project ' + timestamp,
          description: 'Created by assets/test/integration/rest-api/project.js'
        };

        it('should successfully create a new project', function() {
          goToCreateProjectPage();
          for (var key in project) {
            h.setInputValue(key, project[key]);
          }
          h.clickAndGo('Create', '/project/' + project['name'] + '/create');
          h.waitForPresence('.breadcrumb li a', project['displayName']);
          checkProjectSettings(project['name'], project['displayName'], project['description']);
        });

        it('should browse builds', function() {
          h.goToPage('/project/' + project['name'] + '/browse/builds');
          h.waitForPresence('h1', 'Builds');
          // TODO: validate presented strategies, images, repos
        });

        it('should browse deployments', function() {
          h.goToPage('/project/' + project['name'] + '/browse/deployments');
          h.waitForPresence("h1", "Deployments");
          // TODO: validate presented deployments
        });

        it('should browse events', function() {
          h.goToPage('/project/' + project['name'] + '/browse/events');
          h.waitForPresence("h1", "Events");
          // TODO: validate presented events
        });

        it('should browse image streams', function() {
          h.goToPage('/project/' + project['name'] + '/browse/images');
          h.waitForPresence("h1", "Image Streams");
          // TODO: validate presented images
        });

        it('should browse pods', function() {
          h.goToPage('/project/' + project['name'] + '/browse/pods');
          h.waitForPresence("h1", "Pods");
          // TODO: validate presented pods, containers, correlated images, builds, source
        });

        it('should browse services', function() {
          h.goToPage('/project/' + project['name'] + '/browse/services');
          h.waitForPresence("h1", "Services");
          // TODO: validate presented ports, routes, selectors
        });

        it('should browse settings', function() {
          h.goToPage('/project/' + project['name'] + '/settings');
          h.waitForPresence("h1", "Project Settings");
          // TODO: validate presented project info, quota and resource info
        });

        it('should validate taken name when trying to create', function() {
          goToCreateProjectPage();
          element(by.model('name')).clear().sendKeys(project['name']);
          element(by.buttonText("Create")).click();
          expect(element(by.css("[ng-if=nameTaken]")).isDisplayed()).toBe(true);
          expect(browser.getCurrentUrl()).toMatch(/\/createProject$/);
        });

        it('should delete a project', function() {
          h.goToPage('/project/' + project['name'] + '/settings');
          element(by.css(".action-button .fa-trash-o")).click();
          element(by.cssContainingText(".modal-dialog .btn", "Delete")).click();
          h.waitForPresence(".alert-success", "marked for deletion");
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
    });
  });
});
