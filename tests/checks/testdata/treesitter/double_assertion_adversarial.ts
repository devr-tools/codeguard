// Adversarial differential corpus for quality.typescript.double-assertion.
// See explicit_any_adversarial.ts for the marker grammar.

// Plain unknown hop: parity.
export function coerce(value: string): number {
  return value as unknown as number; // EXPECT double-assertion
}

// Any hop: parity (the cast is also a genuine explicit any).
export function viaAny(payload: object): string[] {
  return payload as any as string[]; // EXPECT double-assertion EXPECT explicit-any
}

// Mention inside a string literal: parity clean.
export const castHint = "prefer proper types over as unknown as T";

// Double cast inside a template-literal interpolation: the stripper blanks
// the whole template including ${...}, so the regex path is blind here.
export function fromTemplate(rows: object): number {
  return `${(rows as unknown as number[]).length}`.length; // EXPECT double-assertion BASELINE-FN double-assertion
}

// Regex literal: the pattern body leaks into the regex path's code view
// (and its `as any` also trips the explicit-any regex).
export const castPattern = / as any as /; // BASELINE-FP double-assertion BASELINE-FP explicit-any

// Chained casts through concrete types are outside this rule's scope (no
// unknown/any hop): parity clean.
type Brand = { kind: "brand" };
type OtherBrand = { kind: "other" };
export function rebrand(v: Brand): OtherBrand {
  return v as object as OtherBrand;
}
