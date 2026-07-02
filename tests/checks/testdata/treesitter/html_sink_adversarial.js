// Adversarial differential corpus for the security.javascript.unsafe-html-sink
// mirror: the same tree query serves the javascript grammar (including JSX).
// See explicit_any_adversarial.ts for the marker grammar.

// Direct assignment: parity.
export function renderBanner(el, html) {
  el.innerHTML = html; // EXPECT unsafe-html-sink
}

// Comparison, not assignment: regex false positive.
export function isCleared(el) {
  return el.innerHTML === ""; // BASELINE-FP unsafe-html-sink
}

// Compound assignment: regex misses `+=`.
export function appendChunk(el, chunk) {
  el.innerHTML += chunk; // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// Assignment inside a template-literal interpolation: regex misses it.
export function stampAndReport(el, sanitized) {
  return `updated: ${(el.outerHTML = sanitized)}`; // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// Mention inside a string literal: parity clean.
export const sinkExample = 'el.innerHTML = markup';
