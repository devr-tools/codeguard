const examples = [
  "require('child_process').exec('ls')",
  "eval('danger')",
];

export function sample(): string {
  return examples.join("\n");
}
