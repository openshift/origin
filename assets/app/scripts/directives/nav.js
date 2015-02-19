'use strict';

angular.module('openshiftConsole')
  .directive('sidebar', function(HawtioNav) {
    return {
      restrict: 'E',
      templateUrl: 'views/_sidebar.html',
      link: function($scope, element, attrs) {
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
  .directive('projectNav', function($timeout, $location, LabelFilter) {
    return {
      restrict: 'E',
      templateUrl: 'views/_project-nav.html',
      link: function ($scope, element, attrs) {
        var select = $('.selectpicker', element);

        var updateOptions = function(projects) {
          angular.forEach(projects, function(project) {
            $('<option>')
              .attr("value", project.metadata.name)
              .attr("selected", project.metadata.name == $scope.projectName)
              .text(project.displayName || project.metadata.name)
              .appendTo(select);
          });
          // TODO add back in when we support create project
          // <option data-divider="true"></option>
          // <option>Create new</option>
        };

        updateOptions($scope.projects);

        select.selectpicker({
              iconBase: 'fa',
              tickIcon: 'fa-check'
          }).change(function() {
          var newProject = $( this ).val();
          var currentURL = $location.url();
          var currProjRegex = /\/project\/[^\/]+/;
          var currProjPrefix = currProjRegex.exec(currentURL);
          var newURL = currentURL.replace(currProjPrefix, "/project/" + encodeURIComponent(newProject));
          $scope.$apply(function() {
            $location.url(newURL);
          });
        });
        $scope.$watch("projects", function(projects) {
          select.empty();
          updateOptions(projects);
          select.selectpicker('refresh');
        });

        LabelFilter.setupFilterWidget($(".navbar-filter-widget", element), $(".active-filters", element));
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
