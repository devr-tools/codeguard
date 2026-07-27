package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func dataConfig(name string, dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	on := true
	off := false
	cfg.Checks.Reliability = &off
	cfg.Checks.Data = &on
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func TestDataGoDetectsMissingTransactionAndDualWrite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

type Repo interface {
	Save(any) error
	Update(any) error
}

type Publisher interface {
	Publish(any) error
}

func SaveAndPublish(repo Repo, publisher Publisher, order any, event any) error {
	if err := repo.Save(order); err != nil {
		return err
	}
	if err := repo.Update(order); err != nil {
		return err
	}
	return publisher.Publish(event)
}
`)

	report, err := codeguard.Run(context.Background(), dataConfig("data-dual-write", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
}

func TestDataGoDetectsUnstablePaginationAndUnboundedRead(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "queries.go"), `package sample

const unstable = "SELECT * FROM orders LIMIT 50 OFFSET 100"
const unbounded = "SELECT * FROM events"
`)

	report, err := codeguard.Run(context.Background(), dataConfig("data-sql", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unbounded-read")
}

func TestDataGoDetectsConsumerWithoutDeduplication(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "consumer.go"), `package sample

type Emailer interface {
	Send(any) error
}

func HandleMessage(email Emailer, event any) error {
	return email.Send(event)
}
`)

	report, err := codeguard.Run(context.Background(), dataConfig("data-consumer", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-deduplication")
}
