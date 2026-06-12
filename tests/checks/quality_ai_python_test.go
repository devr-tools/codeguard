package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForHallucinatedPythonImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests>=2.0\n")
	writeFile(t, filepath.Join(dir, "app.py"), `import totally_made_up_pkg

def run():
    return totally_made_up_pkg.go()
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-import")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.hallucinated-import")
}

func TestQualityCheckResolvesDeclaredAndLocalPythonImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[project]
name = "fixture"
dependencies = [
    "requests>=2.0",
    "PyYAML>=6.0",
    "opencv-python",
]
`)
	writeFile(t, filepath.Join(dir, "helper.py"), "def assist():\n    return 1\n")
	writeFile(t, filepath.Join(dir, "pkg", "__init__.py"), "")
	writeFile(t, filepath.Join(dir, "app.py"), `import os
import json
import requests
import yaml
import cv2
import helper
import pkg
from pkg import thing
from . import sibling

def run():
    return helper.assist()
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-import-ok")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.hallucinated-import")
}

func TestQualityCheckResolvesPythonRequirementsAliasesAndNormalization(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "Pillow==10.0\npython-dateutil\nFoo_Bar>=1.0\n")
	writeFile(t, filepath.Join(dir, "app.py"), `from PIL import Image
import dateutil
import foo_bar
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-alias")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.hallucinated-import")
}

func TestQualityCheckStaysQuietForPythonImportsWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "import some_environment_pkg\n")

	cfg := qualityAITestConfig(dir, "quality-ai-py-nomanifest")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.hallucinated-import")
}

func TestQualityCheckHonorsHallucinatedImportToggle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests>=2.0\n")
	writeFile(t, filepath.Join(dir, "app.py"), "import totally_made_up_pkg\n")

	cfg := qualityAITestConfig(dir, "quality-ai-py-toggle")
	cfg.Targets[0].Language = "python"
	disabled := false
	cfg.Checks.QualityRules.AIChecks.HallucinatedImport = &disabled
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.hallucinated-import")
}
