package haproxy

import (
	"fmt"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	templateutil "github.com/openshift/origin/pkg/router/template/util"
)

// mapEntryGeneratorFunc generates an haproxy config map entry.
type mapEntryGeneratorFunc func(*BackendConfig) *HAProxyMapEntry

// generateWildcardDomainMapEntry generates a wildcard domain map entry.
func generateWildcardDomainMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) > 0 && cfg.IsWildcard {
		return &HAProxyMapEntry{
			Key:   templateutil.GenerateRouteRegexp(cfg.Host, "", cfg.IsWildcard),
			Value: "1",
		}
	}
	return nil
}

// generateHttpMapEntry generates a map entry for insecure/http hosts.
func generateHttpMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) == 0 {
		return nil
	}

	needsHttpMap := false
	if len(cfg.Termination) == 0 {
		needsHttpMap = true
	} else if (cfg.Termination == routeapi.TLSTerminationEdge || cfg.Termination == routeapi.TLSTerminationReencrypt) && cfg.InsecurePolicy == routeapi.InsecureEdgeTerminationPolicyAllow {
		needsHttpMap = true
	}

	if !needsHttpMap {
		return nil
	}

	return &HAProxyMapEntry{
		Key:   templateutil.GenerateRouteRegexp(cfg.Host, cfg.Path, cfg.IsWildcard),
		Value: fmt.Sprintf("%s:%s", templateutil.GenerateBackendNamePrefix(cfg.Termination), cfg.Name),
	}
}

// generateEdgeReencryptMapEntry generates a map entry for edge secured hosts.
func generateEdgeReencryptMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) == 0 || (cfg.Termination != routeapi.TLSTerminationEdge && cfg.Termination != routeapi.TLSTerminationReencrypt) {
		return nil
	}

	return &HAProxyMapEntry{
		Key:   templateutil.GenerateRouteRegexp(cfg.Host, cfg.Path, cfg.IsWildcard),
		Value: fmt.Sprintf("%s:%s", templateutil.GenerateBackendNamePrefix(cfg.Termination), cfg.Name),
	}
}

// generateHttpRedirectMapEntry generates a map entry for redirecting insecure/http hosts.
func generateHttpRedirectMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) > 0 && cfg.InsecurePolicy == routeapi.InsecureEdgeTerminationPolicyRedirect {
		return &HAProxyMapEntry{
			Key:   templateutil.GenerateRouteRegexp(cfg.Host, cfg.Path, cfg.IsWildcard),
			Value: cfg.Name,
		}
	}

	return nil
}

// generateTCPMapEntry generates a map entry for passthrough/secure hosts.
func generateTCPMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) > 0 && len(cfg.Path) == 0 && (cfg.Termination == routeapi.TLSTerminationPassthrough || cfg.Termination == routeapi.TLSTerminationReencrypt) {
		return &HAProxyMapEntry{
			Key:   templateutil.GenerateRouteRegexp(cfg.Host, "", cfg.IsWildcard),
			Value: cfg.Name,
		}
	}

	return nil
}

// generateSNIPassthroughMapEntry generates a map entry for SNI passthrough hosts.
func generateSNIPassthroughMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) > 0 && len(cfg.Path) == 0 && cfg.Termination == routeapi.TLSTerminationPassthrough {
		return &HAProxyMapEntry{
			Key:   templateutil.GenerateRouteRegexp(cfg.Host, "", cfg.IsWildcard),
			Value: "1",
		}
	}

	return nil
}

// generateCertConfigMapEntry generates a cert config map entry.
func generateCertConfigMapEntry(cfg *BackendConfig) *HAProxyMapEntry {
	if len(cfg.Host) > 0 && (cfg.Termination == routeapi.TLSTerminationEdge || cfg.Termination == routeapi.TLSTerminationReencrypt) && cfg.HasCertificate {
		return &HAProxyMapEntry{
			Key:   fmt.Sprintf("%s.pem", cfg.Name),
			Value: templateutil.GenCertificateHostName(cfg.Host, cfg.IsWildcard),
		}
	}

	return nil
}

// GenerateMapEntry generates a haproxy map entry.
func GenerateMapEntry(id string, cfg *BackendConfig) *HAProxyMapEntry {
	generator, ok := map[string]mapEntryGeneratorFunc{
		"os_wildcard_domain.map":     generateWildcardDomainMapEntry,
		"os_http_be.map":             generateHttpMapEntry,
		"os_edge_reencrypt_be.map":   generateEdgeReencryptMapEntry,
		"os_route_http_redirect.map": generateHttpRedirectMapEntry,
		"os_tcp_be.map":              generateTCPMapEntry,
		"os_sni_passthrough.map":     generateSNIPassthroughMapEntry,
		"cert_config.map":            generateCertConfigMapEntry,
	}[id]

	if !ok {
		return nil
	}

	return generator(cfg)
}
