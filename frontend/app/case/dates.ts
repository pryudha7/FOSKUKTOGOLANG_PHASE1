// Date handling for date fields (BR-UI-011). Values travel as yyyy-mm-dd;
// typing uses dd/MM/yyyy; a field without focus shows the long form with
// the Indonesian month name ("09 Februari 1999").

export const monthsLong = [
  "Januari", "Februari", "Maret", "April", "Mei", "Juni",
  "Juli", "Agustus", "September", "Oktober", "November", "Desember",
];

export const monthsShort = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];

// Week starts on Monday (UI-010 date picker).
export const weekdays = ["Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"];

export type YMD = { y: number; m: number; d: number }; // m is 1-12

export function parseISO(iso: string): YMD | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  return m ? { y: Number(m[1]), m: Number(m[2]), d: Number(m[3]) } : null;
}

export function toISO({ y, m, d }: YMD): string {
  return `${String(y).padStart(4, "0")}-${String(m).padStart(2, "0")}-${String(d).padStart(2, "0")}`;
}

// typedToISO reads dd/MM/yyyy. Anything else is passed on as typed, so the
// server reports it as an invalid date.
export function typedToISO(typed: string): string {
  const m = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(typed.trim());
  return m ? `${m[3]}-${m[2]}-${m[1]}` : typed.trim();
}

export function isoToTyped(iso: string): string {
  const p = parseISO(iso);
  return p ? `${String(p.d).padStart(2, "0")}/${String(p.m).padStart(2, "0")}/${p.y}` : iso;
}

export function isoToLong(iso: string): string {
  const p = parseISO(iso);
  if (!p || p.m < 1 || p.m > 12) {
    return iso;
  }
  return `${String(p.d).padStart(2, "0")} ${monthsLong[p.m - 1]} ${p.y}`;
}

export function daysInMonth(y: number, m: number): number {
  return new Date(Date.UTC(y, m, 0)).getUTCDate();
}

// The weekday column (0 = Monday) of the first day of the month.
export function firstColumn(y: number, m: number): number {
  return (new Date(Date.UTC(y, m - 1, 1)).getUTCDay() + 6) % 7;
}

export function today(): YMD {
  const now = new Date();
  return { y: now.getFullYear(), m: now.getMonth() + 1, d: now.getDate() };
}

export function compare(a: YMD, b: YMD): number {
  return a.y - b.y || a.m - b.m || a.d - b.d;
}
