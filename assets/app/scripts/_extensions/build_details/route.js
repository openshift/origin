'use strict';

angular.module('openshiftConsole')
    // add a route to support
    .config([
        '$routeProvider',
        function($routeProvider) {
            // TODO: register a route to a page that will show the last commits
            // for the github repo
            $routeProvider
                .when('/project/:project/browse/builds/:buildconfig/:build/gitrepo', {
                    templateUrl: 'scripts/_extensions/build_details/view.html',
                    controller: 'GitRepoController'
                })
        }
    ])
