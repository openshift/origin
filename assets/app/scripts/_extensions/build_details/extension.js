'use strict';

angular
    .module('openshiftConsole')
    .run([
        '$q',
        '$timeout',
        'extensionInput',
        'Git',
        'GitApiStub',
        function($q, $timeout, extensionInput, Git, GitApiStub) {
            var template = _.template([
                                '<div>',
                                    '<h3>Github Activity</h3>',
                                    '<table>',
                                        '<tr>',
                                            '<td>',
                                                '<strong>Latest Commit Message to source repo: &nbsp;&nbsp;&nbsp;</strong>',
                                            '</td>',
                                            '<td>',
                                                '<%= message %> &nbsp;&nbsp;',
                                            '</td>',
                                            '<td>',
                                                '<a href="<%= route %>">View more </a>',
                                            '</td>',
                                        '</tr>',
                                    '</table>',
                                '</div>'
                            ].join(''));

            extensionInput.register('build_details', _.spread(function(build, buildConfigName) {
                var uri =   build.spec &&
                            build.spec.source &&
                            build.spec.source.git &&
                            build.spec.source.git.uri,
                    route = URI('project')
                            .segment(build.metadata.namespace)
                            .segment('browse')
                            .segment('builds')
                            .segment(buildConfigName)
                            .segment(build.metadata.name)
                            .segment('gitrepo')
                            .toString();

                return uri ?
                          Git            // real API call
                          //GitApiStub   // stub, to avoid hitting github's rate limit
                            .get(Git.uri.for.commits(build))
                            .then(function(request) {
                              return {
                                type: 'html',
                                html: template({ route: route, githubUri: uri, message: _.first(request.data).commit.message })
                              };
                            }) :
                            null;
            }));
    }
  ]);
