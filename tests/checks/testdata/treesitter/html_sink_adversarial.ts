// Adversarial differential corpus for security.typescript.unsafe-html-sink.
// See explicit_any_adversarial.ts for the marker grammar.

// Direct assignment: parity.
export function renderBanner(el: HTMLElement, html: string): void {
  el.innerHTML = html; // EXPECT unsafe-html-sink
}

// insertAdjacentHTML call: parity.
export function appendBanner(el: HTMLElement, html: string): void {
  el.insertAdjacentHTML("beforeend", html); // EXPECT unsafe-html-sink
}

// document.write call: parity.
export function writeBanner(html: string): void {
  document.write(html); // EXPECT unsafe-html-sink
}

// Comparison, not assignment: the regex `\.innerHTML\s*=` matches the first
// `=` of `===`.
export function isCleared(el: HTMLElement): boolean {
  return el.innerHTML === ""; // BASELINE-FP unsafe-html-sink
}

// Compound assignment is still an injection sink, but the regex requires a
// bare `=` and `+=` fails.
export function appendChunk(el: HTMLElement, chunk: string): void {
  el.innerHTML += chunk; // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// Mention inside a string literal: parity clean.
export const sinkExample = 'el.innerHTML = markup';

// Real assignment inside a template-literal interpolation: invisible to the
// regex path (the stripper blanks ${...}).
export function stampAndReport(el: HTMLElement, sanitized: string): string {
  return `updated: ${(el.innerHTML = sanitized)}`; // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// Formatter-split receiver: the regex has no \s* between `document` and
// `.write`, so a line break defeats it.
export function writeFooter(trustedFooter: string): void {
  document
    .write(trustedFooter); // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// Property read: parity clean.
export function snapshot(el: HTMLElement): string {
  const copy = el.innerHTML;
  return copy;
}
