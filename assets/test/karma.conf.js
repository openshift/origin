// Karma configuration
// http://karma-runner.github.io/0.12/config/configuration-file.html
// Generated on 2014-09-12 using
// generator-karma 0.8.3

module.exports = function(config) {
  'use strict';

  config.set({
    // enable / disable watching file and executing tests whenever any file changes
    autoWatch: true,

    // base path, that will be used to resolve files and exclude
    basePath: '../',

    // testing framework to use (jasmine/mocha/qunit/...)
    frameworks: ['jasmine'],

    // list of files / patterns to load in the browser
    files: [
      "bower_components/jquery/dist/jquery.js",
      "bower_components/angular/angular.js",
      'bower_components/angular-mocks/angular-mocks.js',
      "bower_components/angular-resource/angular-resource.js",
      "bower_components/angular-cookies/angular-cookies.js",
      "bower_components/angular-sanitize/angular-sanitize.js",
      "bower_components/angular-animate/angular-animate.js",
      "bower_components/angular-touch/angular-touch.js",
      "bower_components/angular-route/angular-route.js",
      "bower_components/angular-bootstrap/ui-bootstrap-tpls.js",
      "bower_components/angular-patternfly/dist/angular-patternfly.js",
      "bower_components/uri.js/src/URI.js",
      "bower_components/uri.js/src/URITemplate.js",
      "bower_components/uri.js/src/jquery.URI.js",
      "bower_components/uri.js/src/URI.fragmentURI.js",
      "bower_components/moment/moment.js",
      "bower_components/patternfly/components/bootstrap/dist/js/bootstrap.js",
      "bower_components/patternfly/components/bootstrap-select/bootstrap-select.js",
      "bower_components/patternfly/components/c3/c3.min.js",
      "bower_components/js-logger/src/logger.js",
      "bower_components/hawtio-core/hawtio-core.js",
      "bower_components/lodash/lodash.js",
      "bower_components/hawtio-core-navigation/dist/hawtio-core-navigation.js",
      "bower_components/hawtio-extension-service/dist/hawtio-extension-service.js",
      "bower_components/kubernetes-label-selector/labelSelector.js",
      "bower_components/kubernetes-label-selector/labelFilter.js",
      "bower_components/kubernetes-object-describer/dist/object-describer.js",
      "bower_components/messenger/build/js/messenger.js",
      "bower_components/openshift-object-describer/dist/object-describer.js",
      'app/config.js',
      'app/scripts/**/*.js',
      //'test/mock/**/*.js',
      'test/spec/spec-helper.js',
      'test/spec/**/*.js'
    ],

    // list of files / patterns to exclude
    exclude: [],

    // web server port
    port: 8443,

    // Start these browsers, currently available:
    // - Chrome
    // - ChromeCanary
    // - Firefox
    // - Opera
    // - Safari (only Mac)
    // - PhantomJS
    // - IE (only Windows)
    browsers: [
      'PhantomJS'
    ],

    // Which plugins to enable
    plugins: [
      'karma-phantomjs-launcher',
      'karma-jasmine',
      'karma-coverage'
    ],

    // Continuous Integration mode
    // if true, it capture browsers, run tests and exit
    singleRun: false,

    colors: true,

    // level of logging
    // possible values: LOG_DISABLE || LOG_ERROR || LOG_WARN || LOG_INFO || LOG_DEBUG
    logLevel: config.LOG_DEBUG,

    // Help karma find the views on disk in the app subdirectory
    proxies: {
      '/views/': '/app/views/'
    },
    // URL root prevent conflicts with the site root
    // urlRoot: '_karma_'

    preprocessors: {
      // source files, that you wanna generate coverage for
      // do not include tests or libraries
      // (these files will be instrumented by Istanbul)
      'app/**/*.js': ['coverage']
    },

    reporters: ['progress', 'coverage'],

    coverageReporter: {
      reporters:[
        {type: 'json', dir:'test/coverage/'},
        {type: 'text-summary', dir:'test/coverage/'}
      ]
    }    
  });
};
