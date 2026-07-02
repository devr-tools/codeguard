// Adversarial differential corpus for the TSX grammar: JSX constructs mixed
// with the migrated TypeScript rules.
// See explicit_any_adversarial.ts for the marker grammar.

// Type position inside a component signature: parity.
export function List(props: { rows: any[] }) { // EXPECT explicit-any
  return <ul>{props.rows.map((row) => <li key={row.id}>{row.label}</li>)}</ul>;
}

// JSX text is neither a string literal nor a comment, so the regex path's
// stripper leaves it in the code view and `: any` matches; the grammar knows
// it is jsx_text.
export function Hint() {
  return <span>fall back to : any only as a last resort</span>; // BASELINE-FP explicit-any
}

// Non-null assertion inside a JSX expression container: parity.
export function Count(props: { items: string[] | null }) {
  return <em>{props.items!.length}</em>; // EXPECT non-null-assertion
}

// HTML sink assignment inside a JSX event handler: parity.
export function Danger(props: { el: HTMLElement; html: string }) {
  return <button onClick={() => { props.el.innerHTML = props.html; }}>go</button>; // EXPECT unsafe-html-sink
}
