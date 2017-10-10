#!groovy

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Jenkins continuous integration
//
// Parameters Jenkins needs to / can supply:
//
// TEST_PROJECT:   Google Cloud Project ID of the project to use for testing.
// TEST_ZONE:      GCP Zone in which to create test GKE cluster
// TEST_ACCOUNT:   GCP service account credentials (JSON file) to use for testing.

def repo_url = params.REPO_URL

def updatePullRequest(flow, success = false) {
  def state, message
  switch (flow) {
    case 'run':
      state = 'PENDING'
      message = "Running presubmits at ${env.BUILD_URL} ..."
      break
    case 'verify':
      state = success ? 'SUCCESS' : 'FAILURE'
      message = "${success ? 'Successful' : 'Failed'} presubmits. " +
          "Details at ${env.BUILD_URL}."
      break
    default:
      error('flow can only be run or verify')
  }

  step([
      $class: "GitHubCommitStatusSetter",
      reposSource: [$class: "ManuallyEnteredRepositorySource", url: "${repo_url}"],
      contextSource: [$class: "ManuallyEnteredCommitContextSource", context: "${JOB_NAME}"],
      errorHandlers: [[$class: "ChangingBuildStatusErrorHandler", result: "UNSTABLE"]],
      statusResultSource: [ $class: "ConditionalStatusResultSource", results: [[$class: "AnyBuildResult", message: message, state: state]] ]
  ]);
}

// Verify required parameters
if (! params.TEST_PROJECT) {
  error 'Missing required parameter TEST_PROJECT'
}

if (! params.TEST_ACCOUNT) {
  error 'Missing required parameter TEST_ACCOUNT'
}

def test_project = params.TEST_PROJECT
def test_account = params.TEST_ACCOUNT
def test_zone    = params.TEST_ZONE ?: 'us-west1-b'
def namespace    = 'catalog'
def root_path    = 'src/github.com/kubernetes-incubator/service-catalog'
def timeoutMin   = 30

node {
  echo "Service Catalog end-to-end test"

  sh "sudo rm -rf ${env.WORKSPACE}/*"

  updatePullRequest('run')

  // Checkout the source code.
  checkout scm

  env.GOPATH = env.WORKSPACE
  env.ROOT = "${env.WORKSPACE}/${root_path}"

  env.KUBECONFIG = "${env.ROOT}/k8s-kubeconfig"

  dir([path: env.ROOT]) {
    // Run build.

    def clustername = "jenkins-" + sh([returnStdout: true, script: '''openssl rand \
        -base64 100 | tr -dc a-z0-9 | cut -c -25''']).trim()

    def version = sh([returnStdout: true, script: 'git describe --always --abbrev=7 --dirty']).trim()

    try {
      timeout(timeoutMin) {
        // These are done in parallel since creating the cluster takes a while,
        // and the build doesn't depend on it.
        parallel(
          'Cluster': {
            withCredentials([file(credentialsId: "${test_account}", variable: 'TEST_SERVICE_ACCOUNT')]) {
              sh """${env.ROOT}/contrib/jenkins/init_cluster.sh ${clustername} \
                    --project ${test_project} \
                    --zone ${test_zone} \
                    --credentials ${env.TEST_SERVICE_ACCOUNT}"""
            }
          },
          'Build & Unit/Integration Tests': {
            sh """${env.ROOT}/contrib/jenkins/build.sh \
                  --project ${test_project} \
                  --version ${version}"""
          }
        )

        // Run through the walkthrough on the cluster, once with an etcd-backed API server and once
        // with a TPR-backed one.
        sh """${env.ROOT}/contrib/jenkins/test_walkthrough.sh \
              --registry gcr.io/${test_project}/catalog/ \
              --version ${version} \
              --cleanup \
              --fix-auth \
              --create-artifacts
        """

        ansiColor('xterm-darker-gray') {
          // Run the e2e test framework
          sh """${env.ROOT}/contrib/jenkins/run_e2e.sh \
                --registry gcr.io/${test_project}/catalog/ \
                --version ${version} \
                --cleanup \
                --create-artifacts
          """
        }

        echo 'Run succeeded.'
      }
    } catch (Exception e) {
      echo 'Run failed.'
      currentBuild.result = 'FAILURE'
    } finally {
      archiveArtifacts artifacts: 'walkthrough*.txt', fingerprint: true
      archiveArtifacts artifacts: 'e2e*.txt', fingerprint: true
      try {
        sh """${env.ROOT}/contrib/jenkins/cleanup_cluster.sh --kubeconfig ${KUBECONFIG}"""
      } catch (Exception e) {
        echo 'Exception caught during cleanup.'
        currentBuild.result = 'FAILURE'
      }
    }

    if (currentBuild.result == 'FAILURE') {
      updatePullRequest('verify', false)
      error 'Build failed.'
    }
  }

  updatePullRequest('verify', true)
  archiveArtifacts artifacts: 'coverage.html', allowEmptyArchive: true, onlyIfSuccessful: true
}
