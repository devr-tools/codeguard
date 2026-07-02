export function stampAndReport(el: HTMLElement, sanitized: string): string {
  return `updated: ${(el.innerHTML = sanitized)}`;
}
