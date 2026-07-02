// Adversarial differential corpus for quality.typescript.explicit-any.
// Marker grammar (parsed by typescript_treesitter_test.go):
//   EXPECT <rule>       ground truth: the rule genuinely applies on this line
//   BASELINE-FN <rule>  the regex path misses this line
//   BASELINE-FP <rule>  the regex path wrongly flags this line
// Lines without markers must not be flagged by a correct implementation.

// Plain parameter annotation: parity, everyone catches it.
export function parseRows(input: string, reviver: any): string[] { // EXPECT explicit-any
  return JSON.parse(input, reviver);
}

// As-cast: parity.
export function firstCell(table: unknown): string {
  return (table as any).rows[0]; // EXPECT explicit-any
}

// Mention inside a string literal: parity clean (the stripper blanks it, the
// grammar sees a string node).
export const anyHint = "annotate with : any only as a last resort";

// Real cast inside a template-literal interpolation: the stripper blanks the
// whole template including ${...}, so the regex path is blind here.
export function debugLabel(rows: unknown): string {
  return `rows=${(rows as any).length}`; // EXPECT explicit-any BASELINE-FN explicit-any
}

// Regex literal: the stripper has no regex-literal state, so the pattern
// body leaks into the code view and matches `: any`.
export const anyAnnotation = /: any\b/; // BASELINE-FP explicit-any

// `any` as a legal identifier in parameter and call positions: value
// positions, not type positions, but the regex only sees `, any` and `(any`.
export function countMatches(values: number[], any: (n: number) => boolean): number { // BASELINE-FP explicit-any
  return values.filter(any).length; // BASELINE-FP explicit-any
}

// satisfies-expression type position (TS 4.9+): postdates the regex pattern.
export const fallbackConfig = { retries: 3 } satisfies any; // EXPECT explicit-any BASELINE-FN explicit-any

// Multiline generic argument list: parity.
export function reshape(
  rows: ReadonlyArray<Record<string, any>>, // EXPECT explicit-any
): number {
  return rows.length;
}
