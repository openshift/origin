'use strict';

angular.module('openshiftConsole')
  .factory('logUtils', [
    function() {
      return {
        addLineNumbers: function(str, padding) {
          padding = padding || 7;
          return str ?
                    _.reduce(
                      str.split('\n'),
                      function(memo, next, i, list) {
                        return (i < list.length) ?
                                  memo + _.padRight(i+1+'. ', padding) + next + '\n' :
                                  memo;
                      },'') :
                    'Error retrieving log';
        },
        toList: function(str) {
          return str ?
                  _.map(
                    str.split('\n'),
                    function(text) {
                      return {
                        text: text
                      }
                    }) :
                  [{text: 'Error retrieving log'}];
        },
        toStr: function(arr) {
          return _.reduce(
                    arr,
                    function(memo, next, i, list) {
                      return i <= arr.length ?
                              memo + next.text :
                              memo;
                    }, '');
        }
      }
    }
  ]);
