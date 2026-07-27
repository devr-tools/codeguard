package config

import (
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateReliabilityRules(rules core.ReliabilityRulesConfig) error {
	if rules.MaxRetryAttempts < 0 {
		return fmt.Errorf("reliability_rules.max_retry_attempts must not be negative")
	}
	if rules.MaxInlineGoroutinesPerFunction < 0 {
		return fmt.Errorf("reliability_rules.max_inline_goroutines_per_function must not be negative")
	}
	return nil
}

func validateDataRules(rules core.DataRulesConfig) error {
	if rules.MaxUnboundedReadRows < 0 {
		return fmt.Errorf("data_rules.max_unbounded_read_rows must not be negative")
	}
	if rules.MaxWritesWithoutTransaction < 0 {
		return fmt.Errorf("data_rules.max_writes_without_transaction must not be negative")
	}
	return nil
}

func validateObservabilityRules(rules core.ObservabilityRulesConfig) error {
	for _, item := range []struct {
		field  string
		values []string
	}{
		{"observability_rules.structured_logger_patterns", rules.StructuredLoggerPatterns},
		{"observability_rules.sensitive_name_patterns", rules.SensitiveNamePatterns},
		{"observability_rules.high_cardinality_label_patterns", rules.HighCardinalityLabelPatterns},
		{"observability_rules.critical_path_patterns", rules.CriticalPathPatterns},
		{"observability_rules.healthcheck_path_patterns", rules.HealthcheckPathPatterns},
		{"observability_rules.instrumentation_evidence_patterns", rules.InstrumentationEvidencePatterns},
	} {
		if err := validateNonEmptyStrings(item.field, item.values); err != nil {
			return err
		}
	}
	return nil
}

func validateOperationsRules(rules core.OperationsRulesConfig) error {
	for _, item := range []struct {
		field  string
		values []string
	}{
		{"operations_rules.owner_file_patterns", rules.OwnerFilePatterns},
		{"operations_rules.runbook_path_patterns", rules.RunbookPathPatterns},
		{"operations_rules.critical_path_patterns", rules.CriticalPathPatterns},
	} {
		if err := validateNonEmptyStrings(item.field, item.values); err != nil {
			return err
		}
	}
	return nil
}

func validateDeliveryRules(rules core.DeliveryRulesConfig) error {
	for _, item := range []struct {
		field  string
		values []string
	}{
		{"delivery_rules.rollback_evidence_patterns", rules.RollbackEvidencePatterns},
		{"delivery_rules.kill_switch_patterns", rules.KillSwitchPatterns},
		{"delivery_rules.post_deploy_verification_patterns", rules.PostDeployVerificationPatterns},
		{"delivery_rules.migration_path_patterns", rules.MigrationPathPatterns},
		{"delivery_rules.high_risk_path_patterns", rules.HighRiskPathPatterns},
		{"delivery_rules.bootstrap_path_patterns", rules.BootstrapPathPatterns},
	} {
		if err := validateNonEmptyStrings(item.field, item.values); err != nil {
			return err
		}
	}
	return nil
}

func validateChangeRules(rules core.ChangeRulesConfig) error {
	for _, item := range []struct {
		field string
		value int
	}{
		{"change_rules.max_changed_files", rules.MaxChangedFiles},
		{"change_rules.max_changed_directories", rules.MaxChangedDirectories},
		{"change_rules.max_changed_lines", rules.MaxChangedLines},
		{"change_rules.max_public_interfaces_changed", rules.MaxPublicInterfacesChanged},
		{"change_rules.max_concern_families", rules.MaxConcernFamilies},
	} {
		if item.value < 0 {
			return fmt.Errorf("%s must not be negative", item.field)
		}
	}
	if rules.MinTestToProductionRatioPercent < 0 || rules.MinTestToProductionRatioPercent > 100 {
		return fmt.Errorf("change_rules.min_test_to_production_ratio_percent must be between 0 and 100")
	}
	return nil
}

func validateProductionRisk(risk core.ProductionRiskConfig) error {
	if risk.WarnThreshold < 0 || risk.WarnThreshold > 100 {
		return fmt.Errorf("production_risk.warn_threshold must be between 0 and 100")
	}
	if risk.FailThreshold < 0 || risk.FailThreshold > 100 {
		return fmt.Errorf("production_risk.fail_threshold must be between 0 and 100")
	}
	if risk.WarnThreshold > 0 && risk.FailThreshold > 0 && risk.WarnThreshold > risk.FailThreshold {
		return fmt.Errorf("production_risk.warn_threshold must not exceed fail_threshold")
	}
	for _, item := range []struct {
		field string
		value int
	}{
		{"production_risk.reliability_weight", risk.ReliabilityWeight},
		{"production_risk.data_weight", risk.DataWeight},
		{"production_risk.fail_weight", risk.FailWeight},
		{"production_risk.warn_weight", risk.WarnWeight},
	} {
		if item.value < 0 {
			return fmt.Errorf("%s must not be negative", item.field)
		}
	}
	return nil
}
