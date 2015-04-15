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
      restrict: 'A',
      scope: {
        resource: '=',
        kind: '@'
      },
      link: function(scope, elem, attrs) {
        if (scope.resource) {
          $(elem).click(function() {
            ObjectDescriber.setObject(scope.resource, scope.kind || scope.resource.kind);
          });
        }
      }
    };
  })  
  .service("ObjectDescriber", function($timeout){
    function ObjectDescriber() {
      this.resource = null;
      this.kind = null;
      this.callbacks = $.Callbacks();
    }

    ObjectDescriber.prototype.setObject = function(resource, kind) {
      this.resource = resource;
      this.kind = kind;
      var self = this;
      // queue this up to run after the current digest loop finishes
      $timeout(function(){      
        self.callbacks.fire(resource, kind);
      }, 0);
    };

    ObjectDescriber.prototype.clearObject = function() {
      this.setObject(null, null);
    };    

    // Callback will never be called within the current digest loop
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