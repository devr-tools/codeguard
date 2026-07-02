// Adversarial differential corpus for quality.typescript.non-null-assertion.
// See explicit_any_adversarial.ts for the marker grammar.

// Plain postfix assertion: parity.
export function head(items: string[] | null): string {
  return items![0]; // EXPECT non-null-assertion
}

// Definite assignment assertion: parity (the regex sees identifier + `!`).
export let ready!: boolean; // EXPECT non-null-assertion

// Assertion inside a template-literal interpolation: the stripper blanks the
// whole template including ${...}, so the regex path is blind here.
export function label(items: string[] | null): string {
  return `count=${items!.length}`; // EXPECT non-null-assertion BASELINE-FN non-null-assertion
}

// Regex literal: the `!` in the pattern body leaks into the code view.
export const bangPattern = /ok!/; // BASELINE-FP non-null-assertion

// Formatter-split member chain: the assertion ends each expression, so both
// implementations report the line of the `!` token.
export function nested(el: HTMLElement): string {
  return el
    .querySelector("div")! // EXPECT non-null-assertion
    .textContent!; // EXPECT non-null-assertion
}

// Operators that contain `!` are not assertions: parity clean.
export function guard(a: number, b: number): boolean {
  return a != b && !!b && a !== 0;
}
