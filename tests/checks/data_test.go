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

func TestDataGoDetectsReadModifyWriteTransactionSideEffectCacheAndExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "risks.go"), `package sample

type Repo interface {
	Get(string) (Order, error)
	Save(Order) error
}

type DB interface {
	Transaction(func(Tx) error) error
}

type Tx interface{}

type Publisher interface {
	Publish(any) error
}

type Cache interface {
	Set(string, any)
}

type Order struct{}

func UpdateDerived(repo Repo, id string) error {
	order, err := repo.Get(id)
	if err != nil {
		return err
	}
	return repo.Save(order)
}

func PublishInsideTransaction(db DB, publisher Publisher, event any) error {
	return db.Transaction(func(tx Tx) error {
		return publisher.Publish(event)
	})
}

func CacheOrder(cache Cache, order Order) {
	cache.Set("order", order)
}

// consumer assumes exactly once delivery from the broker
func Consumer() {}
`)

	report, err := codeguard.Run(context.Background(), dataConfig("data-more-go-risks", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.read-modify-write-race")
	assertFindingRulePresent(t, report, "Data Correctness", "data.side-effect-in-transaction")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataGoAcceptsTransactionalOutboxBoundedQueryAndPolicy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe.go"), `package sample

type Repo interface {
	WithTx(func(Tx) error) error
	Query(string) ([]Order, error)
}

type Tx interface {
	Get(string) (Order, error)
	Save(Order) error
}

type Outbox interface {
	Publish(any) error
}

type Cache interface {
	Set(string, any, int)
}

type Event struct {
	MessageID string
}

type Order struct{}

func UpdateWithOutbox(repo Repo, outbox Outbox, cache Cache, event Event) error {
	if err := repo.WithTx(func(tx Tx) error {
		order, err := tx.Get(event.MessageID)
		if err != nil {
			return err
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		return outbox.Publish(event)
	}); err != nil {
		return err
	}
	cache.Set("order", event, 60)
	_, err := repo.Query("SELECT * FROM orders WHERE account_id = ? ORDER BY id LIMIT 20")
	return err
}

// exactly once is handled through idempotency and dedupe records
func HandleEvent(event Event) error {
	if processedByMessageID(event.MessageID) {
		return nil
	}
	return nil
}

func processedByMessageID(string) bool { return false }
`)

	report, err := codeguard.Run(context.Background(), dataConfig("data-safe-go", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Data Correctness", "data.read-modify-write-race")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.cache-without-policy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.exactly-once-assumption")
}
