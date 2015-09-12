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
  .directive('projectNav', function($timeout, $location, $filter, LabelFilter, DataService, projectOverviewURLFilter) {
    return {
      restrict: 'EA',
      templateUrl: 'views/_project-nav.html',
      link: function($scope, $elem) {
        var select = $elem.find('.selectpicker');
        var projects = {};
        var sortedProjects = [];
        var options = [];

        var updateOptions = function() {
          var project = $scope.project || {};
          var name = $scope.projectName;
          var isRealProject = !_.isEmpty(project);
          var projectsGroup, linksGroup;

          if(!isRealProject) {
            if(!name) {
              return;
            }
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
                      .text($filter('displayName')(item));
          });

          // Use <optgroup> so bootstrap-select adds a divider.
          projectsGroup = $('<optgroup label=""></optgroup>');
          projectsGroup.append(options);
          linksGroup = $('<optgroup label=""><option value="">View all projects</option></optgroup>');
          // TODO: Add a create project link.

          select.empty();
          select.append(projectsGroup);
          select.append(linksGroup);
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

        LabelFilter.setupFilterWidget($elem.find('.navbar-filter-widget'), $elem.find('.active-filters'), { addButtonText: "Add" });
        LabelFilter.toggleFilterWidget(!$scope.renderOptions || !$scope.renderOptions.hideFilterWidget);

        $scope.$watch("project", updateOptions);
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
  .directive('back', ['$window', function($window) {
    return {
      restrict: 'A',
      link: function (scope, elem) {
        elem.bind('click', function () {
          $window.history.back();
        });
      }
    };
  }]);
