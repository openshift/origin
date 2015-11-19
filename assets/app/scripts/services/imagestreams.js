'use strict';

angular.module("openshiftConsole")
  .factory("ImageStreamsService", function(){
    return {
      tagsByName: function(imageStream) {
        var tagsByName = {};
        angular.forEach(imageStream.spec.tags, function(tag){
          tagsByName[tag.name] = tagsByName[tag.name] || {};
          tagsByName[tag.name].name = tag.name;
          tagsByName[tag.name].spec = angular.copy(tag);

          // Split the name into useful parts, recalculate name to a common form to
          // include self-references within the same IS
          var from = tagsByName[tag.name].spec.from;
          if (from) {
            var splitChar = "";
            if (from.kind === "ImageStreamImage") {
              splitChar = "@";
            }
            else if (from.kind === "ImageStreamTag") {
              splitChar = ":";
            }
            from._nameConnector = splitChar || null;
            var parts = from.name.split(splitChar);
            if (parts.length === 1) {
              from._imageStreamName = imageStream.metadata.name;
              from._idOrTag = parts[0];
              from._completeName = from._imageStreamName + splitChar + from._idOrTag;
            }
            else {
              from._imageStreamName = parts.shift();
              from._idOrTag = parts.join(splitChar); // in case for some reason there was another @ symbol in the rest
              from._completeName = from._imageStreamName + splitChar + from._idOrTag;
            }
          }
        });
        angular.forEach(imageStream.status.tags, function(tag){
          tagsByName[tag.tag] = tagsByName[tag.tag] || {};
          tagsByName[tag.tag].name = tag.tag;
          tagsByName[tag.tag].status = angular.copy(tag);
        });

        return tagsByName;
      }
    };
  });
