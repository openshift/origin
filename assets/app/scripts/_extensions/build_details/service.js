'use strict';

angular.module('openshiftConsole')
  .factory('Git', function($http) {
    return {
      uri: {
        for: {
          commits: function(build) {
            var gitUri = build.spec &&
                     build.spec.source &&
                     build.spec.source.git &&
                     build.spec.source.git.uri,
            segments = URI(gitUri).suffix('').segment(),
            apiUri = URI('https://api.github.com')
                      .segment('repos');

            _.each(segments, function(segment) {
              apiUri.segment(segment);
            });
            apiUri.segment('commits');
            return apiUri;
          }
        }
      },
      get: function(apiUri) {
        return $http({
                  url: apiUri
                });
      }
    }
  });
