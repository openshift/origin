'use strict';

angular.module('openshiftConsole')
  .factory('logLinks', [
    '$anchorScroll',
    '$document',
    '$location',
    '$window',
    function($anchorScroll, $document, $location, $window) {
      // TODO (bpeterse): a lot of these functions are generic and could be moved/renamed to
      // a navigation oriented service.
      var doc = $document[0];

      var createObjectURL = function() {
        return (window.URL || window.webkitURL || {}).createObjectURL || _.noop;
      };

      var revokeObjectURL = function() {
        return (window.URL || window.webkitURL || {}).revokeObjectURL || _.noop;
      };

      var canDownload = function() {
        return !!createObjectURL();
      };

      var makeDownload = function(obj, filename) {
        obj = _.isString(obj) ? obj : JSON.stringify(obj);
        var a = doc.createElement('a');
        a.href = createObjectURL()(new Blob([obj], {type: 'text/plain'}));
        a.download = (filename || 'download') + '.txt';
        doc.body.appendChild(a);
        a.click();
        revokeObjectURL()(a.href);
        doc.body.removeChild(a);
      };

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

      var fullPageLink = function() {
         $location.search('view', 'chromeless');
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
      var chromelessLink = function() {
        newTab({view: 'chromeless'});
      };

      // new tab: path/to/current?view=textonly
      var textOnlyLink = function() {
        newTab({view: 'textonly'});
      };

      return {
        canDownload: canDownload,
        makeDownload: makeDownload,
        scrollTop: scrollTop,
        scrollBottom: scrollBottom,
        scrollTo: scrollTo,
        fullPageLink: fullPageLink,
        newTab: newTab,
        chromelessLink: chromelessLink,
        textOnlyLink: textOnlyLink
      };
    }
  ]);
