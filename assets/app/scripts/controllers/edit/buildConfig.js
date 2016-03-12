'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:EditBuildConfigController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('EditBuildConfigController', function ($scope, $routeParams, DataService, ProjectsService, $filter, ApplicationGenerator, Navigate, $location, AlertMessageService, SOURCE_URL_PATTERN) {

    $scope.projectName = $routeParams.project;
    $scope.buildConfig = null;
    $scope.alerts = {};
    $scope.emptyMessage = "Loading...";
    $scope.sourceURLPattern = SOURCE_URL_PATTERN;
    $scope.options = {};
    $scope.builderOptions = {};
    $scope.outputOptions = {};
    $scope.imageSourceOptions = {};
    $scope.selectTypes = {
      ImageStreamTag: "Image Stream Tag",
      ImageStreamImage: "Image Stream Image",
      DockerImage: "Docker Image Link"
    };
    $scope.buildFromTypes = [
      {
        "id": "ImageStreamTag",
        "title": "Image Stream Tag"
      },
      {
        "id": "ImageStreamImage",
        "title": "Image Stream Image"
      },
      {
        "id": "DockerImage",
        "title": "Docker Image Link"
      }
    ];
    $scope.pushToTypes = [
      {
        "id": "ImageStreamTag",
        "title": "Image Stream Tag"
      },
      {
        "id": "DockerImage",
        "title": "Docker Image Link"
      },
      {
        "id": "None",
        "title": "--- None ---"
      }
    ];
    $scope.breadcrumbs = [
      {
        title: $routeParams.project,
        link: "project/" + $routeParams.project
      },
      {
        title: "Builds",
        link: "project/" + $routeParams.project + "/browse/builds"
      },
      {
        title: $routeParams.buildconfig,
        link: "project/" + $routeParams.project + "/browse/builds/" + $routeParams.buildconfig
      },
      {
        title: "Edit"
      }
    ];
    $scope.buildFrom = {
      projects: [],
      imageStreams: [],
      tags: {},
    };
    $scope.pushTo = {
      projects: [],
      imageStreams: [],
      tags: {},
    };
    $scope.sources = {
      "binary": false,
      "dockerfile": false,
      "git": false,
      "images": false,
      "contextDir": false
    };
    // $scope.triggers.present.imageChange points to builder imageChange trigger.
    $scope.triggers = {
      present: {
        "webhook": false,
        "imageChange": false,
        "configChange": false
      },
      builderImageChangeTrigger: {},
      imageChangeTriggers: []
    };
    $scope.availableProjects = [];

    AlertMessageService.getAlerts().forEach(function(alert) {
      $scope.alerts[alert.name] = alert.data;
    });
    AlertMessageService.clearAlerts();
    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;

        // Update project breadcrumb with display name.
        $scope.breadcrumbs[0].title = $filter('displayName')(project);

        DataService.get("buildconfigs", $routeParams.buildconfig, context).then(
          // success
          function(buildConfig) {
            $scope.buildConfig = buildConfig;
            $scope.updatedBuildConfig = angular.copy($scope.buildConfig);
            $scope.buildStrategy = $filter('buildStrategy')($scope.updatedBuildConfig);
            $scope.strategyType = $scope.buildConfig.spec.strategy.type;
            $scope.envVars = $filter('envVarsPair')($scope.buildStrategy.env);
            $scope.triggers = $scope.getTriggerMap($scope.triggers, $scope.buildConfig.spec.triggers);
            $scope.sources = $scope.getSourceMap($scope.sources, $scope.buildConfig.spec.source);

            if ($scope.buildStrategy.from) {
              var buildFrom = $scope.buildStrategy.from;
              $scope.builderOptions = $scope.setPickedVariables(
                $scope.builderOptions,
                buildFrom.kind,
                buildFrom.namespace || buildConfig.metadata.namespace, 
                buildFrom.name.split(":")[0],
                buildFrom.name.split(":")[1],
                (buildFrom.kind === "ImageStreamImage") ? buildFrom.name : "",
                (buildFrom.kind === "ImageStreamTag") ? buildConfig.metadata.namespace + "/" + buildFrom.name : buildFrom.name);
            } else {
              $scope.builderOptions = $scope.setPickedVariables($scope.builderOptions, "None", buildConfig.metadata.namespace, "", "", "", "");
            }

            if ($scope.updatedBuildConfig.spec.output.to) {
              var pushTo = $scope.updatedBuildConfig.spec.output.to
              $scope.outputOptions = $scope.setPickedVariables(
                $scope.outputOptions,
                pushTo.kind,
                pushTo.namespace || buildConfig.metadata.namespace,
                pushTo.name.split(":")[0],
                pushTo.name.split(":")[1],
                undefined,
                (pushTo.kind === "ImageStreamTag") ? buildConfig.metadata.namespace + "/" + pushTo.name : pushTo.name);
            } else {
              $scope.outputOptions = $scope.setPickedVariables($scope.outputOptions, "None", buildConfig.metadata.namespace, "", "", undefined, "");
            }

            $scope.builderImageStream = {
              namespace: $scope.builderOptions.pickedNamespace,
              imageStream: $scope.builderOptions.pickedImageStream,
              tag: $scope.builderOptions.pickedTag,
            };

            $scope.outputImageStream = {
              namespace: $scope.outputOptions.pickedNamespace,
              imageStream: $scope.outputOptions.pickedImageStream,
              tag: $scope.outputOptions.pickedTag,
            };

            $scope.options.forcePull = !!$scope.buildStrategy.forcePull;

            if ($scope.sources.images) {
              $scope.sourceImages = $scope.buildConfig.spec.source.images;
              // If only one Image Source is present in the buildConfig make it editable, if more then one show them as readonly.
              if ($scope.sourceImages.length === 1) {
                $scope.sourceImage = $scope.buildConfig.spec.source.images[0];
                // Initialize structure in the same way builder and output image.
                $scope.imageSourceBuildFrom = {
                  projects: [],
                  imageStreams: [],
                  tags: {},
                };
                $scope.imageSourcePaths = $filter('destinationSourcePair')($scope.sourceImage.paths);
                $scope.imageSourceTypes = angular.copy($scope.buildFromTypes);
                var imageSourceFrom = $scope.sourceImage.from;
                $scope.imageSourceOptions = $scope.setPickedVariables(
                    $scope.imageSourceOptions,
                    imageSourceFrom.kind,
                    imageSourceFrom.namespace || buildConfig.metadata.namespace,
                    imageSourceFrom.name.split(":")[0],
                    imageSourceFrom.name.split(":")[1],
                    (imageSourceFrom.kind === "ImageStreamImage") ? imageSourceFrom.name : "",
                    (imageSourceFrom.kind === "ImageStreamTag") ? buildConfig.metadata.namespace + "/" + imageSourceFrom.name : imageSourceFrom.name);

                // Save loaded value in case namespace, imageStream or tag are not available, to prevent data loss.
                $scope.imageSourceImageStream = {
                  namespace: $scope.imageSourceOptions.pickedNamespace,
                  imageStream: $scope.imageSourceOptions.pickedImageStream,
                  tag: $scope.imageSourceOptions.pickedTag,
                };
              } else {
                $scope.imageSourceFromObjects = [];
                $scope.sourceImages.forEach(function(sourceImage) {
                  $scope.imageSourceFromObjects.push(sourceImage.from);
                });
              }
            }

            if ($scope.sources.binary) {
              $scope.options.binaryAsFile = ($scope.buildConfig.spec.source.binary.asFile) ? $scope.buildConfig.spec.source.binary.asFile : "";
            }

            if ($scope.strategyType === "Docker") {
              $scope.options.noCache = !!$scope.buildConfig.spec.strategy.dockerStrategy.noCache;
              // Only DockerStrategy can have empty Strategy object and therefore it's from object
              $scope.buildFromTypes.push({"id": "None", "title": "--- None ---"});
            }

            $scope.buildFrom.projects = ["openshift"];
            DataService.list("projects", $scope, function(projects) {
              var projects = projects.by("metadata.name");
              for (var name in projects) {
                $scope.buildFrom.projects.push(name);
                $scope.pushTo.projects.push(name);
              }
              $scope.availableProjects = angular.copy($scope.buildFrom.projects);

              // If builder or output image namespace is not part of users available namespaces, add it to 
              // the namespace array anyway together with and call that checks the availability of the namespace.
              if (!$scope.buildFrom.projects.contains($scope.builderOptions.pickedNamespace)) {
                $scope.checkNamespaceAvailability($scope.builderOptions.pickedNamespace);
                $scope.buildFrom.projects.push($scope.builderOptions.pickedNamespace);
              }
              if (!$scope.pushTo.projects.contains($scope.outputOptions.pickedNamespace)) {
                $scope.checkNamespaceAvailability($scope.outputOptions.pickedNamespace);
                $scope.pushTo.projects.push($scope.outputOptions.pickedNamespace);
              }

              // If builder or output image reference kind is DockerImage select the first imageSteam and imageStreamTag
              // in the picker, so when the user changes the reference kind to ImageStreamTag the picker is filled with
              // default(first) value.
              if ($scope.builderOptions.pickedType === "ImageStreamTag") {
                $scope.updateBuilderImageStreams($scope.builderOptions.pickedNamespace, false);
              }

              if ($scope.outputOptions.pickedType === "ImageStreamTag") {
                $scope.updateOutputImageStreams($scope.outputOptions.pickedNamespace, false);
              }

              if ($scope.sources.images && $scope.sourceImage) {
                $scope.imageSourceBuildFrom.projects = angular.copy($scope.buildFrom.projects);
                if (!$scope.imageSourceBuildFrom.projects.contains($scope.imageSourceOptions.pickedNamespace)) {
                  $scope.checkNamespaceAvailability($scope.imageSourceOptions.pickedNamespace);
                  $scope.imageSourceBuildFrom.projects.push($scope.imageSourceOptions.pickedNamespace);
                }
                if ($scope.imageSourceOptions.pickedType === "ImageStreamTag") {
                  $scope.updateImageSourceImageStreams($scope.imageSourceOptions.pickedNamespace, false);                
                }
              }
              $scope.loaded = true;
            });
            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("buildconfigs", $routeParams.buildconfig, context, function(buildConfig, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: "This build configuration has been deleted."
                };
                $scope.disableInputs = true;
              }
              $scope.buildConfig = buildConfig;
            }));
          },
          // failure
          function(e) {
            $scope.loaded = true;
            $scope.alerts["load"] = {
              type: "error",
              message: "The build configuration details could not be loaded.",
              details: "Reason: " + $filter('getErrorDetails')(e)
            };
          }
        );
      })
    );

    $scope.getTriggerMap = function(triggerMap, triggers) {

      // Even if `from` is set in the image change trigger check if its not pointing to the builder image
      function isBuilder(imageChangeFrom, buildConfigFrom) {
        var imageChangeRef = $filter('imageObjectRef')(imageChangeFrom, $scope.projectName);
        var builderRef = $filter('imageObjectRef')(buildConfigFrom, $scope.projectName);
        return imageChangeRef === builderRef
      };
      var buildConfigFrom = $filter('buildStrategy')($scope.buildConfig).from;

      triggers.forEach(function(trigger) {
        switch (trigger.type) {
          case "Generic":
            break;
          case "GitHub":
            triggerMap.present.webhook = true;
            break;
          case "ImageChange":
            var imageChangeFrom = trigger.imageChange.from;
            if (!imageChangeFrom) {
              imageChangeFrom = buildConfigFrom;
            }
            if (isBuilder(imageChangeFrom, buildConfigFrom)) {
              triggerMap.present.imageChange = true;
              triggerMap.builderImageChangeTrigger = trigger;
            }
            triggerMap.imageChangeTriggers.push(trigger);
            break;
          case "ConfigChange":
            triggerMap.present.configChange = true;
            break;
        }
      });

      // If the builder imageChange trigger is not present, pre-populate the imageChangeTriggers array with it
      // and set the builderImageChangeTrigger object with it.
      if (_.isEmpty(triggerMap.builderImageChangeTrigger)) {
        var builderTrigger = {
          imageChange: {},
          type: "ImageChange"
        };
        triggerMap.imageChangeTriggers.push(builderTrigger);
        triggerMap.builderImageChangeTrigger = builderTrigger;
      }
      return triggerMap;
    };

    $scope.setPickedVariables = function(pickedOptions, type, ns, is, ist, isi, di) {
      pickedOptions.pickedType = type;
      pickedOptions.pickedNamespace = ns;
      pickedOptions.pickedImageStream = is;
      pickedOptions.pickedTag = ist;
      if (isi) {
        pickedOptions.pickedImageStreamImage = isi;
      }
      pickedOptions.pickedDockerImage = di;
      return pickedOptions;
    };

    // When BuildFrom/PushTo/ImageBuildFrom type is change, appeared fields need to be filled with proper values.
    $scope.assambleInputType = function(type, pickedType) {
      switch (type) {
        case "builder":
          if ( pickedType === "DockerImage") {
            $scope.builderOptions.pickedDockerImage = $scope.builderOptions.pickedNamespace + "/" + $scope.builderOptions.pickedImageStream + ":" + $scope.builderOptions.pickedTag;
          } else if ( pickedType === "ImageStreamTag") {
            $scope.builderOptions.pickedTag = "";
            $scope.updateBuilderImageStreams($scope.builderOptions.pickedNamespace, true);
          }
          break;
        case "output":
          if ( pickedType === "DockerImage") {
            $scope.outputOptions.pickedDockerImage = $scope.outputOptions.pickedNamespace + "/" + $scope.outputOptions.pickedImageStream + ":" + $scope.outputOptions.pickedTag;
          } else if ( pickedType === "ImageStreamTag") {
            $scope.updateOutputImageStreams($scope.outputOptions.pickedNamespace, true);
          }
          break;
        case "imageSource":
          if ( pickedType === "DockerImage") {
            $scope.imageSourceOptions.pickedDockerImage = $scope.imageSourceOptions.pickedNamespace + "/" + $scope.imageSourceOptions.pickedImageStream + ":" + $scope.imageSourceOptions.pickedTag;
          } else if ( pickedType === "ImageStreamTag") {
            $scope.updateImageSourceImageStreams($scope.imageSourceOptions.pickedNamespace, true);
          }
          break;
      }
    };

    $scope.aceLoaded = function(editor) {
      var session = editor.getSession();
      session.setOption('tabSize', 2);
      session.setOption('useSoftTabs', true);
      editor.$blockScrolling = Infinity;
    };

    // updateImageSourceImageStreams creates/updates the list of imageStreams and imageStreamTags for the builder image from picked namespace.
    // As parameter takes the picked namespace, selectFirstOption as a boolean that indicates whether the imageStreams and imageStreamTags
    // selectboxes should be select the first option.
    $scope.updateImageSourceImageStreams = function(projectName, selectFirstOption) {
      if (!$scope.availableProjects.contains(projectName)) {
        $scope.imageSourceBuildFrom.imageStreams = [];
        $scope.imageSourceBuildFrom.tags = {};
        $scope.imageSourceBuildFrom.imageStreams.push($scope.imageSourceImageStream.imageStream);
        $scope.imageSourceOptions.pickedImageStreamImage = $scope.imageSourceImageStream.imageStream;
        $scope.imageSourceBuildFrom.tags[$scope.imageSourceImageStream.imageStream] = [$scope.imageSourceImageStream.tag];
        $scope.imageSourceOptions.pickedTag = $scope.imageSourceImageStream.tag;
      } else {
        DataService.list("imagestreams", {namespace: projectName}, function(imageStreams) {
          $scope.imageSourceBuildFrom.imageStreams = [];
          $scope.imageSourceBuildFrom.tags = {};
          var projectImageStreams = imageStreams.by("metadata.name");
          if (!_.isEmpty(projectImageStreams)) {
            if (!_.has(projectImageStreams, $scope.imageSourceBuildFrom.imageStream) && projectName === $scope.imageSourceBuildFrom.namespace && !selectFirstOption) {
              $scope.imageSourceBuildFrom.imageStreams.push($scope.imageSourceImageStream.imageStream);
              $scope.imageSourceOptions.pickedImageStream = $scope.imageSourceImageStream.imageStream;
              $scope.imageSourceBuildFrom.tags[$scope.imageSourceImageStream.imageStream] = [$scope.imageSourceImageStream.tag];
              $scope.imageSourceOptions.pickedTag = $scope.imageSourceImageStream.tag;
            }
            angular.forEach(projectImageStreams, function(imageStream, name) {
              var tagList = [];
              if (imageStream.status.tags) {
                imageStream.status.tags.forEach(function(item){
                  tagList.push(item["tag"]);
                });
              }
              $scope.imageSourceBuildFrom.imageStreams.push(name);
              // If the namespace and the imageStream are matching with the picked one, add the unavailable tag to the tagList.
              // Prevents losing of data, while build is still taking place and the tag for the imageSource imageStream is not available.
              if (projectName === $scope.imageSourceBuildFrom.namespace && imageStream.metadata.name === $scope.imageSourceBuildFrom.imageStream && _.indexOf(tagList, $scope.imageSourceBuildFrom.tag) === -1) {
                tagList.push($scope.imageSourceBuildFrom.tag);
              }
              $scope.imageSourceBuildFrom.tags[name] = tagList;
              // If ImageStream doesn't have any tags, set tag to empty string, so the user has to pick from a existing Tags.
              if (name === $scope.imageSourceOptions.pickedImageStream && _.isEmpty(tagList)) {
                $scope.imageSourceOptions.pickedTag = "";
              }
            });
            // If defined Image Stream is not present in the Image Streams of picked Namespace set tag to empty string,
            // so the user has to pick from a existing Image Streams.
            if (!$scope.imageSourceBuildFrom.imageStreams.contains($scope.imageSourceOptions.pickedImageStream)) {
              $scope.imageSourceOptions.pickedTag = "";
            }
            if (selectFirstOption) {
              $scope.imageSourceOptions.pickedImageStream = $scope.imageSourceBuildFrom.imageStreams[0];
              $scope.clearSelectedTag($scope.imageSourceOptions, $scope.imageSourceBuildFrom.tags);
            }
          } else if (projectName === $scope.outputImageStream.namespace && !selectFirstOption) {
            $scope.imageSourceBuildFrom.imageStreams.push($scope.imageSourceImageStream.imageStream);
            $scope.imageSourceOptions.pickedImageStream = $scope.imageSourceImageStream.imageStream;
            $scope.imageSourceBuildFrom.tags[$scope.imageSourceImageStream.imageStream] = [$scope.imageSourceImageStream.tag];
            $scope.imageSourceOptions.pickedTag = $scope.imageSourceImageStream.tag;
          } else {
            $scope.imageSourceOptions.pickedImageStream = "";
            $scope.imageSourceOptions.pickedTag = "";
          }
        });
      }
    };

    // updateBuilderImageStreams creates/updates the list of imageStreams and imageStreamTags for the builder image from picked namespace.
    // As parameter takes the picked namespace, selectFirstOption as a boolean that indicates whether the imageStreams and imageStreamTags
    // selectboxes should be select the first option.
    $scope.updateBuilderImageStreams = function(projectName, selectFirstOption) {
      if (!$scope.availableProjects.contains(projectName)) {
        $scope.buildFrom.imageStreams = [];
        $scope.buildFrom.tags = {};
        $scope.buildFrom.imageStreams.push($scope.builderImageStream.imageStream);
        $scope.builderOptions.pickedImageStream = $scope.builderImageStream.imageStream;
        $scope.buildFrom.tags[$scope.builderImageStream.imageStream] = [$scope.builderImageStream.tag];
        $scope.builderOptions.pickedTag = $scope.builderImageStream.tag;
      } else {
        DataService.list("imagestreams", {namespace: projectName}, function(imageStreams) {
          $scope.buildFrom.imageStreams = [];
          $scope.buildFrom.tags = {};
          var projectImageStreams = imageStreams.by("metadata.name");
          if (!_.isEmpty(projectImageStreams)) {
            if (!_.has(projectImageStreams, $scope.builderImageStream.imageStream) && projectName === $scope.builderImageStream.namespace && !selectFirstOption) {
              $scope.buildFrom.imageStreams.push($scope.builderImageStream.imageStream);
              $scope.builderOptions.pickedImageStream = $scope.builderImageStream.imageStream;
              $scope.buildFrom.tags[$scope.builderImageStream.imageStream] = [$scope.builderImageStream.tag];
              $scope.builderOptions.pickedTag = $scope.builderImageStream.tag;
            }
            angular.forEach(projectImageStreams, function(imageStream, name) {
              var tagList = [];
              if (imageStream.status.tags) {
                imageStream.status.tags.forEach(function(item){
                  tagList.push(item["tag"]);
                });
              }
              $scope.buildFrom.imageStreams.push(name);
              // If the namespace and the imageStream are matching with the picked one, add the unavailable tag to the tagList.
              // Prevents losing of data, while build is still taking place and the tag for the output imageStream is not available.
              if (projectName === $scope.builderImageStream.namespace && imageStream.metadata.name === $scope.builderImageStream.imageStream && _.indexOf(tagList, $scope.builderImageStream.tag) === -1) {
                tagList.push($scope.builderImageStream.tag);
              }

              $scope.buildFrom.tags[name] = tagList;
              // If ImageStream doesn't have any tags, set tag to empty string, so the user has to pick from a existing Tags.
              if (name === $scope.builderOptions.pickedImageStream &&_.isEmpty(tagList)) {
                $scope.builderOptions.pickedTag = "";
              }
            });
            // If defined Image Stream is not present in the Image Streams of picked Namespace set tag to empty string,
            // so the user has to pick from a existing Image Streams.
            if (!$scope.buildFrom.imageStreams.contains($scope.builderOptions.pickedImageStream)) {
              $scope.builderOptions.pickedTag = "";
            }
            if (selectFirstOption) {
              $scope.builderOptions.pickedImageStream = $scope.buildFrom.imageStreams[0];
              $scope.clearSelectedTag($scope.builderOptions, $scope.buildFrom.tags);
            }
          } else if (projectName === $scope.builderImageStream.namespace && !selectFirstOption) {
            $scope.buildFrom.imageStreams.push($scope.builderImageStream.imageStream);
            $scope.builderOptions.pickedImageStream = $scope.builderImageStream.imageStream;
            $scope.buildFrom.tags[$scope.builderImageStream.imageStream] = [$scope.builderImageStream.tag];
            $scope.builderOptions.pickedTag = $scope.builderImageStream.tag;
          } else {
            $scope.builderOptions.pickedImageStream = "";
            $scope.builderOptions.pickedTag = "";
          }
        });
      }
    };

    // updateOutputImageStreams creates/updates the list of imageStreams and imageStreamTags for the output image from picked namespace.
    // As parameter takes the picked namespace, selectFirstOption as a boolean that indicates whether the imageStreams and imageStreamTags
    // selectboxes should be select the first option.
    $scope.updateOutputImageStreams = function(projectName, selectFirstOption) {
      if (!$scope.availableProjects.contains(projectName)) {
        $scope.pushTo.imageStreams = [];
        $scope.pushTo.tags = {};
        $scope.pushTo.imageStreams.push($scope.outputImageStream.imageStream);
        $scope.outputOptions.pickedImageStream = $scope.outputImageStream.imageStream;
        $scope.outputOptions.pickedTag = $scope.outputImageStream.tag;
      } else {
        DataService.list("imagestreams", {namespace: projectName}, function(imageStreams) {
          $scope.pushTo.imageStreams = [];
          $scope.pushTo.tags = {};
          var projectImageStreams = imageStreams.by("metadata.name");
          if (!_.isEmpty(projectImageStreams)) {
            if (!_.has(projectImageStreams, $scope.outputImageStream.imageStream) && projectName === $scope.outputImageStream.namespace && !selectFirstOption) {
              $scope.pushTo.imageStreams.push($scope.outputImageStream.imageStream);
              $scope.outputOptions.pickedImageStream = $scope.outputImageStream.imageStream;
              $scope.outputOptions.pickedTag = $scope.outputImageStream.tag;
            }
            angular.forEach(projectImageStreams, function(imageStream, name) {
              var tagList = [];
              if (imageStream.status.tags) {
                imageStream.status.tags.forEach(function(item){
                  tagList.push(item["tag"]);
                });
              }
              $scope.pushTo.imageStreams.push(name);
              $scope.pushTo.tags[name] = tagList;
            });

            if (selectFirstOption) {
              $scope.outputOptions.pickedImageStream = $scope.pushTo.imageStreams[0];
              $scope.clearSelectedTag($scope.outputOptions, $scope.pushTo.tags, true);
              // If defined Image Stream is not present in the Image Streams of picked Namespace set tag to empty string,
              // so the user has to pick from a existing Image Streams.
            } else if (!$scope.pushTo.imageStreams.contains($scope.outputOptions.pickedImageStream)) {
              $scope.outputOptions.pickedTag = "";
            }
          } else if (projectName === $scope.outputImageStream.namespace && !selectFirstOption) {
            $scope.pushTo.imageStreams.push($scope.outputImageStream.imageStream);
            $scope.outputOptions.pickedImageStream = $scope.outputImageStream.imageStream;
            $scope.outputOptions.pickedTag = $scope.outputImageStream.tag;
          } else {
            $scope.outputOptions.pickedImageStream = "";
            $scope.outputOptions.pickedTag = "";
          }
        });
      }
    };

    $scope.clearSelectedTag = function(optionsModel, tagHash, isOutput) {
      var tags = tagHash[optionsModel.pickedImageStream];
      if (tags.length > 0) {
        optionsModel.pickedTag = _.find(tags, function(tag) { return tag == "latest" }) || tags[0];
      } else if (isOutput) {
        optionsModel.pickedTag = "latest";
      } else {
        optionsModel.pickedTag = "";
      }
    };

    // Check if the namespace is available. If so add him to available namespaces and remove him from unavailable 
    $scope.checkNamespaceAvailability = function(namespace) {
      DataService.get("projects", namespace, {}, { errorNotification: false})
      .then(function() {
        $scope.availableProjects.push(namespace);
      }, function(result) {
      });
    };

    $scope.updatedImageSourcePath = function(imageSourcePaths) {
      var updatedImageSourcePath = [];
      angular.forEach(imageSourcePaths, function(v, k) {
        var env = {
          sourcePath: k,
          destinationDir: v
        };
        updatedImageSourcePath.push(env);
      });
      return updatedImageSourcePath
    };

    $scope.updateEnvVars = function(envVars) {
      var updatedEnvVars = [];
      angular.forEach(envVars, function(v, k) {
        var env = {
          name: k,
          value: v
        };
        updatedEnvVars.push(env);
      });
      return updatedEnvVars;
    };

    $scope.updateBinarySource = function() {
      // If binarySource check if the AsFile string is set and construct the object accordingly.
      if ($scope.sources.binary) {
        if ($scope.options.binaryAsFile !== "") {
          $scope.updatedBuildConfig.spec.source.binary.asFile = $scope.options.binaryAsFile;
        } else {
          $scope.updatedBuildConfig.spec.source.binary = {};
        }
      }      
    };

    $scope.constructImageObject = function(optionsModel) {
      var imageObject = {};
      if (optionsModel.pickedType === "ImageStreamTag") {
        imageObject = {
          kind: optionsModel.pickedType,
          namespace: optionsModel.pickedNamespace,
          name: optionsModel.pickedImageStream + ":" + optionsModel.pickedTag
        };
      } else if (optionsModel.pickedType === "DockerImage") {
        imageObject = {
          kind: optionsModel.pickedType,
          name: optionsModel.pickedDockerImage
        };
      } else if(optionsModel.pickedType === "ImageStreamImage") {
        imageObject = {
          kind: optionsModel.pickedType,
          namespace: optionsModel.pickedNamespace,
          name: optionsModel.pickedImageStreamImage
        }
      }
      return imageObject;
    };

    $scope.updateTriggers = function() {
      var presentTriggers = $scope.triggers.present;
      var triggers = [];
      if (presentTriggers.webhook) {
        var webhooks = _.filter($scope.buildConfig.spec.triggers, function(obj) { return obj.type === "GitHub" })
        if (webhooks.length === 0) {
          webhooks.push({
            github: {
              secret: ApplicationGenerator._generateSecret()
            },
            type: "GitHub"
          });
        }
        triggers = triggers.concat(webhooks);
      }
      triggers = triggers.concat(_.filter($scope.buildConfig.spec.triggers, function(obj) { return obj.type === "Generic" }));

      if (presentTriggers.configChange) {
        triggers.push({
          type: "ConfigChange"
        });
      }

      // Add all imageChange triggers to the triggers array. The imageChange trigger for the builder is added in getTriggerMap()
      // method into imageChangeTriggers array, even if the buildConfig doesn't contain it. If builder imageChange trigger is checked, 
      // keep the trigger, otherwise delete it.
      triggers = triggers.concat($scope.triggers.imageChangeTriggers)
      if (!presentTriggers.imageChange) {
        _.remove(triggers, function(trigger) {return trigger === $scope.triggers.builderImageChangeTrigger})
      }

      return triggers;
    };

    $scope.save = function() {
      $scope.disableInputs = true;
      // Update Configuration
      $filter('buildStrategy')($scope.updatedBuildConfig).forcePull = $scope.options.forcePull;
      if ($scope.strategyType === "Docker") {
        $filter('buildStrategy')($scope.updatedBuildConfig).noCache = $scope.options.noCache;
      }

      $scope.updateBinarySource();

      // If imageSources are present update each ones From and Paths.
      if ($scope.sources.images && $scope.sourceImage) {
        $scope.updatedBuildConfig.spec.source.images[0].paths  = $scope.updatedImageSourcePath($scope.imageSourcePaths);
        // Construct updated imageSource builder image object based on it's kind for the only imageSource
        $scope.updatedBuildConfig.spec.source.images[0].from = $scope.constructImageObject($scope.imageSourceOptions);
      }

      // Construct updated builder image object based on it's kind
      if ($scope.builderOptions.pickedType === "None") {
        delete $filter('buildStrategy')($scope.updatedBuildConfig).from;
      } else {
        $filter('buildStrategy')($scope.updatedBuildConfig).from = $scope.constructImageObject($scope.builderOptions);
      }

      // Construct updated output image object based on it's kind. Only Image Stream Tag, Docker Image and None can 
      // be specified for the output image. Not Image Stream Image since they are immutable.
      if ($scope.outputOptions.pickedType === "None") {
        // If user will change the output reference to 'None' shall the potential PushSecret be deleted as well?
        // This case won't delete them.
        delete $scope.updatedBuildConfig.spec.output.to
      } else {
        $scope.updatedBuildConfig.spec.output.to = $scope.constructImageObject($scope.outputOptions);
      }

      // Update envVars
      $filter('buildStrategy')($scope.updatedBuildConfig).env = $scope.updateEnvVars($scope.envVars);

      // Update triggers
      $scope.updatedBuildConfig.spec.triggers = $scope.updateTriggers();

      DataService.update("buildconfigs", $scope.updatedBuildConfig.metadata.name, $scope.updatedBuildConfig, {
        namespace: $scope.updatedBuildConfig.metadata.namespace
      }).then(
        function() {
          AlertMessageService.addAlert({
            name: $scope.updatedBuildConfig.metadata.name,
            data: {
              type: "success",
              message: "Build Config " + $scope.updatedBuildConfig.metadata.name + " was successfully updated."
            }
          });
          $location.path(Navigate.resourceURL($scope.updatedBuildConfig, "BuildConfig", $scope.updatedBuildConfig.metadata.namespace));
        },
        function(result) {
          $scope.disableInputs = false;

          $scope.alerts["save"] = {
            type: "error",
            message: "An error occurred updating the build " + $scope.updatedBuildConfig.metadata.name + "Build Config",
            details: $filter('getErrorDetails')(result)
          }
        }
      );
    };

    $scope.isNamespaceAvailable = function(namespace) {
      return $scope.availableProjects.contains(namespace);
    };

    $scope.inspectNamespace = function(imageStreams, pickedImageStream) {
      if (imageStreams.length === 0) {
        return "empty";
      } 
      if (imageStreams.length !== 0 && !imageStreams.contains(pickedImageStream)) {
        return "noMatch";
      }
      return "";
    };

    $scope.inspectTags = function(tagHash, pickedImageStream, pickedTag) {
      if (tagHash[pickedImageStream] && pickedImageStream !== '') {
        if (tagHash[pickedImageStream].length === 0) {
          return "empty";
        } 
        if (tagHash[pickedImageStream].length !== 0 && !tagHash[pickedImageStream].contains(pickedTag)) {
          return "noMatch";
        }
      }
      return "";
    };

    $scope.showOutputTagWarning = function(form) {
      if (form.outputNamespace.$dirty || form.outputImageStream.$dirty || form.outputTag.$dirty) {
        if ($scope.pushTo.tags[$scope.outputOptions.pickedImageStream] && $scope.pushTo.tags[$scope.outputOptions.pickedImageStream].contains($scope.outputOptions.pickedTag)) {
          return true;
        }
      }
      return false;
    };

    $scope.getSourceMap = function(sourceMap, sources) {
      angular.forEach(sources, function(value, key) {
        sourceMap[key] = true;
      });
      return sourceMap;
    };

  });
