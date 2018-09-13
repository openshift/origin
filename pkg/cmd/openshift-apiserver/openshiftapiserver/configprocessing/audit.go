package configprocessing

import (
	"io"

	"github.com/golang/glog"
	"gopkg.in/natefinch/lumberjack.v2"

	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	"k8s.io/apiserver/pkg/audit"
	auditpolicy "k8s.io/apiserver/pkg/audit/policy"
	auditlog "k8s.io/apiserver/plugin/pkg/audit/log"
	auditwebhook "k8s.io/apiserver/plugin/pkg/audit/webhook"
	pluginwebhook "k8s.io/apiserver/plugin/pkg/audit/webhook"

	configv1 "github.com/openshift/api/config/v1"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

func GetAuditConfig(auditConfig configv1.AuditConfig) (audit.Backend, auditpolicy.Checker, error) {
	if !auditConfig.Enabled {
		return nil, nil, nil
	}
	var (
		backend       audit.Backend
		policyChecker auditpolicy.Checker
		writer        io.Writer
	)
	if len(auditConfig.AuditFilePath) > 0 {
		writer = &lumberjack.Logger{
			Filename:   auditConfig.AuditFilePath,
			MaxAge:     int(auditConfig.MaximumFileRetentionDays),
			MaxBackups: int(auditConfig.MaximumRetainedFiles),
			MaxSize:    int(auditConfig.MaximumFileSizeMegabytes),
		}
	} else {
		// backwards compatible writer to regular log
		writer = cmdutil.NewGLogWriterV(0)
	}
	backend = auditlog.NewBackend(writer, auditlog.FormatJson, auditv1beta1.SchemeGroupVersion)
	policyChecker = auditpolicy.NewChecker(&auditinternal.Policy{
		// This is for backwards compatibility maintaining the old visibility, ie. just
		// raw overview of the requests comming in.
		Rules: []auditinternal.PolicyRule{{Level: auditinternal.LevelMetadata}},
	})

	// when a policy file is defined we enable the advanced auditing
	if len(auditConfig.PolicyConfiguration.Raw) != 0 || len(auditConfig.PolicyFile) > 0 {
		// policy configuration
		if len(auditConfig.PolicyConfiguration.Raw) != 0 {
			p, err := auditpolicy.LoadPolicyFromBytes(auditConfig.PolicyConfiguration.Raw, "config.json")
			if err != nil {
				return nil, nil, err
			}
			policyChecker = auditpolicy.NewChecker(p)
		} else if len(auditConfig.PolicyFile) > 0 {
			p, err := auditpolicy.LoadPolicyFromFile(auditConfig.PolicyFile)
			if err != nil {
				return nil, nil, err
			}
			policyChecker = auditpolicy.NewChecker(p)
		}

		// log configuration, only when file path was provided
		if len(auditConfig.AuditFilePath) > 0 {
			backend = auditlog.NewBackend(writer, string(auditConfig.LogFormat), auditv1beta1.SchemeGroupVersion)
		}

		// webhook configuration, only when config file was provided
		if len(auditConfig.WebHookKubeConfig) > 0 {
			webhook, err := auditwebhook.NewBackend(auditConfig.WebHookKubeConfig, auditv1beta1.SchemeGroupVersion, pluginwebhook.DefaultInitialBackoff)
			if err != nil {
				glog.Fatalf("Audit webhook initialization failed: %v", err)
			}
			backend = audit.Union(backend, webhook)
		}
	}

	return backend, policyChecker, nil
}
