package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// qualityExamples cover the quality rules codeguard reports most often on real
// repositories (measured by scanning codeguard itself and an unrelated Go
// CLI). Threshold rules lower the threshold through Settings so the snippet
// stays short; the same `-set` override reproduces them from the CLI.
var qualityExamples = map[string][]core.RuleExample{
	"function.inconsistent-return-contract": {goExample("internal/orders/load.go", `package orders

type Order struct{ ID string }

func LoadOrder(id string) (*Order, error) {
	order, err := fetchOrder(id)
	if err != nil {
		return order, err
	}
	if order.ID == "" {
		return nil, nil
	}
	return order, nil
}

func fetchOrder(id string) (*Order, error) { return &Order{ID: id}, nil }
`, `package orders

import "errors"

type Order struct{ ID string }

var ErrNotFound = errors.New("order not found")

func LoadOrder(id string) (*Order, error) {
	order, err := fetchOrder(id)
	if err != nil {
		return nil, err
	}
	if order.ID == "" {
		return nil, ErrNotFound
	}
	return order, nil
}

func fetchOrder(id string) (*Order, error) { return &Order{ID: id}, nil }
`)},
	"function.hidden-mutation": {goExample("internal/orders/prepare.go", `package orders

type Audit interface{ Save(string) error }

func PrepareOrder(audit Audit, id string) string {
	audit.Save(id)
	return id
}
`, `package orders

type Audit interface{ Save(string) error }

func RecordOrder(audit Audit, id string) error {
	return audit.Save(id)
}
`)},
	"quality.mutable-global-state": {goExample("internal/orders/tenant.go", `package orders

var currentTenant = "default"

func Tenant() string { return currentTenant }
`, `package orders

const defaultTenant = "default"

type Service struct{ tenant string }

func NewService() *Service { return &Service{tenant: defaultTenant} }

func (s *Service) Tenant() string { return s.tenant }
`)},
	"quality.boolean-argument": {goExample("internal/orders/format.go", `package orders

import "strconv"

func FormatTotal(cents int, verbose bool) string {
	if verbose {
		return "total: " + strconv.Itoa(cents)
	}
	return strconv.Itoa(cents)
}
`, `package orders

import "strconv"

func FormatTotal(cents int) string {
	return strconv.Itoa(cents)
}

func FormatTotalLabel(cents int) string {
	return "total: " + FormatTotal(cents)
}
`)},
	"naming.boolean-not-predicate": {goExample("internal/orders/ship.go", `package orders

type Shipment struct{ Express bool }

func NewShipment(express bool) Shipment {
	return Shipment{Express: express}
}
`, `package orders

type Shipment struct{ Express bool }

func NewShipment(isExpress bool) Shipment {
	return Shipment{Express: isExpress}
}
`)},
	"error.context-lost": {goExample("internal/orders/save.go", `package orders

func SaveOrder(id string) error {
	if err := writeOrder(id); err != nil {
		return err
	}
	return nil
}

func writeOrder(string) error { return nil }
`, `package orders

import "fmt"

func SaveOrder(id string) error {
	if err := writeOrder(id); err != nil {
		return fmt.Errorf("save order %s: %w", id, err)
	}
	return nil
}

func writeOrder(string) error { return nil }
`)},
	"quality.cyclomatic-complexity": {goExample("internal/orders/shipping.go", `package orders

func ShippingCost(weight int, region string, express bool) int {
	cost := 5
	if weight > 10 {
		cost += 5
	}
	if region == "eu" || region == "uk" {
		cost += 3
	}
	if express && weight < 20 {
		cost *= 2
	}
	return cost
}
`, `package orders

var regionSurcharge = map[string]int{"eu": 3, "uk": 3}

func ShippingCost(weight int, region string) int {
	return 5 + weightSurcharge(weight) + regionSurcharge[region]
}

func weightSurcharge(weight int) int {
	if weight > 10 {
		return 5
	}
	return 0
}
`, "checks.quality_rules.max_cyclomatic_complexity=3")},
	"function.mixed-abstraction-level": {goExample("internal/orders/checkout.go", `package orders

import "database/sql"

type Order struct{ ID string }

func Checkout(db *sql.DB, order Order) error {
	validateOrder(order)
	rows, err := db.Query("select id from stock where order_id = $1", order.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	return persistOrder(order)
}

func validateOrder(Order)            {}
func persistOrder(Order) error       { return nil }
`, `package orders

type Order struct{ ID string }

type Stock interface{ Reserve(orderID string) error }

func Checkout(stock Stock, order Order) error {
	validateOrder(order)
	if err := stock.Reserve(order.ID); err != nil {
		return err
	}
	return persistOrder(order)
}

func validateOrder(Order)      {}
func persistOrder(Order) error { return nil }
`)},
	"function.command-query-mix": {goExample("internal/users/query.go", `package users

type User struct{}

type Repository interface {
	Find(string) (User, error)
	SaveAudit(string) error
}

func GetUser(repo Repository, id string) (User, error) {
	if err := repo.SaveAudit(id); err != nil {
		return User{}, err
	}
	return repo.Find(id)
}
`, `package users

type User struct{}

type Repository interface {
	Find(string) (User, error)
	SaveAudit(string) error
}

func GetUser(repo Repository, id string) (User, error) {
	return repo.Find(id)
}

func RecordUserRead(repo Repository, id string) error {
	return repo.SaveAudit(id)
}
`)},
	"error.partial-failure-hidden": {goExample("internal/orders/send.go", `package orders

func SendAll(items []string) ([]string, error) {
	sent := make([]string, 0, len(items))
	for _, item := range items {
		if err := sendItem(item); err != nil {
			continue
		}
		sent = append(sent, item)
	}
	return sent, nil
}

func sendItem(string) error { return nil }
`, `package orders

import "fmt"

func SendAll(items []string) ([]string, error) {
	sent := make([]string, 0, len(items))
	for _, item := range items {
		if err := sendItem(item); err != nil {
			return sent, fmt.Errorf("send %s: %w", item, err)
		}
		sent = append(sent, item)
	}
	return sent, nil
}

func sendItem(string) error { return nil }
`)},
	"quality.max-parameters": {goExample("internal/orders/create.go", `package orders

func CreateOrder(customer, sku, currency, coupon, region, channel string) string {
	return customer + sku + currency + coupon + region + channel
}
`, `package orders

type OrderRequest struct {
	Customer, SKU, Currency, Coupon, Region, Channel string
}

func CreateOrder(req OrderRequest) string {
	return req.Customer + req.SKU + req.Currency + req.Coupon + req.Region + req.Channel
}
`)},
	"function.excessive-parameters": {goExample("internal/orders/create.go", `package orders

func CreateOrder(customer, sku, currency, coupon, region, channel string) string {
	return customer + sku + currency + coupon + region + channel
}
`, `package orders

type OrderRequest struct {
	Customer, SKU, Currency, Coupon, Region, Channel string
}

func CreateOrder(req OrderRequest) string {
	return req.Customer + req.SKU + req.Currency + req.Coupon + req.Region + req.Channel
}
`)},
	"quality.max-file-lines": {goExample("internal/orders/status.go", `package orders

func Pending() string   { return "pending" }
func Paid() string      { return "paid" }
func Shipped() string   { return "shipped" }
func Delivered() string { return "delivered" }
func Refunded() string  { return "refunded" }
func Cancelled() string { return "cancelled" }
`, `package orders

func Pending() string { return "pending" }
func Paid() string    { return "paid" }
`, "checks.quality_rules.max_file_lines=6")},
	"defensive.unvalidated-boundary-input": {goExample("internal/api/user.go", `package api

type Body struct{ ID string }
type Request struct{ Body Body }

func HandleUser(req Request) string {
	return req.Body.ID
}
`, `package api

import "errors"

type Body struct{ ID string }
type Request struct{ Body Body }

func HandleUser(req Request) (string, error) {
	if err := validateUserID(req.Body.ID); err != nil {
		return "", err
	}
	return req.Body.ID, nil
}

func validateUserID(id string) error {
	if id == "" {
		return errors.New("id is required")
	}
	return nil
}
`)},
	"defensive.missing-schema-validation": {goExample("internal/api/decode.go", `package api

import "encoding/json"

func Decode(raw []byte) map[string]string {
	var payload map[string]string
	json.Unmarshal(raw, &payload)
	return payload
}
`, `package api

import (
	"encoding/json"
	"errors"
)

type Payload struct{ Name string }

func Decode(raw []byte) (Payload, error) {
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Payload{}, err
	}
	if payload.Name == "" {
		return Payload{}, errors.New("name is required")
	}
	return payload, nil
}
`)},
	"defensive.invalid-state-representable": {goExample("internal/deploy/state.go", `package deploy

type DeploymentState struct {
	IsActive  bool
	IsDeleted bool
}
`, `package deploy

type DeploymentPhase int

const (
	PhaseActive DeploymentPhase = iota + 1
	PhaseDeleted
)

type DeploymentState struct {
	Phase DeploymentPhase
}
`)},
	"defensive.integer-overflow": {goExample("internal/buffers/alloc.go", `package buffers

func Allocate(count int) []byte {
	totalBytes := count * 4096
	return make([]byte, totalBytes)
}
`, `package buffers

import (
	"errors"
	"math"
)

func Allocate(count int) ([]byte, error) {
	if count < 0 || count > math.MaxInt/4096 {
		return nil, errors.New("page count out of range")
	}
	return make([]byte, count*4096), nil
}
`)},
	"defensive.bounds-assumption": {goExample("internal/orders/first.go", `package orders

func First(items []string) string {
	return items[0]
}
`, `package orders

func First(items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}
	return items[0], true
}
`)},
	"defensive.unchecked-type-assertion": {goExample("internal/orders/decode.go", `package orders

func DecodeValue(value any) string {
	return value.(string)
}
`, `package orders

func DecodeValue(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}
`)},
}

// designExamples cover the design rules among the most commonly reported.
var designExamples = map[string][]core.RuleExample{
	"design.max-decls-per-file": {goExample("internal/orders/status.go", `package orders

func Pending() string   { return "pending" }
func Paid() string      { return "paid" }
func Shipped() string   { return "shipped" }
func Delivered() string { return "delivered" }
`, `package orders

func Pending() string { return "pending" }
func Paid() string    { return "paid" }
`, "checks.design_rules.max_decls_per_file=3")},
}
