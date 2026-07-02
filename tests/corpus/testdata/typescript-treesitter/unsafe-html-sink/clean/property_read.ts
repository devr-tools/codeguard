export function snapshot(el: HTMLElement): string {
  const copy = el.innerHTML;
  return copy;
}
