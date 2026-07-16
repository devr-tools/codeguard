package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Framework-aware rules (performance_rules.detect_framework_patterns): each
// rule is gated on file-level framework evidence, so every positive fixture
// carries an import/idiom and every negative test proves either the evidence
// gate or the rule's exemption.

func TestPerformanceCheckWarnsForDjangoRelationAccessInQuerysetLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "emails.py"),
		"from django.db import models\n\nfrom app.models import Order\n\n\ndef send_receipts():\n    for order in Order.objects.all():\n        send(order.customer.email)\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-django-relation", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.python.django-nplusone-relation")
}

func TestPerformanceCheckWarnsForDjangoReverseRelationOnTrackedQuerysetVar(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "counts.py"),
		"from django.db import models\n\nfrom app.models import User\n\n\ndef order_counts():\n    users = User.objects.filter(active=True)\n    counts = {}\n    for user in users:\n        counts[user.pk] = user.order_set.count()\n    return counts\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-django-reverse", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.python.django-nplusone-relation")
}

func TestPerformanceCheckSkipsDjangoRelationWhenSelectRelatedPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "emails.py"),
		"from django.db import models\n\nfrom app.models import Order\n\n\ndef send_receipts():\n    for order in Order.objects.select_related(\"customer\"):\n        send(order.customer.email)\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-django-prefetched", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.django-nplusone-relation")
}

func TestPerformanceCheckSkipsRelationChainsWithoutDjangoEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "plain.py"),
		"def send_all(orders):\n    for order in orders:\n        send(order.customer.email)\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-no-django", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.django-nplusone-relation")
}

func TestPerformanceCheckSkipsScalarMethodChainsInQuerysetLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "names.py"),
		"from django.db import models\n\nfrom app.models import User\n\n\ndef names():\n    out = []\n    for user in User.objects.all():\n        out.append(user.name.strip())\n    return out\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-scalar-chain", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.django-nplusone-relation")
}

func TestPerformanceCheckWarnsForDjangoORMQueryInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "articles.py"),
		"from django.db import models\n\nfrom app.models import Article\n\n\ndef load(ids):\n    out = []\n    for pk in ids:\n        out.append(Article.objects.get(pk=pk))\n    return out\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-orm-loop", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.python.orm-query-in-loop")
	// Disjointness: the generic n-plus-one pattern does not cover
	// .objects.get, so the line must not double-report.
	assertFindingRuleAbsent(t, report, "Performance", "performance.n-plus-one-query")
}

func TestPerformanceCheckWarnsForSQLAlchemySessionGetInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "rows.py"),
		"from sqlalchemy.orm import Session\n\nfrom app.models import User\n\n\ndef load(session, ids):\n    rows = []\n    for pk in ids:\n        rows.append(session.get(User, pk))\n    return rows\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-sqlalchemy-loop", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.python.orm-query-in-loop")
}

func TestPerformanceCheckSkipsRequestsSessionGetInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "pages.py"),
		"import requests\n\n\ndef load(urls):\n    session = requests.Session()\n    out = []\n    for url in urls:\n        out.append(session.get(url))\n    return out\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-requests-session", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// session.get without SQLAlchemy evidence is an HTTP session, not the ORM.
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.orm-query-in-loop")
}

func TestPerformanceCheckSkipsORMQueryOutsideLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "single.py"),
		"from django.db import models\n\nfrom app.models import Article\n\n\ndef load_one(pk):\n    return Article.objects.get(pk=pk)\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-orm-no-loop", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.orm-query-in-loop")
}

func TestPerformanceCheckFrameworkPatternsToggleDisablesRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "emails.py"),
		"from django.db import models\n\nfrom app.models import Order\n\n\ndef send_receipts():\n    for order in Order.objects.all():\n        send(order.customer.email)\n        refresh(Order.objects.get(pk=order.pk))\n")

	cfg := performanceConfig("performance-py-frameworks-off", dir, "python")
	cfg.Checks.PerformanceRules.DetectFrameworkPatterns = boolPtr(false)

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.django-nplusone-relation")
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.orm-query-in-loop")
}
