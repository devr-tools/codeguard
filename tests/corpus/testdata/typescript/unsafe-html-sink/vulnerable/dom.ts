export function render(html: string): void {
  const target = document.createElement("div");
  target.innerHTML = html;
}
