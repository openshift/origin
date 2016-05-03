'use strict';

angular.module('openshiftConsole')
  .directive('oscFileInput', function(Logger) {
    return {
      restrict: 'E',
      scope: {
        model: "=",
        required: "=",
        disabled: "=ngDisabled",
        // Show the file contents below the file input.
        showValues: "=",
        helpText: "@?",
        dropZoneId: "@?",
        dragging: "=",
      },
      templateUrl: 'views/directives/osc-file-input.html',
      link: function(scope, element){
        scope.helpID = _.uniqueId('help-');
        scope.supportsFileUpload = (window.File && window.FileReader && window.FileList && window.Blob);
        scope.uploadError = false;

        var droppableElement = (scope.dropZoneId) ? $('#' + scope.dropZoneId) : element,
        dropZoneName = scope.dropZoneId + "-drag-and-drop-zone",
        dropZoneSelectorName = "#" + dropZoneName,
        highlightDropZone = false,
        showDropZone = false;

        // Add/Remove dropZone based on if the directive element is disabled
        scope.$watch('disabled', function() {
          if (scope.disabled) {
            removeDropZoneElements();
          } else {
            addDropZoneToElement();
            addDropZoneListeners();            
          }
        }, true);

        // Add handler for dropping file out of the dragZone only once
        if ( _.isUndefined($._data( $(document)[0], "events")) || _.isUndefined($._data( $(document)[0], "events").drop)) {
          $(document).on('drop.oscFileInput', function() {
            removeDropZoneClasses();
            $('.drag-and-drop-zone').trigger('putDropZoneFront', false);
            return false;
          });

          $(document).on('dragenter.oscFileInput', function() {
            showDropZone = true;
            $('.drag-and-drop-zone').addClass('show-drag-and-drop-zone');
            $('.drag-and-drop-zone').trigger('putDropZoneFront', true);
            return false;
          });

          $(document).on('dragover.oscFileInput', function() {
            showDropZone = true;
            $('.drag-and-drop-zone').addClass('show-drag-and-drop-zone');
            return false;
          });

          $(document).on('dragleave.oscFileInput', function() {
            showDropZone = false;
            _.delay(function(){
              if( !showDropZone ){
                $('.drag-and-drop-zone').removeClass('show-drag-and-drop-zone');
              }
            }, 200 );
            return false;
          });
        }

        element.change(function() {
          addFile($('input[type=file]', this)[0].files[0]);
        });

        // Add listeners for the dropZone element
        function addDropZoneListeners(){
          var dropZoneElement = $(dropZoneSelectorName);

          dropZoneElement.on('dragover', function() {
            dropZoneElement.addClass('highlight-drag-and-drop-zone');
            highlightDropZone = true;
          });

          $(dropZoneSelectorName + " p").on('dragover', function() {
            highlightDropZone = true;
          });

          dropZoneElement.on('dragleave', function() {
            highlightDropZone = false;
            _.delay(function(){
              if (!highlightDropZone) {
                dropZoneElement.removeClass('highlight-drag-and-drop-zone');
              }
            }, 200 );
          });

          dropZoneElement.on('drop', function(e) {
            var files = _.get(e, 'originalEvent.dataTransfer.files', []);
            if (files.length > 0 ) {
              scope.file = _.head(files);
              addFile(scope.file);
            }
            removeDropZoneClasses();
            $('.drag-and-drop-zone').trigger('putDropZoneFront', false);
            $('.drag-and-drop-zone').trigger('reset');
            return false;
          });

          dropZoneElement.on('putDropZoneFront', function(event, putFront) {
            if (putFront) {
              $(dropZoneSelectorName).width(droppableElement.outerWidth()).height(droppableElement.outerHeight()).css('z-index', 100);
            } else {
              $(dropZoneSelectorName).css('z-index', -1);
            }       
            return false;
          });

          dropZoneElement.on('reset', function() {
            showDropZone = false;
            return false;
          });
        }

        // Add drop zone before the droppable element
        function addDropZoneToElement() {
          droppableElement.before('<div id=' + dropZoneName + ' class="drag-and-drop-zone"><p>Drop file here</p></div>');
          element.css('z-index', 50);
        }

        function addFile(file) {
          var reader = new FileReader();
          reader.onloadend = function(){
            scope.$apply(function(){
              scope.fileName = file.name;
              scope.model = reader.result;
            });
          };
          reader.onerror = function(e){
            scope.supportsFileUpload = false;
            scope.uploadError = true;
            Logger.error("Could not read file", e);
          };
          reader.readAsText(file);
        }

        function removeDropZoneClasses(){
          $('.drag-and-drop-zone').removeClass("show-drag-and-drop-zone highlight-drag-and-drop-zone");
        }

        function removeDropZoneElements() {
          $(dropZoneSelectorName).remove();
        }

        scope.$on('$destroy', function(){
          $(dropZoneSelectorName).off();
          $(document).off('drop.oscFileInput')
            .off('dragenter.oscFileInput')
            .off('dragover.oscFileInput')
            .off('dragleave.oscFileInput');
        });
      }
    };
  });
