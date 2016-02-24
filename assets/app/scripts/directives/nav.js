'use strict';

angular.module('openshiftConsole')
  .directive('sidebar', function(HawtioNav) {
    return {
      restrict: 'E',
      templateUrl: 'views/_sidebar.html',
      link: function($scope) {
        var selectedTab = HawtioNav.selected();
        if (selectedTab) {
          $scope.sidebarHeading = selectedTab.title();
        }
      }
    };
  })
  .directive('sidebarNavItem', function() {
    return {
      restrict: 'E',
      replace: true,
      templateUrl: "views/_sidebar-main-nav-item.html"
    };
  })
  .directive('projectHeader', function($timeout, $location, $filter, DataService, projectOverviewURLFilter) {

    // cache these to eliminate flicker
    var projects = {};
    var sortedProjects = [];

    return {
      restrict: 'EA',
      templateUrl: 'views/directives/header/project-header.html',
      link: function($scope, $elem) {
        var select = $elem.find('.selectpicker');
        var options = [];

        var updateOptions = function() {
          var project = $scope.project || {};
          var name = $scope.projectName;
          var isRealProject = project.metadata && project.metadata.name;

          // If we don't have a name or a real project, nothing to do yet.
          if (!name && !isRealProject) {
            return;
          }

          if (!name) {
            name = project.metadata.name;
          }

          if (!isRealProject) {
            project = {
              metadata: {
                name: name
              }
            };
          }

          if(!projects[name]) {
            projects[name] = project;
          }

          sortedProjects = $filter('orderByDisplayName')(projects);
          options = _.map(sortedProjects, function(item) {
            return $('<option>')
                      .attr("value", item.metadata.name)
                      .attr("selected", item.metadata.name === name)
                      .text($filter("uniqueDisplayName")(item, sortedProjects));
          });

          select.empty();
          select.append(options);
          select.append($('<option data-divider="true"></option>'));
          select.append($('<option value="">View all projects</option>'));
          select.selectpicker('refresh');
        };


        DataService.list("projects", $scope, function(items) {
          projects = items.by("metadata.name");
          updateOptions();
        });

        updateOptions();

        select
          .selectpicker({
            iconBase: 'fa',
            tickIcon: 'fa-check'
          })
          .change(function() {
            var val = $(this).val();
            var newURL = (val === "") ? "/" : projectOverviewURLFilter(val);
            $scope.$apply(function() {
              $location.url(newURL);
            });
          });

        $scope.$on('project.settings.update', function(event, data) {
          projects[data.metadata.name] = data;
          updateOptions();
        });

      }
    };
  })
  .directive('projectFilter', function(LabelFilter) {
    return {
      restrict: 'E',
      templateUrl: 'views/directives/_project-filter.html',
      link: function($scope, $elem) {
        LabelFilter.setupFilterWidget($elem.find('.navbar-filter-widget'), $elem.find('.active-filters'), { addButtonText: "Add" });
        LabelFilter.toggleFilterWidget(!$scope.renderOptions || !$scope.renderOptions.hideFilterWidget);

        $scope.$watch("renderOptions", function(renderOptions) {
          LabelFilter.toggleFilterWidget(!renderOptions || !renderOptions.hideFilterWidget);
        });
      }
    };
  })
  .directive('projectPage', function() {
    return {
      restrict: 'E',
      transclude: true,
      templateUrl: 'views/_project-page.html'
    };
  })
  .directive('navbarUtility', function() {
    return {
      restrict: 'E',
      transclude: true,
      templateUrl: 'views/directives/header/_navbar-utility.html'
    };
  })
  .directive('navbarUtilityMobile', function() {
    return {
      restrict: 'E',
      transclude: true,
      templateUrl: 'views/directives/header/_navbar-utility-mobile.html'
    };
  })
  .directive('defaultHeader', function() {
    return {
      restrict: 'E',
      transclude: true,
      templateUrl: 'views/directives/header/default-header.html'
    };
  })
  // TODO: rename this :)
  .directive('navPfVerticalAlt', function() {
    return {
      restrict: 'EAC',
      link: function() {
        // Short term solution to trigger the patternfly nav
        $.fn.navigation();
      }
    };
  })
  .directive('breadcrumbs', function() {
    return {
      restrict: 'E',
      scope: {
        breadcrumbs: '='
      },
      templateUrl: 'views/directives/breadcrumbs.html'
    };
  })
  .directive('back', ['$window', function($window) {
    return {
      restrict: 'A',
      link: function (scope, elem) {
        elem.bind('click', function () {
          $window.history.back();
        });
      }
    };
  }])
  .directive('oscSecondaryNav', function() {
    return {
      restrict: 'A',
      scope: {
        tabs: '='
      },
      templateUrl: 'views/directives/osc-secondary-nav.html'
    };
  });
