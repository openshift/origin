'use strict';

angular
  .module('openshiftConsole')
  .factory('BaseHref', function($document) {
    return $document.find('base').attr('href') || '/';
  });


