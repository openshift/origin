# Remote Shell to Kubernetes Pods

This OpenShift Web Console extension enables direct in-browser connections to containers in Kubernetes pods over WebSockets and the exec API endpoint. It leverages hterm.js from Chromium to provide an in browser console.

Inspired by https://github.com/yudai/gotty

## Try it out

1. Enable the shell extension directory in your OpenShift configuration by adding the following stanza:

        # master-config.yaml
        ...
        assetConfig:
          ...
          extensions:
          - name: shell
            sourceDirectory: ... # path to this directory, relative to master-config.yaml

2. Restart your server
3. Find your current token from the CLI with `oc whoami -t` (you must be logged in)
4. Identify the pod you want to connect to, its namespace, and the container name within the pod
5. In your browser, visit:

        <server>/console/extensions/shell/index.html?container=<containerName>&api=<host>/api/v1/namespaces/<namespace>/pods/<podname>#<apitoken>

6. You should see an exec window and be able to type commands.