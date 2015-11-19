'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:CreateController
 * @description
 * # CreateController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('CreateController', function ($scope, DataService, tagsFilter, uidFilter, hashSizeFilter, imageStreamTagAnnotationFilter, descriptionFilter, LabelFilter, $filter, $location, Logger) {
    var projectImageStreams,
        openshiftImageStreams,
        projectTemplates,
        openshiftTemplates,
        buildersByCategory = {},
        templatesByCategory = {},
        nonBuilderImages = [];

    // The tags to use for categories in the order we want to display.
    $scope.categoryTags = [
      "instant-app",
      "xpaas",
      "java",
      "php",
      "ruby",
      "perl",
      "python",
      "nodejs",
      "database",
      "messaging",
      "" // "Other" category
    ];

    // Map of tags to labels to show in the view.
    $scope.categoryLabels = {
      "instant-app": "Instant Apps",
      java: "Java",
      xpaas: "xPaaS",
      php: "PHP",
      ruby: "Ruby",
      perl: "Perl",
      python: "Python",
      nodejs: "NodeJS",
      database: "Databases",
      messaging: "Messaging",
      "": "Other"
    };

    // Category tags with items that match the current filter.
    $scope.filteredCategoryTags = [];

    // Templates that match the current filter, or all templates if no filter is set.
    // Key is the category tag, value is an array.
    $scope.filteredTemplatesByCategory = {};

    // Builders that match the current filter, or all builders if no filter is set.
    // Key is the category tag, value is an array.
    $scope.filteredBuildersByCategory = {};

    // Set to true when everything has finished loading.
    $scope.loaded = false;

    // Set to false if there is data to show.
    $scope.emptyCatalog = true;

    // The maximum number of items of type to show by default in a category.
    $scope.itemLimit = 4;

    // The current filter value.
    $scope.filter = {
      keyword: '',
      tag: ''
    };

    $scope.filterTag = function(tag) {
      $scope.filter.tag = tag;
    };

    // List templates in the project namespace as well as the shared `openshift` namespace.
    DataService.list("templates", $scope, function(templates) {
      projectTemplates = templates.by("metadata.name");
      categorizeTemplates(projectTemplates);
      updateState();
    });

    DataService.list("templates", {namespace: "openshift"}, function(templates) {
      openshiftTemplates = templates.by("metadata.name");
      categorizeTemplates(openshiftTemplates);
      updateState();
    });

    // List image streams in the project namespace as well as the shared `openshift` namespace.
    DataService.list("imagestreams", $scope, function(imageStreams) {
      projectImageStreams = imageStreams.by("metadata.name");
      categorizeImages(projectImageStreams);
      updateState();
    });

    DataService.list("imagestreams", {namespace: "openshift"}, function(imageStreams) {
      openshiftImageStreams = imageStreams.by("metadata.name");
      categorizeImages(openshiftImageStreams);
      updateState();
    });

    // Check if tag in is in the array of tags. Substring matching is optional
    // and useful for typeahead search. Typing "jav" should match tag "java".
    function hasTag(tag, tags, substrMatch) {
      var i;

      tag = tag.toLowerCase();
      for (i = 0; i < tags.length; i++) {
        var next = tags[i].toLowerCase();
        if (tag === next || (substrMatch && next.indexOf(tag) === 0)) {
          return true;
        }
      }

      return false;
    }

    function matchesKeyword(name, description, tags, keyword) {
      // Match tag substrings when comparing keywords.
      if (hasTag(keyword, tags, true)) {
        return true;
      }

      return name.toLowerCase().indexOf(keyword) !== -1 ||
        (description && description.toLowerCase().indexOf(keyword) !== -1);
    }

    // Return true only if item matches both tag and keyword filters when both are set.
    function matchesFilter(name, description, tags) {
      var keywords, i;

      if ($scope.filter.tag && !hasTag($scope.filter.tag, tags)) {
        return false;
      }

      if ($scope.filter.keyword) {
        keywords = $scope.filter.keyword.split(/\s+/);
        for (i = 0; i < keywords.length; i++) {
          if (!matchesKeyword(name, description, tags, keywords[i])) {
            return false;
          }
        }
      }

      return true;
    }

    function countItems(category) {
      var builders = $scope.filteredBuildersByCategory[category] || [],
        templates = $scope.filteredTemplatesByCategory[category] || [];

      return Math.max(builders.length, $scope.itemLimit) + Math.max(templates.length, $scope.itemLimit);
    }

    // Keep the more important categories at the top of each column, but try to
    // keep roughly the same number of visible items in each column.
    function updateColumns() {
      var numLeft = 0,
          numRight = 0,
          categories = $scope.filteredCategoryTags;

      $scope.leftCategories  = [];
      $scope.rightCategories = [];

      angular.forEach(categories, function(category) {
        if (numLeft > numRight) {
          $scope.rightCategories.push(category);
          numRight += countItems(category);
        } else {
          $scope.leftCategories.push(category);
          numLeft += countItems(category);
        }
      });
    }

    function filterCatalog() {
      $scope.filteredCategoryTags = [];
      $scope.filteredBuildersByCategory = {};
      $scope.filteredTemplatesByCategory = {};
      $scope.filteredNonBuilders = [];

      angular.forEach($scope.categoryTags, function(tag) {
        var builders = buildersByCategory[tag] || [],
            templates = templatesByCategory[tag] || [],
            filteredBuilders,
            filteredTemplates;

        filteredBuilders = builders.filter(function(builder) {
          return matchesFilter(builder.name, builder.description, builder.categoryTags);
        });
        $scope.filteredBuildersByCategory[tag] = filteredBuilders;

        filteredTemplates = templates.filter(function(template) {
          var tags = tagsFilter(template);
          return matchesFilter(template.metadata.name, descriptionFilter(template), tags);
        });
        $scope.filteredTemplatesByCategory[tag] = filteredTemplates;

        // Only add the category tag if there were any matches.
        if (filteredBuilders.length || filteredTemplates.length) {
          $scope.filteredCategoryTags.push(tag);
        }
      });

      updateColumns();

      $scope.filteredNonBuilders = nonBuilderImages.filter(function(image) {
        return matchesFilter(image.name, image.description, image.categoryTags);
      });
    }

    // Filter the catalog when the keyword or tag changes.
    $scope.$watch('filter', filterCatalog, true);

    function categorizeImages(imageStreams) {
      angular.forEach(imageStreams, function(imageStream) {
        if (!imageStream.status) {
          return;
        }

        // Create a map of spec tags so we can find them efficiently later when
        // looking at status tags.
        var specTags = {};
        if (imageStream.spec && imageStream.spec.tags) {
          angular.forEach(imageStream.spec.tags, function(tag) {
            if (tag.annotations && tag.annotations.tags) {
              specTags[tag.name] = tag.annotations.tags.split(/\s*,\s*/);
            }
          });
        }

        // Loop over status tags to categorize the images.
        angular.forEach(imageStream.status.tags, function(tag) {
          var imageStreamTag = tag.tag;
          var category;
          var categoryTags = specTags[imageStreamTag] || [];
          var image = {
            imageStream: imageStream,
            imageStreamTag: imageStreamTag,
            name: imageStream.metadata.name + ":" + imageStreamTag,
            description: imageStreamTagAnnotationFilter(imageStream, 'description', imageStreamTag),
            version: imageStreamTagAnnotationFilter(imageStream, 'version', imageStreamTag),
            categoryTags: categoryTags
          };
          if (categoryTags.indexOf("builder") >= 0) {
            // Add the builder image to its category.
            category = getCategory(categoryTags);
            buildersByCategory[category] = buildersByCategory[category] || [];
            buildersByCategory[category].push(image);
          } else {
            // Group non-builder images separately so we can hide them by default.
            nonBuilderImages.push(image);
          }
        });
      });
    }

    function categorizeTemplates(templates) {
      angular.forEach(templates, function(template) {
        var tags = tagsFilter(template);
        var category = getCategory(tags);
        templatesByCategory[category] = templatesByCategory[category] || [];
        templatesByCategory[category].push(template);
      });
    }

    function getCategory(tags) {
      var i, j;

      // Find the first matching category tag in tags.
      for (i = 0; i < $scope.categoryTags.length; i++) {
        for (j = 0; j < tags.length; j++) {
          if (tags[j].toLowerCase() === $scope.categoryTags[i]) {
            return $scope.categoryTags[i];
          }
        }
      }

      // Empty string is the "Other" category.
      return "";
    }

    function updateState() {
      // Have we finished loading all of the templates and image streams in
      // both the project and openshift namespaces? If undefined, they're not
      // loaded.
      $scope.loaded =
        projectTemplates &&
        openshiftTemplates &&
        projectImageStreams &&
        openshiftImageStreams;

      // Does anything we've loaded so far have data we show by default?
      $scope.emptyCatalog =
        hashSizeFilter(projectTemplates) === 0 &&
        hashSizeFilter(openshiftTemplates) === 0 &&
        hashSizeFilter(buildersByCategory) === 0;

      // Update filtered scope variables.
      filterCatalog();

      if ($scope.loaded) {
        Logger.info("templates by category", templatesByCategory);
        Logger.info("builder images", buildersByCategory);
        Logger.info("non-builder images", nonBuilderImages);
      }
    }
  });
