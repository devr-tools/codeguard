// Adversarial corpus for the tree-sitter spike.
//
// Marker grammar (parsed by treesitter_test.go):
//   EXPECT <rule>       ground truth: the rule genuinely applies on this line
//   BASELINE-FN <rule>  the current regex implementation misses this line
//   BASELINE-FP <rule>  the current regex implementation wrongly flags this line
// Lines without markers must not be flagged by a correct implementation.

// ---------------------------------------------------------------------------
// explicit-any cases
// ---------------------------------------------------------------------------

// C1: plain parameter annotation. Everyone catches this.
export function parseRows(input: string, reviver: any): string[] { // EXPECT explicit-any
  return JSON.parse(input, reviver);
}

// C2: as-cast. Everyone catches this.
export function firstCell(table: unknown): string {
  return (table as any).rows[0]; // EXPECT explicit-any
}

// C3: mention inside a comment. Neither implementation flags it because the
// baseline strips comments first: fallback signature is (rows: any) => any.

// C4: mention inside a string literal. Also stripped by the baseline.
export const anyHint = "annotate with : any only as a last resort";

// C5: real cast inside a template-literal interpolation. The baseline
// stripper blanks the entire template including ${...} expressions, so the
// production rule cannot see code here.
export function debugLabel(rows: unknown): string {
  return `rows=${(rows as any).length}`; // EXPECT explicit-any BASELINE-FN explicit-any
}

// C6: regex literal. The baseline stripper has no regex-literal state, so
// the pattern text leaks into the "code" view and matches `: any`.
export const anyAnnotation = /: any\b/; // BASELINE-FP explicit-any

// C7: `any` is a legal identifier. Both lines below are value positions,
// not type positions, but the baseline regex only sees `, any` and `(any`.
export function countMatches(values: number[], any: (n: number) => boolean): number { // BASELINE-FP explicit-any
  return values.filter(any).length; // BASELINE-FP explicit-any
}

// C8: satisfies-expression type position (TS 4.9+). The baseline pattern
// list predates `satisfies` and has no alternation for it.
export const fallbackConfig = { retries: 3 } satisfies any; // EXPECT explicit-any BASELINE-FN explicit-any

// C9: multiline generic argument list. Parity: both implementations catch
// it because `,` and newline are both \s for the regex.
export function reshape(
  rows: ReadonlyArray<Record<string, any>>, // EXPECT explicit-any
): number {
  return rows.length;
}

// ---------------------------------------------------------------------------
// unsafe-html-sink cases
// ---------------------------------------------------------------------------

// S1: direct assignment. Everyone catches this.
export function renderBanner(el: HTMLElement, html: string): void {
  el.innerHTML = html; // EXPECT unsafe-html-sink
}

// S2: insertAdjacentHTML call. Everyone catches this.
export function appendBanner(el: HTMLElement, html: string): void {
  el.insertAdjacentHTML("beforeend", html); // EXPECT unsafe-html-sink
}

// S3: document.write call. Everyone catches this.
export function writeBanner(html: string): void {
  document.write(html); // EXPECT unsafe-html-sink
}

// S4: comparison, not assignment. The baseline regex `\.innerHTML\s*=`
// matches the first `=` of `===`.
export function isCleared(el: HTMLElement): boolean {
  return el.innerHTML === ""; // BASELINE-FP unsafe-html-sink
}

// S5: compound assignment is still an injection sink, but the baseline
// regex requires `=` immediately after optional whitespace and `+=` fails.
export function appendChunk(el: HTMLElement, chunk: string): void {
  el.innerHTML += chunk; // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// S6: mention inside a comment; parity, nobody flags it:
// legacy path was node.innerHTML = raw

// S7: mention inside a string literal; parity, nobody flags it.
export const sinkExample = 'el.innerHTML = markup';

// S8: real assignment inside a template-literal interpolation; invisible to
// the baseline for the same reason as C5.
export function stampAndReport(el: HTMLElement, sanitized: string): string {
  return `updated: ${(el.innerHTML = sanitized)}`; // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// S9: formatter-split receiver. The baseline pattern has no \s* between
// `document` and `.write`, so a line break defeats it.
export function writeFooter(trustedFooter: string): void {
  document
    .write(trustedFooter); // EXPECT unsafe-html-sink BASELINE-FN unsafe-html-sink
}

// S10: property read; parity, nobody flags it.
export function snapshot(el: HTMLElement): string {
  const copy = el.innerHTML;
  return copy;
}
