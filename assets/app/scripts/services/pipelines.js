'use strict';

angular.module("openshiftConsole")
  .factory("PipelinesService", function() {
    var Pipeline = function() {
			this.stages = {};
		};

		Pipeline.prototype.connect = function(fromStage, toStage) {
			this.setStage(fromStage);
			this.setStage(toStage);
			fromStage.connectTo(toStage);
	  };

		Pipeline.prototype.hasStage = function(stage) {
	    return this.stages[stage.id];
	  };

		Pipeline.prototype.addStage = function(stage) {
			if (!this.hasStage(stage)) {
				this.setStage(stage);
			}
	  };

		Pipeline.prototype.setStage = function(stage) {
			this.stages[stage.id] = stage;
	  };

		Pipeline.Stage = function NewStage(id, resource, kind) {
			var stage = {};
			stage.id = id;
			stage.kind = kind;
			stage.connectsTo = [];
			stage.resource = resource;
			stage.connectTo = function(stage) {
				var edge = Pipeline.Edge(stage);
				if (!_.contains(this.connectsTo, edge)) {
					this.connectsTo.push(edge);
				}
			};
			return stage;
		};

		Pipeline.Edge = function NewEdge(target, kind, status) {
			var edge = {};
			edge.kind = kind;
			edge.status = status;
			edge.target = target;
			return edge;
		};

  	return {
  		newPipeline: function() {
  			return new Pipeline();
  		},
  		newStage: function(id, resource, kind) {
  			return Pipeline.Stage(id, resource, kind);
  		}
  	};
  });
