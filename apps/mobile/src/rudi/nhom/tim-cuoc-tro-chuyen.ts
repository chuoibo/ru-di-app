/** Search displayed identities only; encrypted message content stays local. */
export function khopTenCuocTroChuyen(name: string, query: string): boolean {
  const fold = (value: string) => value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").replace(/[đĐ]/g, "d").toLocaleLowerCase("vi").trim();
  const haystack = fold(name);
  return fold(query).split(/\s+/).every((word) => haystack.includes(word));
}
