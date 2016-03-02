'use strict';

var Pipeline = function() {
	this.stages = {};
};

Pipeline.prototype = {
	connect: function(fromStage, toStage) {
		this.setStage(fromNode);
		this.setStage(toNode);
		fromNode.connectTo(toStage);
  },
	hasStage: function(stage) {
    return this.stages[stage.id] === undefined;
  },
	addStage: function(stage) {
		if (!this.hasStage(stage)) {
			this.setStage(stage);
		}
  },
	setNode: function(stage) {
		this.stages[stage.id] = stage;
  }
};

Pipeline.Stage = function NewNode(resource, kind) {
	var stage = {};
	stage.id = "";
	stage.kind = kind;
	stage.connectsTo = [];
	stage.resource = resource;
	stage.connectTo = function(stage) {
		var edge = Pipeline.Edge(resource);
		if (!_.contains(this.connectsTo, edge)) {
			this.connectsTo.push(edge);
		}
	}
	return stage;
};

Pipeline.Edge = function NewEdge(target, kind, status) {
	var edge = {};
	edge.kind = kind;
	edge.status = status;
	edge.target = target;
	return edge;
};