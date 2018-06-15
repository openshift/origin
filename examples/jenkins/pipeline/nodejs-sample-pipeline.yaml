kind: "BuildConfig"
apiVersion: "v1"
metadata:
  name: "nodejs-sample-pipeline"
spec:
  strategy:
    jenkinsPipelineStrategy:
      jenkinsfile: |-
        // path of the template to use
        def templatePath = 'https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb.json'
        // name of the template that will be created
        def templateName = 'nodejs-mongodb-example'
        // NOTE, the "pipeline" directive/closure from the declarative pipeline syntax needs to include, or be nested outside,
        // and "openshift" directive/closure from the OpenShift Client Plugin for Jenkins.  Otherwise, the declarative pipeline engine
        // will not be fully engaged.
        pipeline {
            agent {
              node {
                // spin up a node.js slave pod to run this build on
                label 'nodejs'
              }
            }
            options {
                // set a timeout of 20 minutes for this pipeline
                timeout(time: 20, unit: 'MINUTES')
            }

            stages {
                stage('preamble') {
                    steps {
                        script {
                            openshift.withCluster() {
                                openshift.withProject() {
                                    echo "Using project: ${openshift.project()}"
                                }
                            }
                        }
                    }
                }
                stage('cleanup') {
                    steps {
                        script {
                            openshift.withCluster() {
                                openshift.withProject() {
                                    // delete everything with this template label
                                    openshift.selector("all", [ template : templateName ]).delete()
                                    // delete any secrets with this template label
                                    if (openshift.selector("secrets", templateName).exists()) {
                                        openshift.selector("secrets", templateName).delete()
                                    }
                                }
                            }
                        } // script
                    } // steps
                } // stage
                stage('create') {
                    steps {
                        script {
                            openshift.withCluster() {
                                openshift.withProject() {
                                    // create a new application from the templatePath
                                    openshift.newApp(templatePath)
                                }
                            }
                        } // script
                    } // steps
                } // stage
                stage('build') {
                    steps {
                        script {
                            openshift.withCluster() {
                                openshift.withProject() {
                                    def builds = openshift.selector("bc", templateName).related('builds')
                                    builds.untilEach(1) {
                                        return (it.object().status.phase == "Complete")
                                    }
                                }
                            }
                        } // script
                    } // steps
                } // stage
                stage('deploy') {
                    steps {
                        script {
                            openshift.withCluster() {
                                openshift.withProject() {
                                    def rm = openshift.selector("dc", templateName).rollout()
                                    openshift.selector("dc", templateName).related('pods').untilEach(1) {
                                        return (it.object().status.phase == "Running")
                                    }
                                }
                            }
                        } // script
                    } // steps
                } // stage
                stage('tag') {
                    steps {
                        script {
                            openshift.withCluster() {
                                openshift.withProject() {
                                    // if everything else succeeded, tag the ${templateName}:latest image as ${templateName}-staging:latest
                                    // a pipeline build config for the staging environment can watch for the ${templateName}-staging:latest
                                    // image to change and then deploy it to the staging environment
                                    openshift.tag("${templateName}:latest", "${templateName}-staging:latest")
                                }
                            }
                        } // script
                    } // steps
                } // stage
            } // stages
        } // pipeline
      type: JenkinsPipeline
