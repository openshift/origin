"use strict";

angular.module('openshiftConsole')
  .directive('buildTrendsChart', function($filter, $location, $rootScope) {
    return {
      restrict: 'E',
      scope: {
        builds: '='
      },
      templateUrl: 'views/_build-trends-chart.html',
      link: function($scope) {
        var buildByNumber;
        var completePhases = ['Complete', 'Failed', 'Cancelled', 'Error'];

        // Simple humanize function that returns an abbreviated duration like "1h 4m" or "2m 4s"
        // suitable for use on the chart y-axis.
        var humanize = function(value) {
          var result = [];
          var duration = moment.duration(value);
          var hours = Math.floor(duration.asHours());
          var minutes = duration.minutes();
          var seconds = duration.seconds();

          if (!hours && !minutes && !seconds) {
            return '';
          }

          if (hours) {
            result.push(hours + "h");
          }
          if (minutes) {
            result.push(minutes + "m");
          }

          // Only show seconds if not duration doesn't include hours.
          // Always show seconds otherwise (even 0s).
          if (!hours) {
            result.push(seconds + "s");
          }

          return result.join(" ");
        };

        var getStartTimestsamp = function(build) {
          return build.status.startTimestamp || build.metadata.creationTimestamp;
        };

        // Chart configuration, see http://c3js.org/reference.html
        $scope.chartID = _.uniqueId('build-trends-chart-');
        var animationDuration = _.constant(350);
        var config = {
          bindto: '#' + $scope.chartID,
          padding: {
            right: 30,
            // Prevent problems with the y-axis label getting cut off on updates.
            left: 80
          },
          axis: {
            x: {
              fit: true,
              label: {
                text: 'Build Number',
                position: 'outer-right'
              },
              tick: {
                culling: true,
                fit: true,
                format: function(x) {
                  return '#' + x;
                }
              }
            },
            y: {
              label: {
                text: 'Duration',
                position: 'outer-top'
              },
              min: 0,
              padding: {
                bottom: 0
              },
              tick: {
                count: 5,
                culling: true,
                fit: true,
                format: humanize
              }
            }
          },
          bar: {
            width: {
              max: 50
            }
          },
          legend: {
            item: {
              // Disable clicking to filter groups
              onclick: _.noop
            }
          },
          size: {
            height: 200
          },
          tooltip: {
            format: {
              title: function(x) {
                var build = buildByNumber[x];
                var startTimestamp = getStartTimestsamp(build);
                return '#' + x + ' (' + moment(startTimestamp).fromNow() + ')';
              }
            }
          },
          transition: {
            duration: animationDuration()
          },
          data: {
            // https://www.patternfly.org/styles/color-palette/
            colors: {
              Cancelled: "#d1d1d1",
              Complete: "#00b9e4",
              Error: "#393f44",
              Failed: "#cc0000"
            },
            empty: {
              label: {
                text: "No Completed Builds"
              }
            },
            onclick: function(d) {
              var build = buildByNumber[d.x];
              var url = $filter('navigateResourceURL')(build);
              if (url) {
                $rootScope.$apply(function() {
                  $location.path(url);
                });
              }
            },
            selection: {
              enabled: true
            },
            type: 'bar'
          }
        };

        var updateCompleteBuilds = function() {
          $scope.completeBuilds = [];
          var isIncomplete = $filter('isIncompleteBuild');
          angular.forEach($scope.builds, function(build) {
            if (!isIncomplete(build)) {
              $scope.completeBuilds.push(build);
            }
          });
        };

        var numCompleteBuilds = function() {
          updateCompleteBuilds();
          return $scope.completeBuilds.length;
        };

        var annotationFilter = $filter('annotation');
        var getBuildNumber = function(build) {
          var buildNumber = annotationFilter(build, 'buildNumber') || parseInt(build.metadata.name.match(/(\d+)$/), 10);
          if (isNaN(buildNumber)) {
            return null;
          }

          return buildNumber;
        };

        var getBuildDuration = function(build) {
          var startTimestamp = getStartTimestsamp(build);
          var endTimestamp = build.status.completionTimestamp;
          if (!startTimestamp || !endTimestamp) {
            return 0;
          }

          return moment(endTimestamp).diff(moment(startTimestamp));
        };

        var chart, averageDuration, showAverageLine = false;
        var updateAvgLine = function() {
          if (averageDuration && showAverageLine) {
            chart.ygrids([{
              value: averageDuration,
              'class': 'build-trends-avg-line'
            }]);
          } else {
            chart.ygrids.remove();
          }
        };

        $scope.toggleAvgLine = function() {
          showAverageLine = !showAverageLine;
          updateAvgLine();
        };

        var update = function() {
          // Keep a map of builds by number so we can find the build later when a data point is clicked.
          buildByNumber = {};
          var data = {
            json: [],
            keys: {
              x: 'buildNumber'
            }
          };

          var sum = 0, count = 0;
          angular.forEach($scope.completeBuilds, function(build) {
            var buildNumber = getBuildNumber(build);
            if (!buildNumber) {
              return;
            }

            var duration = getBuildDuration(build);

            // Track the sum and count to calculate the average duration.
            sum += duration;
            count++;

            var buildData = {
              buildNumber: buildNumber,
              phase: build.status.phase
            };
            buildData[build.status.phase] = duration;
            data.json.push(buildData);
            buildByNumber[buildNumber] = build;
          });

          // Show only the last 50 builds.
          if (data.json.length > 50) {
            data.json.sort(function(a, b) {
              return a.buildNumber - b.buildNumber;
            });
            data.json = data.json.slice(data.json.length - 50);
          }

          // Check for found phases only after we've sliced the array.
          var foundPhases = {};
          angular.forEach(data.json, function(buildData) {
            foundPhases[buildData.phase] = true;
          });

          // Calculate the average duration.
          // TODO: Should we only show the average for the last 50 builds
          //       instead of all builds?
          if (count) {
            averageDuration = sum / count;
            $scope.averageDurationText = humanize(averageDuration);
          } else {
            averageDuration = null;
            $scope.averageDurationText = null;
          }

          var groups = [], unload = [];
          angular.forEach(completePhases, function(phase) {
            if (foundPhases[phase]) {
              // Only show found phases in the chart legend.
              groups.push(phase);
            } else {
              // Unload any groups not found to remove them from the chart.
              // This can happen for filters, deleted builds, or if a build
              // phase is no longer in the last 50.
              unload.push(phase);
            }
          });
          data.keys.value = groups;
          data.groups = [groups];

          if (!chart) {
            config.data = angular.extend(data, config.data);
            chart = c3.generate(config);
          } else {
            data.unload = unload;
            // Call flush to work around a c3.js bug where the y-axis label
            // overlaps the tick values when the data changes.
            data.done = function() {
              // done() is called before the chart animation finishes.
              // Wait until the animation is complete to call flush().
              setTimeout(function() {
                chart.flush();
              }, animationDuration() + 25);
            };
            chart.load(data);
          }

          // Update average line.
          updateAvgLine();
        };

        $scope.$watch(numCompleteBuilds, update);

        $scope.$on('destroy', function() {
          if (chart) {
            // http://c3js.org/reference.html#api-destroy
            chart = chart.destroy();
          }
        });
      }
    };
  });

