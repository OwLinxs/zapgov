const AV_COLORS = [
  "oklch(0.6 0.13 256)",
  "oklch(0.62 0.14 152)",
  "oklch(0.6 0.15 25)",
  "oklch(0.64 0.13 78)",
  "oklch(0.6 0.14 300)",
  "oklch(0.6 0.12 200)",
  "oklch(0.58 0.15 340)",
  "oklch(0.6 0.13 130)",
];

export function initials(name: string): string {
  return name
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0])
    .join("")
    .toUpperCase();
}

export function Avatar({ name, size = 34 }: { name: string; size?: number }) {
  const safe = name || "?";
  const idx = safe.split("").reduce((a, c) => a + c.charCodeAt(0), 0) % AV_COLORS.length;
  return (
    <div
      className="avatar"
      style={{ width: size, height: size, background: AV_COLORS[idx], fontSize: size * 0.4 }}
    >
      {initials(safe)}
    </div>
  );
}
