'use strict';
/* jshint unused: false, eqeqeq: false */

angular.module('openshiftConsole')
  .directive('oscObjectDescriber', function(ObjectDescriber) {
    return {
      restrict: 'E',
      scope: {},
      templateUrl: 'views/directives/osc-object-describer.html',
      link: function(scope, elem, attrs) {
        var callback = ObjectDescriber.onResourceChanged(function(resource, kind) {
          scope.$apply(function() {
            scope.kind = kind;
            scope.resource = resource;
          });
        });
        scope.$on('$destroy', function() {
          ObjectDescriber.removeResourceChangedCallback(callback);
        });
      }
    };
  })
  .directive('oscObject', function(ObjectDescriber) {
    return {
      restrict: 'AC',
      scope: {
        resource: '=',
        kind: '@'
      },
      link: function(scope, elem, attrs) {
        // TODO need to make these elements selectable via the keyboard
        $(elem).on("click.oscobject", function(event) {
          // Clicks directly on a link inside osc-object elements should simply follow the link.
          if ($(event.target).closest("a").length > 0) {
            return;
          }

          if (scope.resource) {
            ObjectDescriber.setObject(scope.resource, scope.kind || scope.resource.kind, {source: scope});
            // Have to stop event propagation or nested resources will fire parent handlers
            return false;
          }
        });

        $(elem).on("mousemove.oscobject", function() {
          if (scope.resource || $(this).hasClass("osc-object-stacked")) {
            $(".osc-object-hover").not(this).removeClass("osc-object-hover");
            $(this).addClass("osc-object-hover");
            return false;
          }
        });

        $(elem).on("mouseleave.oscobject", function() {
          if (scope.resource || $(this).hasClass("osc-object-stacked")) {
            $(this).removeClass("osc-object-hover");
          }
        });

        // TODO can we be more efficient about this to reduce the number of listeners
        var resourceChangeCallback = ObjectDescriber.onResourceChanged(function(resource, kind) {
          if (resource && resource.metadata && scope.resource && scope.resource.metadata && resource.metadata.uid == scope.resource.metadata.uid) {
            $(elem).addClass("osc-object-active");
          }
          else {
            $(elem).removeClass("osc-object-active");
          }
        });

        scope.$watch('resource', function(newValue, oldValue) {
          if (ObjectDescriber.getSource() === scope) {
            ObjectDescriber.setObject(scope.resource, scope.kind || scope.resource.kind, {source: scope});
          }
        });

        scope.$on('$destroy', function() {
          ObjectDescriber.removeResourceChangedCallback(resourceChangeCallback);

          if (ObjectDescriber.getSource() === scope) {
            ObjectDescriber.clearObject();
          }
        });
      }
    };
  })
  .filter("isOscActiveObject", function(ObjectDescriber, uidFilter) {
    return function(resource) {
      var selectedResource = ObjectDescriber.getResource();
      return uidFilter(resource) === uidFilter(selectedResource);
    };
  })
  .service("ObjectDescriber", function($timeout){
    function ObjectDescriber() {
      this.resource = null;
      this.kind = null;
      this.source = null;
      this.callbacks = $.Callbacks();
    }

    ObjectDescriber.prototype.setObject = function(resource, kind, opts) {
      this.resource = resource;
      this.kind = kind;
      opts = opts || {};
      this.source = opts.source || null;
      var self = this;
      // queue this up to run after the current digest loop finishes
      $timeout(function(){
        self.callbacks.fire(resource, kind);
      }, 0);
    };

    ObjectDescriber.prototype.clearObject = function() {
      this.setObject(null, null);
    };

    ObjectDescriber.prototype.getResource = function() {
      return this.resource;
    };

    ObjectDescriber.prototype.getSource = function() {
      return this.source;
    };

    // Callback will never be called within a digest loop
    ObjectDescriber.prototype.onResourceChanged = function(callback) {
      this.callbacks.add(callback);
      var self = this;
      if (this.resource) {
        // queue this up to run after the current digest loop finishes
        $timeout(function(){
          callback(self.resource, self.kind);
        }, 0);
      }
      return callback;
    };

    ObjectDescriber.prototype.removeResourceChangedCallback = function(callback) {
      this.callbacks.remove(callback);
    };

    return new ObjectDescriber();
  });
