'use strict';

angular.module('openshiftConsole')
  .factory('GitApiStub', function($q) {
    return {
      get: function() {
        // Fake data of same format as Github's api, but with some dummy values injected
        return $q.when({
                  data: [
                    {
                        "sha": "123456789012345",
                        "commit": {
                          "author": {
                            "name": "John Doe",
                            "email": "jdoe@users.noreply.github.com",
                            "date": "2015-12-02T12:54:13Z"
                          },
                          "message": "Merge pull request #49 from jdoe/bump_version\n\nmove hello world example to ruby-22 image",
                          "url": "https://api.github.com/repos/jdoe/stuff-and-things/git/commits/123456789012345",
                          "comment_count": 0
                        },
                        "url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/123456789012345",
                        "html_url": "https://github.com/jdoe/stuff-and-things/commit/123456789012345",
                        "comments_url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/123456789012345/comments",
                        "author": {
                          "login": "jdoe",
                          "id": 1234567,
                          "avatar_url": "https://avatars.githubusercontent.com/u/1234567?v=3",
                          "gravatar_id": "",
                          "url": "https://api.github.com/users/jdoe",
                          "html_url": "https://github.com/jdoe",
                          "repos_url": "https://api.github.com/users/jdoe/repos",
                          "events_url": "https://api.github.com/users/jdoe/events{/privacy}",
                          "received_events_url": "https://api.github.com/users/jdoe/received_events",
                          "type": "User",
                          "site_admin": false
                        },
                        "committer": {
                          "login": "jdoe",
                          "id": 1234567,
                          "avatar_url": "https://avatars.githubusercontent.com/u/1234567?v=3",
                          "gravatar_id": "",
                          "url": "https://api.github.com/users/jdoe",
                          "html_url": "https://github.com/jdoe",
                          "repos_url": "https://api.github.com/users/jdoe/repos",
                          "type": "User"
                        },
                        "parents": [
                          {
                            "sha": "12345123451234512345",
                            "url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/12345123451234512345",
                            "html_url": "https://github.com/jdoe/stuff-and-things/commit/12345123451234512345"
                          },
                          {
                            "sha": "09876543210987654321",
                            "url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/09876543210987654321",
                            "html_url": "https://github.com/jdoe/stuff-and-things/commit/09876543210987654321"
                          }
                        ]
                      },{
                        "sha": "123456789012345",
                        "commit": {
                          "author": {
                            "name": "John Doe",
                            "email": "jdoe@users.noreply.github.com",
                            "date": "2015-12-02T12:54:13Z"
                          },
                          "message": "Merge pull request #49 from jdoe/bump_version\n\nmove hello world example to ruby-22 image",
                          "url": "https://api.github.com/repos/jdoe/stuff-and-things/git/commits/123456789012345",
                          "comment_count": 0
                        },
                        "url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/123456789012345",
                        "html_url": "https://github.com/jdoe/stuff-and-things/commit/123456789012345",
                        "comments_url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/123456789012345/comments",
                        "author": {
                          "login": "jdoe",
                          "id": 1234567,
                          "avatar_url": "https://avatars.githubusercontent.com/u/1234567?v=3",
                          "gravatar_id": "",
                          "url": "https://api.github.com/users/jdoe",
                          "html_url": "https://github.com/jdoe",
                          "repos_url": "https://api.github.com/users/jdoe/repos",
                          "events_url": "https://api.github.com/users/jdoe/events{/privacy}",
                          "received_events_url": "https://api.github.com/users/jdoe/received_events",
                          "type": "User",
                          "site_admin": false
                        },
                        "committer": {
                          "login": "jdoe",
                          "id": 1234567,
                          "avatar_url": "https://avatars.githubusercontent.com/u/1234567?v=3",
                          "gravatar_id": "",
                          "url": "https://api.github.com/users/jdoe",
                          "html_url": "https://github.com/jdoe",
                          "repos_url": "https://api.github.com/users/jdoe/repos",
                          "type": "User"
                        },
                        "parents": [
                          {
                            "sha": "12345123451234512345",
                            "url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/12345123451234512345",
                            "html_url": "https://github.com/jdoe/stuff-and-things/commit/12345123451234512345"
                          },
                          {
                            "sha": "09876543210987654321",
                            "url": "https://api.github.com/repos/jdoe/stuff-and-things/commits/09876543210987654321",
                            "html_url": "https://github.com/jdoe/stuff-and-things/commit/09876543210987654321"
                          }
                        ]
                      }
                    ]
                });
    }
  }
});
