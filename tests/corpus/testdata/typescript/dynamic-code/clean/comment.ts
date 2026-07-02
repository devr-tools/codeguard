// Never call eval() on user input; JSON.parse covers our config format.
export function safeParse(text: string): unknown {
  return JSON.parse(text);
}
