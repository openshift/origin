/*
Package defaults contains the BuildDefaults admission control plugin.

The plugin allows setting default values for build setings like the git HTTP
and HTTPS proxy URLs and additional environment variables for the build
strategy

Configuration

Configuration is done via a BuildDefaultsConfig object:

 apiVersion: v1
 kind: BuildDefaultsConfiguration
 gitHTTPProxy: http://my.proxy.server:12345
 gitHTTPSProxy: https://my.proxy.server:7890
 env:
 - name: ENV_VAR1
   value: VALUE1
 - name: ENV_VAR2
   value: VALUE2
*/
package defaults
