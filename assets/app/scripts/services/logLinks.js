'use strict';

angular.module('openshiftConsole')
  .factory('logLinks', [
    '$anchorScroll',
    '$document',
    '$location',
    '$timeout',
    '$window',
    function($anchorScroll, $document, $location, $timeout, $window) {
      // TODO (bpeterse): a lot of these functions are generic and could be moved/renamed to
      // a navigation oriented service.

      var scrollTop = function() {
        $window.scrollTo(null, 0);
      };

      var scrollBottom = function() {
        $window.scrollTo(null, $document.innerHeight());
      };

      var scrollTo = function(anchor, event) {
        // sad face. stop reloading the page!!!!
        event.preventDefault();
        event.stopPropagation();
        $location.hash(anchor);
        $anchorScroll(anchor);
      };

      // @params an object or array of objects
      var newTab = function(params) {
        params = _.isArray(params) ?
                  params :
                  [params];
        var uri = new URI();
        _.each(params, function(param) {
          uri.addSearch(param);
        });
        $window.open(uri.toString(), '_blank');
      };

      // new tab: path/to/current?view=chromeless
      var chromelessLink = function(options) {
        var params = {
          view: 'chromeless'
        };
        if (options && options.container) {
          params.container = options.container;
        }
        newTab(params);
      };

      // Not currently used.
      // new tab: path/to/current?view=textonly
      var textOnlyLink = function() {
        newTab({view: 'textonly'});
      };

      return {
        scrollTop: scrollTop,
        scrollBottom: scrollBottom,
        scrollTo: scrollTo,
        newTab: newTab,
        chromelessLink: chromelessLink,
        textOnlyLink: textOnlyLink
      };
    }
  ]);
