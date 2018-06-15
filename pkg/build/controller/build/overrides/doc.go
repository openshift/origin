/*
Package overrides contains the BuildOverrides admission control plugin.

The plugin allows overriding settings on builds via the build pod.

Configuration

Configuration is done via a BuildOverridesConfig object:

 apiVersion: v1
 kind: BuildOverridesConfig
 forcePull: true
*/
package overrides
