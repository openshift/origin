'use strict';

angular.module('openshiftConsole')
  .factory('logLinks', [
    '$anchorScroll',
    '$document',
    '$location',
    '$window',
    function($anchorScroll, $document, $location, $window) {
      var doc = $document[0];
      return {
        makeDownload: function(obj) {
          var a = doc.createElement('a');
          a.href = $window.URL.createObjectURL(new Blob([obj], {type: 'text/plain'}));
          a.download = 'log.txt';
          doc.body.appendChild(a);
          a.click();
          doc.body.removeChild(a);
        },
        scrollTop: function() {
          $window.scrollTo(null, 0);
        },
        scrollBottom: function() {
          $window.scrollTo(null, $document.innerHeight());
        },
        scrollTo: function(anchor, event) {
          // sad face. stop reloading the page!!!!
          event.preventDefault();
          event.stopPropagation();
          $location.hash(anchor);
          $anchorScroll(anchor);
        },
        fullPageLink: function() {
         $location
          .path($location.path())
          .search(
            angular.extend($location.search(), {
              view: 'chromeless'
          }));
        },
        chromelessLink: function() {
          $window
            .open([
              $location.path(),
              '?',
              $.param(
                angular
                  .extend(
                    $location.search(), {
                      view: 'chromeless'
                    }))
            ].join(''), '_blank');
        },
        // @deprecated, see above 'makeDownload',
        // will likely remove
        textOnlyLink: function() {
          $window
            .open([
              $location.path(),
              '?',
              $.param(
                angular
                  .extend(
                    $location.search(), {
                      view: 'textonly'
                    }))
            ].join(''), '_blank');
        }
      }
    }
  ]);
