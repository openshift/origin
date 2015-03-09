'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:LabelsController
 * @description
 * # LabelsController
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('LabelsController', function ($scope) {
     $scope.expanded = true;
     $scope.toggleExpanded = function() {
       $scope.expanded = !$scope.expanded;
     };
     $scope.addLabel = function() {
       if ($scope.labelKey && $scope.labelValue) {
         $scope.labels[$scope.labelKey] = $scope.labelValue;
         $scope.labelKey = "";
         $scope.labelValue = "";
         $scope.form.$setPristine();
         $scope.form.$setUntouched();
       }
     };
     $scope.deleteLabel = function(key) {
       if ($scope.labels[key]) {
         delete $scope.labels[key];
       }
     };
  });
