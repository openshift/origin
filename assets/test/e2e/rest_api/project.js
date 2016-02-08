'use strict';
/* jshint unused:false */

require('jasmine-beforeall');
// helpers
var h = require('../helpers/helpers.js');
var user = require('../helpers/user.js');
var win = require('../helpers/window.js');
var browse = require('../helpers/browser.js');
var Project =  require('../helpers/project.js');
// matchers, shared across pages, contain 'expect' assertions
var expectInput = require('../matchers/expectInput.js');
var expectHeading = require('../matchers/expectHeading.js');
var expectName = require('../matchers/expectName.js');
var expectDisplayName = require('../matchers/expectDisplayName.js');
var expectDescription = require('../matchers/expectDescription.js');
// page_objects
var settingsPage = require('../page_objects/settingsPage.js');
var buildsPage = require('../page_objects/buildsPage.js');
var deploymentsPage = require('../page_objects/deploymentsPage.js');
var eventsPage = require('../page_objects/eventsPage.js');
var imagesPage = require('../page_objects/imagesPage.js');
var podsPage = require('../page_objects/podsPage.js');
var servicesPage = require('../page_objects/servicesPage.js');
var createProjectPage = require('../page_objects/createProjectPage.js');
var addToProjectPage = require('../page_objects/addToProjectPage.js');


// TODO: share some config to pass to all methods in this file?
var maxTimeout = 5000;

describe('e2e tests', function() {

  afterAll(function(){
    win.clearStorage();
    browser.driver.sleep(1000);
  });

  describe('authenticated e2e-user', function() {

    beforeEach(function() {
      // Want a longer browser size since the screenshot reporter only grabs the visible window
      win.resize();
      user.login();
    });

    afterEach(function() {
      win.clearStorage();
    });

    describe('new project', function() {

      describe('when creating a new project', function() {

        it('should be able to show the create project page', function() {
          createProjectPage.visit();
          expectHeading('New Project');
          expectInput('name');
          expectInput('displayName');
          expectInput('description');
        });

        var project = new Project();

        it('should successfully create a new project', function() {
          createProjectPage.visit();
          createProjectPage
            .submitProject(project)
            // the submit action should redirect to addToProjectPage
            // this may not need to be a promise.then(),
            // but addToProjectPage.visit() is not being called here.
            .then(function() {
              expect(browse.urlMatches(addToProjectPage.uri(project)));

              h.waitForPresence('.breadcrumb li a', project['displayName']);

              settingsPage.visit(project);
              expectName(project.name);
              expectDisplayName(project.displayName);
              expectDescription(project.description);
            });
        });


        it('should browse builds', function() {
          buildsPage.visit(project);
          expectHeading('Builds');
          // TODO: validate presented strategies, images, repos
        });

        it('should browse deployments', function() {
          deploymentsPage.visit(project);
          expectHeading('Deployments');
          // TODO: validate presented deployments
        });

        it('should browse events', function() {
          eventsPage.visit(project);
          expectHeading('Events');
          // TODO: validate presented events
        });

        it('should browse image streams', function() {
          imagesPage.visit(project);
          expectHeading('Image Streams');
          // TODO: validate presented images
        });

        it('should browse pods', function() {
          podsPage.visit(project);
          expectHeading('Pods');
          // TODO: validate presented pods, containers, correlated images, builds, source
        });

        it('should browse services', function() {
          servicesPage.visit(project);
          expectHeading('Services');
          // TODO: validate presented ports, routes, selectors
        });

        it('should browse settings', function() {
          settingsPage.visit(project);
          expectHeading('Project Settings');
          // TODO: validate presented project info, quota and resource info
        });

        it('should validate taken name when trying to create', function() {
          createProjectPage.visit();
          element(by.model('name')).clear().sendKeys(project['name']);
          element(by.buttonText("Create")).click();
          expect(element(by.css("[ng-if=nameTaken]")).isDisplayed()).toBe(true);
          expect(browser.getCurrentUrl()).toMatch(/\/create-project$/);
        });

        it('should delete a project', function() {
          settingsPage.visit(project);
          settingsPage.openMenu();
          settingsPage.deleteProject();
          settingsPage.confirmProjectDelete();
          settingsPage.expectSuccessfulDelete();
        });


  /* BROKEN/STALE/INVALID tests:



      var goToAddToProjectPage = function(projectName) {
        addToProject.visit();
        addToProject.expectHeading('Create Using Your Code');
        addToProject.expectHeading('Create Using a Template');
        addToProject.expectFromSourceUrl();
        addToProject.expectTemplate('ruby-helloworld-sample');
        addToProject.expectBuilderImage('ruby:2.0');
      };



      // TODO: needs project, not just projectName!
      var requestCreateFromSource = function(projectName, sourceUrl) {
        addToProject.visit(); // TODO: project!
        addToProject.setFromSource(sourceUrl);
        addToProject.next();
      };


      // TODO: needs project, not just projectName!
      var requestCreateFromTemplate = function(projectName, templateName) {
        var uri = '/project/' + projectName + '/create';
        h.waitForUri(uri);
        var template = element(by.cssContainingText('.catalog h3 > a', templateName));
        expect(template.isPresent()).toBe(true);
        template.click();
      };

      var attachBuilderImageToSource = function(projectName, builderImageName) {
        var uri = '/project/' + projectName + '/catalog/images';
        h.waitForUri(uri);
        expect(element(by.cssContainingText('h1', "Select a builder image")).isPresent()).toBe(true);
        var builderImageLink = element(by.cssContainingText('h3 > a', builderImageName));
        expect(builderImageLink.isPresent()).toBe(true);
        builderImageLink.click();
      };




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
      };

      var createFromTemplate = function(projectName, templateName, parameterNames, labelNames) {
        var uri = '/project/' + projectName + '/create/fromtemplate';
        h.waitForUri(uri);
        expect(element(by.css('.create-from-template h1')).getText()).toEqual(templateName);
        expect(element(by.cssContainingText('h2', "Images")).isPresent()).toBe(true);
        expect(element(by.cssContainingText('h2', "Parameters")).isPresent()).toBe(true);
        expect(element(by.cssContainingText('h2', "Labels")).isPresent()).toBe(true);
        if (parameterNames) {
          parameterNames.forEach(function(val) {
            expect(element(by.cssContainingText('.env-variable-list label.key', val)).isPresent()).toBe(true);
          });
        }
        if (labelNames) {
          labelNames.forEach(function(val) {
            expect(element(by.cssContainingText('.label-list span.key', val)).isPresent()).toBe(true);
          });
        }
        h.clickAndGo('Create', '/project/' + projectName + '/overview');
      };

      var checkServiceCreated = function(projectName, serviceName) {
        browse.goTo('/project/' + projectName + '/overview');
        h.waitForPresence('.component .service', serviceName, 10000);
        browse.goTo('/project/' + projectName + '/browse/services');
        h.waitForPresence('h3', serviceName, 10000);
      };


      // ------------------------------------------------------------------------------------------------------------



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
