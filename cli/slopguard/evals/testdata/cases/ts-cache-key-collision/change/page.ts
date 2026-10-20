import { cached } from "./cache";
import { render } from "./render";

export function pageTitle(template: string, locale: string): string {
  return cached(`title:${template}`, () => render(template, locale));
}
