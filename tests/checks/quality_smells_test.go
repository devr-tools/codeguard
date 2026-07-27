package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

var structuralSmellRuleIDs = []string{
	"smell.god-object",
	"smell.feature-envy",
	"smell.middle-man",
	"smell.message-chain",
	"smell.data-clump",
	"smell.switch-on-type",
	"smell.refused-bequest",
}

type structuralSmellCase struct {
	name     string
	language string
	path     string
	source   string
}

func TestQualityStructuralSmellsDetectMultiLanguageSignals(t *testing.T) {
	for _, tc := range structuralSmellPositiveCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.path), tc.source)
			cfg := qualityPrecisionConfig(dir)
			cfg.Name = "quality-structural-smells-" + tc.name
			cfg.Targets[0].Language = tc.language

			report := runQualityPrecisionScan(t, cfg)

			for _, ruleID := range structuralSmellRuleIDs {
				assertStructuralSmellPresent(t, report, ruleID)
				assertFindingLevel(t, report, "Code Quality", ruleID, "warn")
			}
		})
	}
}

func assertStructuralSmellPresent(t *testing.T, report codeguard.Report, ruleID string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != "Code Quality" {
			continue
		}
		seen := make([]string, 0, len(result.Findings))
		for _, finding := range result.Findings {
			seen = append(seen, finding.RuleID)
			if finding.RuleID == ruleID {
				return
			}
		}
		t.Fatalf("section %q missing rule %q; saw %s", "Code Quality", ruleID, strings.Join(seen, ", "))
	}
	t.Fatalf("section %q not found", "Code Quality")
}

func TestQualityStructuralSmellsAllowSmallCohesiveCode(t *testing.T) {
	for _, tc := range structuralSmellNegativeCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.path), tc.source)
			cfg := qualityPrecisionConfig(dir)
			cfg.Name = "quality-structural-smells-negative-" + tc.name
			cfg.Targets[0].Language = tc.language

			report := runQualityPrecisionScan(t, cfg)

			for _, ruleID := range structuralSmellRuleIDs {
				assertFindingRuleAbsent(t, report, "Code Quality", ruleID)
			}
		})
	}
}

func structuralSmellPositiveCases() []structuralSmellCase {
	return []structuralSmellCase{
		goStructuralSmellPositiveCase(),
		pythonStructuralSmellPositiveCase(),
		{
			name:     "typescript",
			language: "typescript",
			path:     "smells.ts",
			source:   scriptStructuralSmellSource(true),
		},
		{
			name:     "javascript",
			language: "javascript",
			path:     "smells.js",
			source:   scriptStructuralSmellSource(false),
		},
		cppStructuralSmellPositiveCase(),
	}
}

func goStructuralSmellPositiveCase() structuralSmellCase {
	return structuralSmellCase{
		name:     "go",
		language: "go",
		path:     "smells.go",
		source: strings.Join([]string{
			"package sample",
			"",
			"type AccountCoordinator struct {",
			"\trepo string",
			"\tcache string",
			"\tmailer string",
			"\trenderer string",
			"\taudit string",
			"\tclock string",
			"}",
			"",
			"func (a *AccountCoordinator) ValidateAccount() {}",
			"func (a *AccountCoordinator) SaveAccount() {}",
			"func (a *AccountCoordinator) SendEmail() {}",
			"func (a *AccountCoordinator) RenderReport() {}",
			"func (a *AccountCoordinator) CacheAccount() {}",
			"func (a *AccountCoordinator) SyncAccount() {}",
			"func (a *AccountCoordinator) DeleteAccount() {}",
			"func (a *AccountCoordinator) LoadAccount() {}",
			"",
			"type BillingClient interface { Create(string) error; Update(string) error; Delete(string) error; Find(string) error }",
			"type BillingFacade struct { client BillingClient }",
			"func (b BillingFacade) Create(value string) error { return b.client.Create(value) }",
			"func (b BillingFacade) Update(value string) error { return b.client.Update(value) }",
			"func (b BillingFacade) Delete(value string) error { return b.client.Delete(value) }",
			"func (b BillingFacade) Find(value string) error { return b.client.Find(value) }",
			"",
			"func (a *AccountCoordinator) Score(customer Customer) string {",
			"\treturn customer.Profile.Name + customer.Profile.Email + customer.Account.Region + customer.Account.Plan + customer.Account.Status",
			"}",
			"func countryCode(user User) string { return user.Account().Profile().Address().Country().Code() }",
			"func createOrder(customerID string, orderID string, currency string) {}",
			"func updateOrder(customerID string, orderID string, currency string) {}",
			"func cancelOrder(customerID string, orderID string, currency string) {}",
			"func handleOne(event Event) { switch event.Kind { case \"created\": case \"updated\": } }",
			"func handleTwo(event Event) { switch event.Kind { case \"deleted\": case \"archived\": } }",
			"type Store struct{}",
			"type ReadOnlyStore struct { Store }",
			"func (ReadOnlyStore) Save() { panic(\"unsupported\") }",
			"func (ReadOnlyStore) Delete() { panic(\"not implemented\") }",
			"type Customer struct { Profile Profile; Account Account }",
			"type Profile struct { Name string; Email string }",
			"type Account struct { Region string; Plan string; Status string }",
			"type User struct{}",
			"func (User) Account() UserAccount { return UserAccount{} }",
			"type UserAccount struct{}",
			"func (UserAccount) Profile() UserProfile { return UserProfile{} }",
			"type UserProfile struct{}",
			"func (UserProfile) Address() UserAddress { return UserAddress{} }",
			"type UserAddress struct{}",
			"func (UserAddress) Country() UserCountry { return UserCountry{} }",
			"type UserCountry struct{}",
			"func (UserCountry) Code() string { return \"\" }",
			"type Event struct { Kind string }",
		}, "\n"),
	}
}

func pythonStructuralSmellPositiveCase() structuralSmellCase {
	return structuralSmellCase{
		name:     "python",
		language: "python",
		path:     "smells.py",
		source: strings.Join([]string{
			"class AccountCoordinator:",
			"    def __init__(self):",
			"        self.repo = None",
			"        self.cache = None",
			"        self.mailer = None",
			"        self.renderer = None",
			"        self.audit = None",
			"        self.clock = None",
			"    def validate_account(self): pass",
			"    def save_account(self): pass",
			"    def send_email(self): pass",
			"    def render_report(self): pass",
			"    def cache_account(self): pass",
			"    def sync_account(self): pass",
			"    def delete_account(self): pass",
			"    def load_account(self): pass",
			"",
			"class BillingFacade:",
			"    def __init__(self, client):",
			"        self.client = client",
			"    def create(self, value):",
			"        return self.client.create(value)",
			"    def update(self, value):",
			"        return self.client.update(value)",
			"    def delete(self, value):",
			"        return self.client.delete(value)",
			"    def find(self, value):",
			"        return self.client.find(value)",
			"class CustomerScorer:",
			"    def score(self, customer):",
			"        return customer.profile.name + customer.profile.email + customer.account.region + customer.account.plan + customer.account.status",
			"def country_code(user):",
			"    return user.account().profile().address().country().code()",
			"def create_order(customer_id: str, order_id: str, currency: str): pass",
			"def update_order(customer_id: str, order_id: str, currency: str): pass",
			"def cancel_order(customer_id: str, order_id: str, currency: str): pass",
			"def handle_one(event):",
			"    if event.kind == 'created': pass",
			"    elif event.kind == 'updated': pass",
			"def handle_two(event):",
			"    if event.kind == 'deleted': pass",
			"    elif event.kind == 'archived': pass",
			"class ReadOnlyFile(File):",
			"    def write(self, value):",
			"        raise NotImplementedError('unsupported')",
			"    def truncate(self):",
			"        raise NotImplementedError('not supported')",
		}, "\n"),
	}
}

func cppStructuralSmellPositiveCase() structuralSmellCase {
	return structuralSmellCase{
		name:     "cpp",
		language: "c++",
		path:     "smells.cpp",
		source: strings.Join([]string{
			"class AccountCoordinator {",
			"  Repo repo;",
			"  Cache cache;",
			"  Mailer mailer;",
			"  Renderer renderer;",
			"  Audit audit;",
			"  Clock clock;",
			"  void validateAccount() {}",
			"  void saveAccount() {}",
			"  void sendEmail() {}",
			"  void renderReport() {}",
			"  void cacheAccount() {}",
			"  void syncAccount() {}",
			"  void deleteAccount() {}",
			"  void loadAccount() {}",
			"};",
			"class BillingFacade {",
			"  Client client;",
			"  Result create(Value value) { return client.create(value); }",
			"  Result update(Value value) { return client.update(value); }",
			"  Result remove(Value value) { return client.remove(value); }",
			"  Result find(Value value) { return client.find(value); }",
			"};",
			"class CustomerScorer {",
			"  Text score(Customer customer) {",
			"    return customer.profile.name + customer.profile.email + customer.account.region + customer.account.plan + customer.account.status;",
			"  }",
			"};",
			"Text countryCode(User user) { return user.account().profile().address().country().code(); }",
			"void createOrder(String customerId, String orderId, String currency) {}",
			"void updateOrder(String customerId, String orderId, String currency) {}",
			"void cancelOrder(String customerId, String orderId, String currency) {}",
			"void handleOne(Event event) { switch (event.kind) { case Created: break; case Updated: break; } }",
			"void handleTwo(Event event) { switch (event.kind) { case Deleted: break; case Archived: break; } }",
			"class ReadOnlyFile : public File {",
			"  void write() override { throw std::runtime_error(\"unsupported\"); }",
			"  void truncate() override { throw std::runtime_error(\"not supported\"); }",
			"};",
		}, "\n"),
	}
}

func scriptStructuralSmellSource(typed bool) string {
	paramType := ""
	fieldType := ""
	if typed {
		paramType = ": string"
		fieldType = ": unknown"
	}
	return strings.Join([]string{
		"class AccountCoordinator {",
		"  repo" + fieldType + ";",
		"  cache" + fieldType + ";",
		"  mailer" + fieldType + ";",
		"  renderer" + fieldType + ";",
		"  audit" + fieldType + ";",
		"  clock" + fieldType + ";",
		"  validateAccount() {}",
		"  saveAccount() {}",
		"  sendEmail() {}",
		"  renderReport() {}",
		"  cacheAccount() {}",
		"  syncAccount() {}",
		"  deleteAccount() {}",
		"  loadAccount() {}",
		"}",
		"",
		"class BillingFacade {",
		"  client" + fieldType + ";",
		"  create(value" + paramType + ") { return this.client.create(value) }",
		"  update(value" + paramType + ") { return this.client.update(value) }",
		"  delete(value" + paramType + ") { return this.client.delete(value) }",
		"  find(value" + paramType + ") { return this.client.find(value) }",
		"}",
		"",
		"class CustomerScorer {",
		"  score(customer" + paramType + ") {",
		"    return customer.profile.name + customer.profile.email + customer.account.region + customer.account.plan + customer.account.status",
		"  }",
		"}",
		"",
		"function countryCode(user) { return user.account().profile().address().country().code() }",
		"function createOrder(customerId" + paramType + ", orderId" + paramType + ", currency" + paramType + ") {}",
		"function updateOrder(customerId" + paramType + ", orderId" + paramType + ", currency" + paramType + ") {}",
		"function cancelOrder(customerId" + paramType + ", orderId" + paramType + ", currency" + paramType + ") {}",
		"",
		"function handleOne(event) { switch (event.kind) { case 'created': break; case 'updated': break } }",
		"function handleTwo(event) { switch (event.kind) { case 'deleted': break; case 'archived': break } }",
		"",
		"class ReadOnlyFile extends File {",
		"  write(value" + paramType + ") { throw new Error('unsupported') }",
		"  truncate() { throw new Error('not supported') }",
		"}",
	}, "\n")
}

func structuralSmellNegativeCases() []structuralSmellCase {
	return []structuralSmellCase{
		{
			name:     "go",
			language: "go",
			path:     "safe.go",
			source: strings.Join([]string{
				"package sample",
				"",
				"type OrderService struct { repo string }",
				"func (o OrderService) Save(order Order) error { return nil }",
				"func (o OrderService) Find(id string) (Order, error) { return Order{}, nil }",
				"func createOrder(key OrderKey) {}",
				"func updateOrder(key OrderKey) {}",
				"type Order struct{}",
				"type OrderKey struct { CustomerID string; OrderID string; Currency string }",
			}, "\n"),
		},
		{
			name:     "python",
			language: "python",
			path:     "safe.py",
			source: strings.Join([]string{
				"class OrderService:",
				"    def __init__(self, repo): self.repo = repo",
				"    def save(self, order): return self.repo.save(order)",
				"    def find(self, order_id): return self.repo.find(order_id)",
				"def create_order(key): return key",
				"def update_order(key): return key",
			}, "\n"),
		},
		{
			name:     "typescript",
			language: "typescript",
			path:     "safe.ts",
			source: strings.Join([]string{
				"class OrderService {",
				"  repo: Repo",
				"  save(order: Order) { return this.repo.save(order) }",
				"  find(orderId: string) { return this.repo.find(orderId) }",
				"}",
				"function createOrder(key: OrderKey) { return key }",
				"function updateOrder(key: OrderKey) { return key }",
			}, "\n"),
		},
		{
			name:     "javascript",
			language: "javascript",
			path:     "safe.js",
			source: strings.Join([]string{
				"class OrderService {",
				"  save(order) { return this.repo.save(order) }",
				"  find(orderId) { return this.repo.find(orderId) }",
				"}",
				"function createOrder(key) { return key }",
				"function updateOrder(key) { return key }",
			}, "\n"),
		},
		{
			name:     "cpp",
			language: "c++",
			path:     "safe.cpp",
			source: strings.Join([]string{
				"class OrderService {",
				"  Repo repo;",
				"  Result save(Order order) { return repo.save(order); }",
				"  Result find(String orderId) { return repo.find(orderId); }",
				"};",
				"void createOrder(OrderKey key) {}",
				"void updateOrder(OrderKey key) {}",
			}, "\n"),
		},
	}
}
