# pytest marks a test module by file-name PREFIX, so this path is a fixture
# scope. A production-looking credential here is still reported (demoted to the
# review tier), never dropped.
BILLING_DSN = "mysql://billing_svc:Zr4tKp9wQm2x@billing.prod.internal:3306/billing"
