// Relative time in English as on the evidence ("less than a minute ago",
// "10 hours ago"). The rounding rule is TBD-020; this floors to the unit.
export function relativeTime(iso: string, now: Date = new Date()): string {
  const seconds = Math.max(0, Math.floor((now.getTime() - new Date(iso).getTime()) / 1000));
  if (seconds < 60) {
    return "less than a minute ago";
  }
  const units: [number, string][] = [
    [60 * 60 * 24 * 365, "year"],
    [60 * 60 * 24 * 30, "month"],
    [60 * 60 * 24, "day"],
    [60 * 60, "hour"],
    [60, "minute"],
  ];
  for (const [size, unit] of units) {
    const n = Math.floor(seconds / size);
    if (n >= 1) {
      return `${n} ${unit}${n === 1 ? "" : "s"} ago`;
    }
  }
  return "less than a minute ago";
}

// The attachments panel's short form ("8m ago", Gambar 18). Units below a
// minute and above minutes are not on the evidence (TBD-020); these follow
// the same pattern.
export function shortRelativeTime(iso: string, now: Date = new Date()): string {
  const seconds = Math.max(0, Math.floor((now.getTime() - new Date(iso).getTime()) / 1000));
  const units: [number, string][] = [
    [60 * 60 * 24 * 365, "y"],
    [60 * 60 * 24 * 30, "mo"],
    [60 * 60 * 24, "d"],
    [60 * 60, "h"],
    [60, "m"],
  ];
  for (const [size, unit] of units) {
    const n = Math.floor(seconds / size);
    if (n >= 1) {
      return `${n}${unit} ago`;
    }
  }
  return "0m ago";
}
