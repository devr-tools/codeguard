package fixtures

// PEM blocks such as -----BEGIN RSA PRIVATE KEY----- must never be committed;
// the pre-receive hook rejects them.
func pemPolicyNote() string { return "use the secret manager" }
