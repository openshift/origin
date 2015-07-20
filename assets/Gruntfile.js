// Generated on 2014-09-12 using generator-angular 0.9.8
'use strict';
/* jshint unused: false */

// # Globbing
// for performance reasons we're only matching one level down:
// 'test/spec/{,*/}*.js'
// use this if you want to recursively match all subfolders:
// 'test/spec/**/*.js'

var modRewrite = require('connect-modrewrite');

module.exports = function (grunt) {

  // Load grunt tasks automatically
  require('load-grunt-tasks')(grunt, {
    pattern: ['grunt-*', '!grunt-template-jasmine-istanbul']
  });

  // Time how long tasks take. Can help when optimizing build times
  require('time-grunt')(grunt);

  // Configurable paths for the application
  var appConfig = {
    app: require('./bower.json').appPath || 'app',
    dist: 'dist'
  };

  // Define the configuration for all the tasks
  grunt.initConfig({

    // Project settings
    yeoman: appConfig,

    // Watches files for changes and runs tasks based on the changed files
    watch: {
      bower: {
        files: ['bower.json'],
        tasks: ['wiredep']
      },
      js: {
        files: ['<%= yeoman.app %>/scripts/{,*/}*.js'],
        tasks: ['newer:jshint:all'],
        options: {
          livereload: '<%= connect.options.livereload %>'
        }
      },
      jsTest: {
        files: ['test/spec/{,*/}*.js'],
        tasks: ['newer:jshint:test', 'karma']
      },
      css: {
        files: '<%= yeoman.app %>/styles/*.less',
        tasks: ['less']
      },
      gruntfile: {
        files: ['Gruntfile.js']
      },
      livereload: {
        options: {
          livereload: '<%= connect.options.livereload %>'
        },
        files: [
          '<%= yeoman.app %>/{,*/}*.html',
          '.tmp/styles/{,*/}*.css',
          '<%= yeoman.app %>/images/{,*/}*.{png,jpg,jpeg,gif,webp,svg}'
        ]
      }
    },

    // The actual grunt server settings
    connect: {
      options: {
        protocol: grunt.option('scheme') || 'https',
        port: grunt.option('port') || 9000,
        // Change this to '0.0.0.0' to access the server from outside.
        hostname: grunt.option('hostname') || 'localhost',
        livereload: 35729
      },
      livereload: {
        options: {
          open: true,
          middleware: function (connect) {
            return [
              modRewrite(['!^/(config.js|favicon.ico|(java|bower_components|scripts|images|styles|views)(/.*)?)$ /index.html [L]']),
              connect.static('.tmp'),
              connect().use(
                '/java',
                connect.static('./openshift-jvm')
              ),
              connect().use(
                '/bower_components',
                connect.static('./bower_components')
              ),
              connect.static(appConfig.app)
            ];
          }
        }
      },
      test: {
        options: {
          middleware: function (connect) {
            return [
              modRewrite(['!^/(config.js|favicon.ico|(bower_components|scripts|images|styles|views)(/.*)?)$ /index.html [L]']),
              connect.static('.tmp'),
              connect.static('test'),
              connect().use(
                '/bower_components',
                connect.static('./bower_components')
              ),
              connect.static(appConfig.app)
            ];
          }
        }
      },
      dist: {
        options: {
          open: true,
          base: '<%= yeoman.dist %>'
        }
      }
    },

    // Make sure code styles are up to par and there are no obvious mistakes
    jshint: {
      options: {
        jshintrc: '.jshintrc',
        reporter: require('jshint-stylish')
      },
      all: {
        src: [
          'Gruntfile.js',
          '<%= yeoman.app %>/scripts/{,*/}*.js'
        ]
      },
      test: {
        options: {
          jshintrc: 'test/.jshintrc'
        },
        src: ['test/spec/{,*/}*.js']
      }
    },

    // Empties folders to start fresh
    clean: {
      dist: {
        files: [{
          dot: true,
          src: [
            '.tmp',
            '<%= yeoman.dist %>/{,*/}*',
            '!<%= yeoman.dist %>/.git*'
          ]
        }]
      },
      server: '.tmp'
    },

    // Add vendor prefixed styles
    autoprefixer: {
      options: {
        browsers: ['last 1 version']
      },
      dist: {
        files: [{
          expand: true,
          cwd: '.tmp/styles/',
          src: '{,*/}*.css',
          dest: '.tmp/styles/'
        }]
      }
    },

    // Automatically inject Bower components into the app
    wiredep: {
      app: {
        src: ['<%= yeoman.app %>/index.html'],
        ignorePath:  /\.\.\//,
        exclude: [
          'bower_components/uri.js/src/IPv6.js',
          'bower_components/uri.js/src/SecondLevelDomains.js',
          'bower_components/uri.js/src/punycode.js',
          'bower_components/uri.js/src/URI.min.js',
          'bower_components/uri.js/src/jquery.URI.min.js',
          'bower_components/uri.js/src/URI.fragmentQuery.js',
          'bower_components/messenger/build/css/messenger.css',
          'bower_components/messenger/build/css/messenger-theme-future.css',
          'bower_components/messenger/build/css/messenger-theme-block.css',
          'bower_components/messenger/build/css/messenger-theme-air.css',
          'bower_components/messenger/build/css/messenger-theme-ice.css',
          'bower_components/messenger/build/js/messenger-theme-future.js',
          'bower_components/fontawesome/css/font-awesome.css'
        ]
      }
    },

    less: {
      development: {
        files: {
          '.tmp/styles/main.css': '<%= yeoman.app %>/styles/main.less'
        },
        options: {
          paths: ['<%= yeoman.app %>/styles']
        }
      },
      production: {
        files: {
          'dist/css/main.css': '<%= yeoman.app %>/styles/main.less'
        },
        options: {
          cleancss: true,
          paths: ['<%= yeoman.app %>/styles']
        }
      }
    },

    // Renames files for browser caching purposes
    filerev: {
      dist: {
        src: [
          '<%= yeoman.dist %>/scripts/{,*/}*.js',
          '<%= yeoman.dist %>/styles/{,*/}*.css',
          '<%= yeoman.dist %>/images/{,*/}*.{png,jpg,jpeg,gif,webp,svg}',
          '<%= yeoman.dist %>/styles/fonts/*'
        ]
      }
    },

    // Reads HTML for usemin blocks to enable smart builds that automatically
    // concat, minify and revision files. Creates configurations in memory so
    // additional tasks can operate on them
    useminPrepare: {
      html: '<%= yeoman.app %>/index.html',
      options: {
        dest: '<%= yeoman.dist %>',
        flow: {
          html: {
            steps: {
              js: ['concat', 'uglifyjs'],
              css: ['cssmin']
            },
            post: {
              css: [{
                name:'cssmin',
                createConfig: function(context, block) {
                  var generated = context.options.generated;
                  generated.options = {
                    keepBreaks: true,
                  };
                }
              }],

              js: [{
                name:'uglify',
                createConfig: function(context, block) {
                  var generated = context.options.generated;
                  generated.options = {
                    compress: {},
                    mangle: {},
                    beautify: {
                      beautify: true,
                      indent_level: 0, // Don't waste characters indenting
                      space_colon: false, // Don't waste characters
                      width: 1000,
                    },
                  };
                }
              }]
            }
          }
        }
      }
    },

    // Performs rewrites based on filerev and the useminPrepare configuration
    usemin: {
      html: ['<%= yeoman.dist %>/{,*/}*.html'],
      css: ['<%= yeoman.dist %>/styles/{,*/}*.css'],
      options: {
        assetsDirs: ['<%= yeoman.dist %>','<%= yeoman.dist %>/images']
      }
    },

    // The following *-min tasks will produce minified files in the dist folder
    // By default, your `index.html`'s <!-- Usemin block --> will take care of
    // minification. These next options are pre-configured if you do not wish
    // to use the Usemin blocks.
    // cssmin: {
    //   dist: {
    //     files: {
    //       '<%= yeoman.dist %>/styles/main.css': [
    //         '.tmp/styles/{,*/}*.css'
    //       ]
    //     }
    //   }
    // },
    // uglify: {
    //   dist: {
    //     files: {
    //       '<%= yeoman.dist %>/scripts/scripts.js': [
    //         '<%= yeoman.dist %>/scripts/scripts.js'
    //       ]
    //     }
    //   }
    // },
    // concat: {
    //   dist: {}
    // },

    imagemin: {
      dist: {
        files: [{
          expand: true,
          cwd: '<%= yeoman.app %>/images',
          src: '{,*/}*.{png,jpg,jpeg,gif}',
          dest: '<%= yeoman.dist %>/images'
        }]
      }
    },

    svgmin: {
      dist: {
        files: [{
          expand: true,
          cwd: '<%= yeoman.app %>/images',
          src: '{,*/}*.svg',
          dest: '<%= yeoman.dist %>/images'
        }]
      }
    },

    htmlhint: {
      html: {
        options: {
          'tag-pair': true,
          'attr-no-duplication': true
        },
        src: ['app/**/*.html']
      }
    },

    htmlmin: {
      dist: {
        options: {
          preserveLineBreaks: true,
          collapseWhitespace: true,
          conservativeCollapse: false,
          collapseBooleanAttributes: true,
          removeComments: true,
          removeCommentsFromCDATA: true,
          removeOptionalTags: false,
          keepClosingSlash: true
        },
        files: [{
          expand: true,
          cwd: '<%= yeoman.dist %>',
          src: ['*.html', 'views/{,*/}*.html'],
          dest: '<%= yeoman.dist %>'
        }]
      }
    },

    // ng-annotate tries to make the code safe for minification automatically
    // by using the Angular long form for dependency injection.
    ngAnnotate: {
      dist: {
        files: [{
          expand: true,
          cwd: '.tmp/concat/scripts',
          src: ['*.js', '!oldieshim.js'],
          dest: '.tmp/concat/scripts'
        }]
      }
    },

    // Replace Google CDN references
    cdnify: {
      dist: {
        html: ['<%= yeoman.dist %>/*.html']
      }
    },

    // Copies remaining files to places other tasks can use
    copy: {
      dist: {
        files: [{
          expand: true,
          dot: true,
          cwd: '<%= yeoman.app %>',
          dest: '<%= yeoman.dist %>',
          src: [
            '*.{ico,png,txt}',
            '.htaccess',
            '*.html',
            'views/{,*/}*.html',
            'images/{,*/}*.{png,jpg,jpeg,gif}',
            'images/{,*/}*.{webp}',
            'fonts/*',
            'styles/fonts/*'
          ]
        }, {
          expand: true,
          cwd: '.tmp/images',
          dest: '<%= yeoman.dist %>/images',
          src: ['generated/*']
        }, {
          expand: true,
          cwd: 'bower_components/patternfly/dist',
          src: 'fonts/*',
          dest: '<%= yeoman.dist %>/styles'
        }, {
          expand: true,
          cwd: 'bower_components/patternfly/components/font-awesome',
          src: 'fonts/*',
          dest: '<%= yeoman.dist %>/styles'
        },
        {
          expand: true,
          cwd: 'bower_components/zeroclipboard/dist',
          src: 'ZeroClipboard.swf',
          dest: '<%= yeoman.dist %>/scripts'
        },

        // Copy separate components
        {
          expand: true,
          cwd: 'openshift-jvm',
          src: '**/*',
          // Copy to a separate "dist.*" directory for go-bindata
          // Make the folder structure inside the dist.* directory match the desired path
          dest: '<%= yeoman.dist %>.java/java'
        }]
      },
      styles: {
        files: [{
          expand: true,
          cwd: '<%= yeoman.app %>/styles',
          dest: '.tmp/styles/',
          src: '{,*/}*.css'
        }, {
          expand: true,
          cwd: 'bower_components/patternfly/dist',
          src: 'fonts/*',
          dest: '.tmp/styles'
        }, {
          expand: true,
          cwd: 'bower_components/patternfly/components/font-awesome',
          src: 'fonts/*',
          dest: '.tmp/styles'
        }]
      }
    },

    // Run some tasks in parallel to speed up the build process
    concurrent: {
      server: [
        'less:development',
        'copy:styles'
      ],
      test: [
        'less:development'
      ],
      dist: [
        'less:production',
        // remove imagemin from build, since it doesn't tend to behave well cross-platform
        // 'imagemin',
        'svgmin'
      ]
    },

    // Test settings
    karma: {
      unit: {
        configFile: 'test/karma.conf.js',
        singleRun: true
      }
    },

    protractor: {
      options: {
        configFile: "test/protractor.conf.js", // Default config file
        keepAlive: false, // If false, the grunt process stops when the test fails.
        noColor: false, // If true, protractor will not use colors in its output.
        args: {
          // Arguments passed to the command
        }
      },
      phantomjs: {},
      chrome: {
        options: {
          configFile: "test/protractor-chrome.conf.js", // Target-specific config file
          args: {} // Target-specific arguments
        }
      }
    },

    // Settings for grunt-istanbul-coverage
    // NOTE: coverage task is currently not in use
    coverage: {
      options: {
        thresholds: {
          'statements': 90,
          'branches': 90,
          'lines': 90,
          'functions': 90
        },
        dir: 'coverage',
        root: 'test'
      }
    }
  });


  grunt.registerTask('serve', 'Compile then start a connect web server', function (target) {
    if (target === 'dist') {
      return grunt.task.run(['build', 'connect:dist:keepalive']);
    }

    grunt.task.run([
      'clean:server',
      'wiredep',
      'concurrent:server',
      'autoprefixer',
      'connect:livereload',
      'watch'
    ]);
  });

  grunt.registerTask('server', 'DEPRECATED TASK. Use the "serve" task instead', function (target) {
    grunt.log.warn('The `server` task has been deprecated. Use `grunt serve` to start a server.');
    grunt.task.run(['serve:' + target]);
  });

  // Loads the coverage task which enforces the minimum coverage thresholds
  grunt.loadNpmTasks('grunt-istanbul-coverage');

  grunt.loadNpmTasks('grunt-htmlhint');

  // karma must run prior to coverage since karma will generate the coverage results
  grunt.registerTask('test', [
    'clean:server',
    'concurrent:test',
    'autoprefixer',
    'connect:test',
    'karma'
    // 'coverage' - add back if we want to enforce coverage percentages
  ]);

  grunt.registerTask('test-e2e', [
    'clean:server',
    'concurrent:server',
    'autoprefixer',
    'connect:test',
    'protractor:phantomjs',
    'clean:server'
  ]);

  grunt.registerTask('test-e2e-chrome', [
    'clean:server',
    'concurrent:server',
    'autoprefixer',
    'connect:test',
    'protractor:chrome',
    'clean:server'
  ]);

  grunt.registerTask('build', [
    'clean:dist',
    'newer:jshint',
    'htmlhint',
    'wiredep',
    'useminPrepare',
    'concurrent:dist',
    'autoprefixer',
    'concat',
    'ngAnnotate',
    'copy:dist',
    'cdnify',
    'less',
    'cssmin',
    'uglify',
    'filerev',
    'usemin',
    'htmlmin'
  ]);

  grunt.registerTask('default', [
    'newer:jshint',
    'test',
    'build'
  ]);
};
